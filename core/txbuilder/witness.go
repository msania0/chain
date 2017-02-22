package txbuilder

import (
	"bytes"
	"context"
	"encoding/json"

	"chain/crypto/ed25519/chainkd"
	"chain/crypto/sha3pool"
	chainjson "chain/encoding/json"
	"chain/errors"
	"chain/protocol/bc"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
)

// SignFunc is the function passed into Sign that produces
// a signature for a given xpub, derivation path, and hash.
type SignFunc func(context.Context, chainkd.XPub, [][]byte, [32]byte) ([]byte, error)

// WitnessComponent encodes instructions for finalizing a transaction
// by populating its InputWitness fields. Each WitnessComponent object
// produces zero or more items for the InputWitness of the txinput it
// corresponds to.
type WitnessComponent interface {
	// Sign is called to add signatures. Actual signing is delegated to
	// a callback function.
	Sign(context.Context, *Template, *bc.EntryRef, []chainkd.XPub, SignFunc) error

	// Materialize is called to turn the component into a vector of
	// arguments for the input witness.
	Materialize(*Template, bc.Hash, *[][]byte) error
}

// materializeWitnesses takes a filled in Template and "materializes"
// each witness component, turning it into a vector of arguments for
// the tx's input witness, creating a fully-signed transaction.
func materializeWitnesses(txTemplate *Template) error {
	msg := txTemplate.Transaction

	if msg == nil {
		return errors.Wrap(ErrMissingRawTx)
	}

	if len(txTemplate.SigningInstructions) > len(msg.Outputs)+len(msg.Retirements) {
		return errors.Wrap(ErrBadInstructionCount)
	}

	for _, spRef := range msg.Spends {
		hash := spRef.Hash()
		if sigInst, ok := txTemplate.SigningInstructions[hash]; ok {
			var witness [][]byte
			for j, c := range sigInst.WitnessComponents {
				err := c.Materialize(txTemplate, hash, &witness)
				if err != nil {
					return errors.WithDetailf(err, "error in witness component %d of input %x", j, hash[:])
				}
			}
			sp := spRef.Entry.(*bc.Spend)
			sp.SetArguments(witness)
		}
	}
	for _, issRef := range msg.Issuances {
		hash := issRef.Hash()
		if sigInst, ok := txTemplate.SigningInstructions[hash]; ok {
			var witness [][]byte
			for j, c := range sigInst.WitnessComponents {
				err := c.Materialize(txTemplate, hash, &witness)
				if err != nil {
					return errors.WithDetailf(err, "error in witness component %d of input %x", j, hash[:])
				}
			}
			iss := issRef.Entry.(*bc.Issuance)
			iss.SetArguments(witness)
		}
	}

	return nil
}

type (
	SignatureWitness struct {
		// Quorum is the number of signatures required.
		Quorum int `json:"quorum"`

		// Keys are the identities of the keys to sign with.
		Keys []KeyID `json:"keys"`

		// Program is the predicate part of the signature program, whose hash is what gets
		// signed. If empty, it is computed during Sign from the outputs
		// and the current input of the transaction.
		Program chainjson.HexBytes `json:"program"`

		// Sigs are signatures of Program made from each of the Keys
		// during Sign.
		Sigs []chainjson.HexBytes `json:"signatures"`
	}

	KeyID struct {
		XPub           chainkd.XPub         `json:"xpub"`
		DerivationPath []chainjson.HexBytes `json:"derivation_path"`
	}
)

var ErrEmptyProgram = errors.New("empty signature program")

// Sign populates sw.Sigs with as many signatures of the predicate in
// sw.Program as it can from the overlapping set of keys in sw.Keys
// and xpubs.
//
// If sw.Program is empty, it is populated with an _inferred_ predicate:
// a program committing to aspects of the current
// transaction. Specifically, the program commits to:
//  - the mintime and maxtime of the transaction (if non-zero)
//  - the outputID and (if non-empty) reference data of the current input
//  - the assetID, amount, control program, and (if non-empty) reference data of each output.
func (sw *SignatureWitness) Sign(ctx context.Context, tpl *Template, inpRef *bc.EntryRef, xpubs []chainkd.XPub, signFn SignFunc) error {
	// Compute the predicate to sign. This is either a
	// txsighash program if tpl.AllowAdditional is false (i.e., the tx is complete
	// and no further changes are allowed) or a program enforcing
	// constraints derived from the existing outputs and current input.
	if len(sw.Program) == 0 {
		sw.Program = buildSigProgram(tpl, inpRef)
		if len(sw.Program) == 0 {
			return ErrEmptyProgram
		}
	}
	if len(sw.Sigs) < len(sw.Keys) {
		// Each key in sw.Keys may produce a signature in sw.Sigs. Make
		// sure there are enough slots in sw.Sigs and that we preserve any
		// sigs already present.
		newSigs := make([]chainjson.HexBytes, len(sw.Keys))
		copy(newSigs, sw.Sigs)
		sw.Sigs = newSigs
	}
	var h [32]byte
	sha3pool.Sum256(h[:], sw.Program)
	for i, keyID := range sw.Keys {
		if len(sw.Sigs[i]) > 0 {
			// Already have a signature for this key
			continue
		}
		if !contains(xpubs, keyID.XPub) {
			continue
		}
		path := make([]([]byte), len(keyID.DerivationPath))
		for i, p := range keyID.DerivationPath {
			path[i] = p
		}
		sigBytes, err := signFn(ctx, keyID.XPub, path, h)
		if err != nil {
			return errors.WithDetailf(err, "computing signature %d", i)
		}
		sw.Sigs[i] = sigBytes
	}
	return nil
}

func contains(list []chainkd.XPub, key chainkd.XPub) bool {
	for _, k := range list {
		if bytes.Equal(k[:], key[:]) {
			return true
		}
	}
	return false
}

func buildSigProgram(tpl *Template, inpRef *bc.EntryRef) []byte {
	if !tpl.AllowAdditional {
		h := tpl.Hash(inpRef.Hash())
		builder := vmutil.NewBuilder()
		builder.AddData(h[:])
		builder.AddOp(vm.OP_TXSIGHASH).AddOp(vm.OP_EQUAL)
		return builder.Program
	}
	constraints := make([]constraint, 0, 3+len(tpl.Transaction.Outputs))
	constraints = append(constraints, &timeConstraint{
		minTimeMS: tpl.Transaction.MinTimeMS(),
		maxTimeMS: tpl.Transaction.MaxTimeMS(),
	})
	if sp, ok := inpRef.Entry.(*bc.Spend); ok {
		constraints = append(constraints, outputIDConstraint(sp.SpentOutput().Hash()))
	}

	// Commitment to refdata is conditional on it being non-zero.
	data := tpl.Transaction.Data()
	if (data != bc.Hash{}) {
		constraints = append(constraints, refdataConstraint{data[:], true})
	}
	switch inp := inpRef.Entry.(type) {
	case *bc.Issuance:
		data = inp.Data()
	case *bc.Spend:
		data = inp.Data()
	default:
		data = bc.Hash{}
	}
	if (data != bc.Hash{}) {
		constraints = append(constraints, refdataConstraint{data[:], false})
	}

	for _, outRef := range tpl.Transaction.Outputs {
		out := outRef.Entry.(*bc.Output)
		c := &payConstraint{
			Hash:        outRef.Hash(),
			AssetAmount: bc.AssetAmount{AssetID: out.AssetID(), Amount: out.Amount()},
			Program:     out.ControlProgram().Code, // xxx preserve vmversion?
			Data:        out.Data(),
		}
		constraints = append(constraints, c)
	}
	for _, retRef := range tpl.Transaction.Retirements {
		ret := retRef.Entry.(*bc.Retirement)
		c := &payConstraint{
			Hash:        retRef.Hash(),
			AssetAmount: bc.AssetAmount{AssetID: ret.AssetID(), Amount: ret.Amount()},
			// xxx what value for payConstraint.Program?
			Data: ret.Data(),
		}
		constraints = append(constraints, c)
	}

	var program []byte
	for i, c := range constraints {
		program = append(program, c.code()...)
		if i < len(constraints)-1 { // leave the final bool on top of the stack
			program = append(program, byte(vm.OP_VERIFY))
		}
	}
	return program
}

func (sw SignatureWitness) Materialize(tpl *Template, _ bc.Hash, args *[][]byte) error {
	// This is the value of N for the CHECKPREDICATE call. The code
	// assumes that everything already in the arg list before this call
	// to Materialize is input to the signature program, so N is
	// len(*args).
	*args = append(*args, vm.Int64Bytes(int64(len(*args))))

	var nsigs int
	for i := 0; i < len(sw.Sigs) && nsigs < sw.Quorum; i++ {
		if len(sw.Sigs[i]) > 0 {
			*args = append(*args, sw.Sigs[i])
			nsigs++
		}
	}
	*args = append(*args, sw.Program)
	return nil
}

func (sw SignatureWitness) MarshalJSON() ([]byte, error) {
	obj := struct {
		Type   string               `json:"type"`
		Quorum int                  `json:"quorum"`
		Keys   []KeyID              `json:"keys"`
		Sigs   []chainjson.HexBytes `json:"signatures"`
	}{
		Type:   "signature",
		Quorum: sw.Quorum,
		Keys:   sw.Keys,
		Sigs:   sw.Sigs,
	}
	return json.Marshal(obj)
}

func (si *SigningInstruction) AddWitnessKeys(keys []KeyID, quorum int) {
	sw := &SignatureWitness{
		Quorum: quorum,
		Keys:   keys,
	}
	si.WitnessComponents = append(si.WitnessComponents, sw)
}
