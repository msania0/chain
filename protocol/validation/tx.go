package validation

import (
	"math"

	"golang.org/x/sync/errgroup"

	"chain/errors"
	"chain/math/checked"
	"chain/protocol/bc"
	"chain/protocol/state"
	"chain/protocol/vm"
)

// ErrBadTx is returned for transactions failing validation
var ErrBadTx = errors.New("invalid transaction")

var (
	// "suberrors" for ErrBadTx
	errTxVersion              = errors.New("unknown transaction version")
	errNotYet                 = errors.New("block time is before transaction min time")
	errTooLate                = errors.New("block time is after transaction max time")
	errWrongBlockchain        = errors.New("issuance is for different blockchain")
	errTimelessIssuance       = errors.New("zero mintime or maxtime not allowed in issuance with non-empty nonce")
	errIssuanceTime           = errors.New("timestamp outside issuance input's time window")
	errDuplicateIssuance      = errors.New("duplicate issuance transaction")
	errInvalidOutput          = errors.New("invalid output")
	errNoInputs               = errors.New("inputs are missing")
	errTooManyInputs          = errors.New("number of inputs overflows uint32")
	errAllEmptyNonceIssuances = errors.New("all inputs are issuances with empty nonce fields")
	errMisorderedTime         = errors.New("positive maxtime must be >= mintime")
	errAssetVersion           = errors.New("unknown asset version")
	errInputTooBig            = errors.New("input value exceeds maximum value of int64")
	errInputSumTooBig         = errors.New("sum of inputs overflows the allowed asset amount")
	errVMVersion              = errors.New("unknown vm version")
	errDuplicateInput         = errors.New("duplicate input")
	errTooManyOutputs         = errors.New("number of outputs overflows int32")
	errEmptyOutput            = errors.New("output value must be greater than 0")
	errOutputTooBig           = errors.New("output value exceeds maximum value of int64")
	errOutputSumTooBig        = errors.New("sum of outputs overflows the allowed asset amount")
	errUnbalancedV1           = errors.New("amounts for asset are not balanced on v1 inputs and outputs")
)

func badTxErr(err error) error {
	err = errors.WithData(err, "badtx", err)
	err = errors.WithDetail(err, err.Error())
	return errors.Sub(ErrBadTx, err)
}

func badTxErrf(err error, f string, args ...interface{}) error {
	err = errors.WithData(err, "badtx", err)
	err = errors.WithDetailf(err, f, args...)
	return errors.Sub(ErrBadTx, err)
}

// ConfirmTx validates the given transaction against the given state tree
// before it's added to a block. If tx is invalid, it returns a non-nil
// error describing why.
//
// Tx must already have undergone the well-formedness check in
// CheckTxWellFormed. This should have happened when the tx was added
// to the pool.
//
// ConfirmTx must not mutate the snapshot or the block.
func ConfirmTx(snapshot *state.Snapshot, initialBlockHash bc.Hash, block *bc.Block, tx *bc.Transaction) error {
	version := tx.Version()
	if version < 1 || version > block.Version {
		return badTxErrf(errTxVersion, "unknown transaction version %d for block version %d", version, block.Version)
	}

	if block.TimestampMS < tx.MinTimeMS() {
		return badTxErr(errNotYet)
	}
	if tx.MaxTimeMS() > 0 && block.TimestampMS > tx.MaxTimeMS() {
		return badTxErr(errTooLate)
	}

	for _, issRef := range tx.Issuances {
		iss := issRef.Entry.(*bc.Issuance)
		if iss.InitialBlockID() != initialBlockHash {
			return badTxErr(errWrongBlockchain)
		}
		// xxx check for empty nonce? old code had "if len(ii.Nonce) == 0 { continue }"
		anchorRef := iss.Anchor()
		if anchorRef == nil {
			// xxx continue ??
		}
		nonce, ok := anchorRef.Entry.(*bc.Nonce)
		if !ok {
			// xxx continue ??
		}
		trRef := nonce.TimeRange()
		if trRef == nil {
			// xxx continue ??
		}
		tr, ok := trRef.Entry.(*bc.TimeRange)
		if !ok {
			// xxx continue ??
		}
		if tr.MinTimeMS() == 0 || tr.MaxTimeMS() == 0 {
			return badTxErr(errTimelessIssuance)
		}
		if block.TimestampMS < tr.MinTimeMS() || block.TimestampMS > tr.MaxTimeMS() {
			return badTxErr(errIssuanceTime)
		}
		// xxx check issuance memory
	}

	for _, spRef := range tx.Spends {
		sp := spRef.Entry.(*bc.Spend)
		spentOutputID := sp.SpentOutput().Hash()
		k, val := state.OutputTreeItem(spentOutputID)
		if !snapshot.Tree.Contains(k, val) {
			inputID := spRef.Hash()
			return badTxErrf(errInvalidOutput, "output %x for spend input %x is invalid", spentOutputID[:], inputID[:])
		}
	}

	return nil
}

// CheckTxWellFormed checks whether tx is "well-formed" (the
// context-free phase of validation):
// - inputs and outputs balance
// - no duplicate input commitments
// - input scripts pass
//
// Result is nil for well-formed transactions, ErrBadTx with
// supporting detail otherwise.
func CheckTxWellFormed(tx *bc.Transaction) error {
	nInputs := len(tx.Spends) + len(tx.Issuances)
	if nInputs == 0 {
		return badTxErr(errNoInputs)
	}
	if nInputs > math.MaxInt32 {
		return badTxErr(errTooManyInputs)
	}

	nResults := len(tx.Outputs) + len(tx.Retirements)
	if nResults > math.MaxInt32 {
		return badTxErr(errTooManyOutputs)
	}

	// Are all inputs issuances, all with asset version 1, and all with empty nonces?
	allIssuancesWithEmptyNonces := true
	if len(tx.Spends) > 0 {
		allIssuancesWithEmptyNonces = false
	} else {
		for _, issRef := range tx.Issuances {
			iss, ok := issRef.Entry.(*bc.Issuance)
			if !ok {
				// xxx (impossible?) error
			}
			if !iss.Anchor().IsNil() { // xxx is this the right test?
				allIssuancesWithEmptyNonces = false
				break
			}
		}
	}
	if allIssuancesWithEmptyNonces {
		return badTxErr(errAllEmptyNonceIssuances)
	}

	// Check that the transaction maximum time is greater than or equal to the
	// minimum time, if it is greater than 0.
	maxTimeMS := tx.MaxTimeMS()
	if maxTimeMS > 0 && maxTimeMS < tx.MinTimeMS() {
		return badTxErr(errMisorderedTime)
	}

	txVersion := tx.Version()

	// Check that each input commitment appears only once. Also check that sums
	// of inputs and outputs balance, and check that both input and output sums
	// are less than 2^63 so that they don't overflow their int64 representation.
	parity := make(map[bc.AssetID]int64)

	for _, spRef := range tx.Spends {
		sp := spRef.Entry.(*bc.Spend)
		outRef := sp.SpentOutput()
		out := outRef.Entry.(*bc.Output)
		if out.Amount() > math.MaxInt64 {
			return badTxErr(errInputTooBig)
		}
		sum, ok := checked.AddInt64(parity[out.AssetID()], int64(out.Amount()))
		if !ok {
			id := spRef.Hash()
			return badTxErrf(errInputSumTooBig, "adding input %x overflows the allowed asset amount", id[:])
		}
		parity[out.AssetID()] = sum
		if txVersion == 1 {
			prog := out.ControlProgram()
			if prog.VMVersion != 1 {
				id := spRef.Hash()
				outID := outRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in input %x (spending output %x) for transaction version %d", prog.VMVersion, id[:], outID[:], txVersion)
			}
		}
	}

	for _, issRef := range tx.Issuances {
		iss := issRef.Entry.(*bc.Issuance)
		amount := iss.Amount()
		if amount > math.MaxInt64 {
			return badTxErr(errInputTooBig)
		}
		assetID := iss.AssetID()
		sum, ok := checked.AddInt64(parity[assetID], int64(amount))
		if !ok {
			id := issRef.Hash()
			return badTxErrf(errInputSumTooBig, "adding input %x overflows the allowed asset amount", id[:])
		}
		parity[assetID] = sum
		if txVersion == 1 {
			prog := iss.IssuanceProgram()
			if prog.VMVersion != 1 {
				id := issRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in input %x for transaction version %d", prog.VMVersion, id[:], txVersion)
			}
		}
		nonceRef := iss.Anchor()
		_, ok = nonceRef.Entry.(*bc.Nonce)
		if !ok {
			// xxx
		}
	}

	for _, outRef := range tx.Outputs {
		out := outRef.Entry.(*bc.Output)
		if txVersion == 1 {
			prog := out.ControlProgram()
			if prog.VMVersion != 1 {
				id := outRef.Hash()
				return badTxErrf(errVMVersion, "unknown vm version %d in output %x for transaction version %d", prog.VMVersion, id[:], txVersion)
			}
		}
		if out.Amount() == 0 {
			return badTxErr(errEmptyOutput)
		}
		if out.Amount() > math.MaxInt64 {
			return badTxErr(errOutputTooBig)
		}
		assetID := out.AssetID()
		sum, ok := checked.SubInt64(parity[assetID], int64(out.Amount()))
		if !ok {
			id := outRef.Hash()
			return badTxErrf(errOutputSumTooBig, "adding output %x (%d units of asset %x) overflows the allowed asset amount", id[:], out.Amount(), assetID[:])
		}
		parity[assetID] = sum
	}

	for _, retRef := range tx.Retirements {
		ret := retRef.Entry.(*bc.Output)
		if ret.Amount() == 0 {
			return badTxErr(errEmptyOutput)
		}
		if ret.Amount() > math.MaxInt64 {
			return badTxErr(errOutputTooBig)
		}
		assetID := ret.AssetID()
		sum, ok := checked.SubInt64(parity[assetID], int64(ret.Amount()))
		if !ok {
			id := retRef.Hash()
			return badTxErrf(errOutputSumTooBig, "adding retirement %x (%d units of asset %x) overflows the allowed asset amount", id[:], ret.Amount(), assetID[:])
		}
		parity[assetID] = sum
	}

	for assetID, val := range parity {
		if val != 0 {
			return badTxErrf(errUnbalancedV1, "amounts for asset %s are not balanced on v1 inputs and outputs", assetID)
		}
	}

	// verifyFn returns a closure suitable for use in errgroup.Group.Go
	verifyFn := func(e *bc.EntryRef) func() error {
		return func() error {
			err := vm.VerifyTxInput(tx, e)
			if err != nil {
				id := e.Hash()
				return badTxErrf(err, "validation failed in script execution, input %x", id[:])
			}
			return nil
		}
	}

	var g errgroup.Group
	for _, spRef := range tx.Spends {
		g.Go(verifyFn(spRef))
	}
	for _, issRef := range tx.Issuances {
		g.Go(verifyFn(issRef))
	}

	return g.Wait()
}

// ApplyTx updates the state tree with all the changes to the ledger.
func ApplyTx(snapshot *state.Snapshot, tx *bc.Transaction) error {
	for range tx.Issuances {
		// xxx add issuance to the issuance memory
	}

	for _, spRef := range tx.Spends {
		sp := spRef.Entry.(*bc.Spend)
		spentOutputID := sp.SpentOutput().Hash()
		err := snapshot.Tree.Delete(spentOutputID.Bytes())
		if err != nil {
			return err
		}
	}

	for _, outRef := range tx.Outputs {
		// Insert new outputs into the state tree.
		err := snapshot.Tree.Insert(state.OutputTreeItem(outRef.Hash()))
		if err != nil {
			return err
		}
	}

	return nil
}
