package txbuilder

import (
	"context"
	"encoding/json"
	"time"

	chainjson "chain/encoding/json"
	"chain/errors"
	"chain/protocol/bc"
)

// Template represents a partially- or fully-signed transaction.
type Template struct {
	Transaction         *bc.Transaction                 `json:"raw_transaction"` // xxx must json-encode to the transitive collection of entries
	SigningInstructions map[bc.Hash]*SigningInstruction `json:"signing_instructions"`

	// Local indicates that all inputs to the transaction are signed
	// exclusively by keys managed by this Core. Whenever accepting
	// a template from an external Core, `Local` should be set to
	// false.
	Local bool `json:"local"`

	// AllowAdditional affects whether Sign commits to the tx sighash or
	// to individual details of the tx so far. When true, signatures
	// commit to tx details, and new details may be added but existing
	// ones cannot be changed. When false, signatures commit to the tx
	// as a whole, and any change to the tx invalidates the signature.
	AllowAdditional bool `json:"allow_additional_actions"`
}

// Hash produces the txsighash for a specific input (given as an entry
// hash)
func (t *Template) Hash(inpHash bc.Hash) bc.Hash {
	return t.Transaction.SigHash(inpHash)
}

// SigningInstruction gives directions for signing inputs in a TxTemplate.
type SigningInstruction struct {
	bc.Hash // hash of entry to sign
	bc.AssetAmount
	WitnessComponents []WitnessComponent `json:"witness_components,omitempty"`
}

func (si *SigningInstruction) UnmarshalJSON(b []byte) error {
	var pre struct {
		bc.Hash
		bc.AssetAmount
		WitnessComponents []struct {
			Type string
			SignatureWitness
		} `json:"witness_components"`
	}
	err := json.Unmarshal(b, &pre)
	if err != nil {
		return err
	}

	si.Hash = pre.Hash
	si.AssetAmount = pre.AssetAmount
	si.WitnessComponents = make([]WitnessComponent, 0, len(pre.WitnessComponents))
	for i, w := range pre.WitnessComponents {
		if w.Type != "signature" {
			return errors.WithDetailf(ErrBadWitnessComponent, "witness component %d has unknown type '%s'", i, w.Type)
		}
		si.WitnessComponents = append(si.WitnessComponents, &w.SignatureWitness)
	}
	return nil
}

type Action interface {
	Build(context.Context, *TemplateBuilder) error
}

// Reciever encapsulates information about where to send assets.
type Receiver struct {
	ControlProgram chainjson.HexBytes `json:"control_program"`
	ExpiresAt      time.Time          `json:"expires_at"`
}
