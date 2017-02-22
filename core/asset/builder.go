package asset

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"time"

	"chain/core/signers"
	"chain/core/txbuilder"
	"chain/crypto/sha3pool"
	"chain/database/pg"
	chainjson "chain/encoding/json"
	"chain/errors"
	"chain/protocol/bc"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
)

func (reg *Registry) NewIssueAction(assetAmount bc.AssetAmount, referenceData chainjson.Map) txbuilder.Action {
	return &issueAction{
		assets:        reg,
		AssetAmount:   assetAmount,
		ReferenceData: referenceData,
	}
}

func (reg *Registry) DecodeIssueAction(data []byte) (txbuilder.Action, error) {
	a := &issueAction{assets: reg}
	err := json.Unmarshal(data, a)
	return a, err
}

type issueAction struct {
	assets *Registry
	bc.AssetAmount
	ReferenceData chainjson.Map `json:"reference_data"`
}

func (a *issueAction) Build(ctx context.Context, builder *txbuilder.TemplateBuilder) error {
	if a.AssetID == (bc.AssetID{}) {
		return txbuilder.MissingFieldsError("asset_id")
	}

	asset, err := a.assets.findByID(ctx, a.AssetID)
	if errors.Root(err) == pg.ErrUserInputNotFound {
		err = errors.WithDetailf(err, "missing asset with ID %q", a.AssetID)
	}
	if err != nil {
		return err
	}

	var nonce [8]byte
	_, err = rand.Read(nonce[:])
	if err != nil {
		return err
	}
	progBuilder := vmutil.NewBuilder()
	progBuilder.AddData(nonce[:]).AddOp(vm.OP_TRUE)

	now := time.Now()
	builder.RestrictMinTime(time.Now())

	maxTimeMS := bc.Millis(now.Add(time.Minute)) // xxx placeholder
	trRef := &bc.EntryRef{Entry: bc.NewTimeRange(bc.Millis(now), maxTimeMS)}
	nonceRef := &bc.EntryRef{Entry: bc.NewNonce(bc.Program{VMVersion: 1, Code: progBuilder.Program}, trRef)}

	tplIn := &txbuilder.SigningInstruction{AssetAmount: a.AssetAmount}
	path := signers.Path(asset.Signer, signers.AssetKeySpace)
	keyIDs := txbuilder.KeyIDs(asset.Signer.XPubs, path)
	tplIn.AddWitnessKeys(keyIDs, asset.Signer.Quorum)

	var refdataHash bc.Hash
	if len(a.ReferenceData) > 0 {
		sha3pool.Sum256(refdataHash[:], a.ReferenceData)
		// xxx register data/hash mapping with builder
	}

	return builder.AddIssuance(nonceRef, a.AssetAmount, refdataHash, tplIn)
}
