// Package txbuilder builds a Chain Protocol transaction from
// a list of actions.
package txbuilder

import (
	"context"
	"time"

	"chain/crypto/ed25519/chainkd"
	"chain/encoding/json"
	"chain/errors"
	"chain/math/checked"
	"chain/protocol/bc"
)

var (
	ErrBadRefData          = errors.New("transaction reference data does not match previous template's reference data")
	ErrBadTxInputIdx       = errors.New("unsigned tx missing input")
	ErrBadWitnessComponent = errors.New("invalid witness component")
	ErrBadAmount           = errors.New("bad asset amount")
	ErrBlankCheck          = errors.New("unsafe transaction: leaves assets free to control")
	ErrAction              = errors.New("errors occurred in one or more actions")
	ErrMissingFields       = errors.New("required field is missing")
)

// Build builds or adds on to a transaction.
// Initially, inputs are left unconsumed, and destinations unsatisfied.
// Build partners then satisfy and consume inputs and destinations.
// The final party must ensure that the transaction is
// balanced before calling finalize.
func Build(ctx context.Context, tx *bc.Transaction, actions []Action, maxTime time.Time) (*Template, error) {
	builder := NewBuilder(maxTime, tx)

	// Build all of the actions, updating the builder.
	var errs []error
	for i, action := range actions {
		err := action.Build(ctx, builder)
		if err != nil {
			err = errors.WithData(err, "index", i)
			errs = append(errs, err)
		}
	}

	// If there were any errors, rollback and return a composite error.
	if len(errs) > 0 {
		builder.rollback()
		return nil, errors.WithData(ErrAction, "actions", errs)
	}

	// Build the transaction template.
	tpl, tx, err := builder.Build()
	if err != nil {
		builder.rollback()
		return nil, err
	}

	err = checkBlankCheck(tx)
	if err != nil {
		builder.rollback()
		return nil, err
	}

	return tpl, nil
}

// KeyIDs produces KeyIDs from a list of xpubs and a derivation path
// (applied to all the xpubs).
func KeyIDs(xpubs []chainkd.XPub, path [][]byte) []KeyID {
	result := make([]KeyID, 0, len(xpubs))
	var hexPath []json.HexBytes
	for _, p := range path {
		hexPath = append(hexPath, p)
	}
	for _, xpub := range xpubs {
		result = append(result, KeyID{xpub, hexPath})
	}
	return result
}

func Sign(ctx context.Context, tpl *Template, xpubs []chainkd.XPub, signFn SignFunc) error {
	for hash, sigInst := range tpl.SigningInstructions {
		for j, c := range sigInst.WitnessComponents {
			err := c.Sign(ctx, tpl, hash, xpubs, signFn)
			if err != nil {
				return errors.WithDetailf(err, "adding signature(s) to witness component %d of input %x", j, hash[:])
			}
		}
	}
	return materializeWitnesses(tpl)
}

func checkBlankCheck(tx *bc.Transaction) error {
	assetMap := make(map[bc.AssetID]int64)
	var ok bool
	for _, issRef := range tx.Issuances {
		iss := issRef.Entry.(*bc.Issuance)
		assetID := iss.AssetID()
		assetMap[assetID], ok = checked.AddInt64(assetMap[assetID], int64(iss.Amount()))
		if !ok {
			return errors.WithDetailf(ErrBadAmount, "cumulative amounts for asset %s overflow the allowed asset amount 2^63", assetID)
		}
	}
	for _, spRef := range tx.Spends {
		sp := spRef.Entry.(*bc.Spend)
		spentOutputRef := sp.SpentOutput()
		spentOutput := spentOutputRef.Entry.(*bc.Output)
		assetID := spentOutput.AssetID()
		assetMap[assetID], ok = checked.AddInt64(assetMap[assetID], int64(spentOutput.Amount()))
		if !ok {
			return errors.WithDetailf(ErrBadAmount, "cumulative amounts for asset %s overflow the allowed asset amount 2^63", assetID)
		}
	}
	for _, outRef := range tx.Outputs {
		out := outRef.Entry.(*bc.Output)
		assetID := out.AssetID()
		assetMap[assetID], ok = checked.SubInt64(assetMap[assetID], int64(out.Amount()))
		if !ok {
			return errors.WithDetailf(ErrBadAmount, "cumulative amounts for asset %s overflow the allowed asset amount 2^63", assetID)
		}
	}
	for _, retRef := range tx.Retirements {
		ret := retRef.Entry.(*bc.Retirement)
		assetID := ret.AssetID()
		assetMap[assetID], ok = checked.SubInt64(assetMap[assetID], int64(ret.Amount()))
		if !ok {
			return errors.WithDetailf(ErrBadAmount, "cumulative amounts for asset %s overflow the allowed asset amount 2^63", assetID)
		}
	}

	var requiresOutputs, requiresInputs bool
	for _, amt := range assetMap {
		if amt > 0 {
			requiresOutputs = true
		}
		if amt < 0 {
			requiresInputs = true
		}
	}

	// 4 possible cases here:
	//
	// requiresOutputs  requiresInputs
	// ---------------  --------------
	//  false            false
	//    This is a balanced transaction with no free assets to consume.
	//    It could potentially be a complete transaction.
	//
	//  true             false
	//    This is an unbalanced transaction with free assets to consume.
	//
	//  false            true
	//    This is an unbalanced transaction requiring assets to be spent.
	//
	//  true             true
	//    This is an unbalanced transaction with free assets to consume
	//    and requiring assets to be spent.
	//
	// The only case that needs to be protected against is 2 ("free
	// assets to consume").

	if requiresOutputs && !requiresInputs {
		return errors.Wrap(ErrBlankCheck)
	}

	return nil
}

// MissingFieldsError returns a wrapped error ErrMissingFields
// with a data item containing the given field names.
func MissingFieldsError(name ...string) error {
	return errors.WithData(ErrMissingFields, "missing_fields", name)
}
