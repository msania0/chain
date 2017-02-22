package prottest

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"

	"chain/crypto/ed25519/chainkd"
	"chain/crypto/sha3pool"
	"chain/protocol"
	"chain/protocol/bc"
	"chain/protocol/vm"
	"chain/protocol/vmutil"
	"chain/testutil"
)

// NewIssuanceTx creates a new signed, issuance transaction issuing 100 units
// of a new asset to a garbage control program. The resulting transaction has
// one input and one output.
//
// The asset issued is created from randomly-generated keys. The resulting
// transaction is finalized (signed with a TXSIGHASH commitment).
func NewIssuanceTx(tb testing.TB, c *protocol.Chain) *bc.Transaction {
	ctx := context.Background()
	b1, err := c.GetBlock(ctx, 1)
	if err != nil {
		testutil.FatalErr(tb, err)
	}

	// Generate a random key pair for the asset being issued.
	xprv, xpub, err := chainkd.NewXKeys(nil)
	if err != nil {
		testutil.FatalErr(tb, err)
	}
	pubkeys := chainkd.XPubKeys([]chainkd.XPub{xpub})

	// Create a corresponding issuance program.
	sigProg, err := vmutil.P2SPMultiSigProgram(pubkeys, 1)
	if err != nil {
		testutil.FatalErr(tb, err)
	}
	builder := vmutil.NewBuilder()
	builder.AddRawBytes(sigProg)
	issuanceProgram := builder.Program

	// Create a transaction issuing this new asset.
	var nonce [32]byte
	_, err = rand.Read(nonce[:])
	if err != nil {
		testutil.FatalErr(tb, err)
	}
	builder = vmutil.NewBuilder()
	builder.AddData(nonce[:])
	builder.AddOp(vm.OP_TRUE)
	nonceProg := builder.Program

	minTimeMS := bc.Millis(time.Now().Add(-5 * time.Minute))
	maxTimeMS := bc.Millis(time.Now().Add(5 * time.Minute))
	trRef := &bc.EntryRef{Entry: bc.NewTimeRange(minTimeMS, maxTimeMS)}

	nonceRef := &bc.EntryRef{Entry: bc.NewNonce(bc.Program{VMVersion: 1, Code: nonceProg}, trRef)}

	assetdef := []byte(`{"type": "prottest issuance"}`)
	var assetDefHash bc.Hash
	sha3pool.Sum256(assetDefHash[:], assetdef)

	assetID := bc.ComputeAssetID(issuanceProgram, b1.Hash(), 1, assetDefHash)
	assetAmount := bc.AssetAmount{AssetID: assetID, Amount: 100}

	bcBuilder := bc.NewBuilder(1, minTimeMS, maxTimeMS, nil)
	issRef := bcBuilder.AddIssuance(nonceRef, assetAmount, bc.Hash{})
	bcBuilder.AddOutput(assetAmount, bc.Program{VMVersion: 1, Code: []byte{0xbe, 0xef}}, bc.Hash{})
	tx := bcBuilder.Build()

	// Sign with a simple TXSIGHASH signature.
	builder = vmutil.NewBuilder()
	h := tx.SigHash(issRef.Hash())
	builder.AddData(h[:])
	builder.AddOp(vm.OP_TXSIGHASH).AddOp(vm.OP_EQUAL)
	sigprog := builder.Program
	sigproghash := sha3.Sum256(sigprog)
	signature := xprv.Sign(sigproghash[:])

	var witness [][]byte
	witness = append(witness, vm.Int64Bytes(0)) // 0 args to the sigprog
	witness = append(witness, signature)
	witness = append(witness, sigprog)
	iss := issRef.Entry.(*bc.Issuance)
	iss.SetArguments(witness)

	return tx
}
