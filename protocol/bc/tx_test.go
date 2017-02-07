package bc

import (
	"encoding/hex"
	"testing"
)

func TestTxHashes(t *testing.T) {
	cases := []struct {
		txdata *TxData
		hash   Hash
	}{
		{
			txdata: &TxData{},
			hash:   mustDecodeHash("827b87bafb63c999922d0190010351435bb73a4d96612beea007b4d811607fb0"),
		},
		{
			txdata: sampleTx(),
			hash:   mustDecodeHash("fbca3b1e447c46f7926931950960b60fc86237a9402ce68c78e6144da02a5d82"),
		},
	}

	for i, c := range cases {
		hashes, err := TxHashes(c.txdata)
		if err != nil {
			t.Fatal(err)
		}
		if len(hashes.VMContexts) != len(c.txdata.Inputs) {
			t.Errorf("case %d: len(hashes.VMContexts) = %d, want %d", i, len(hashes.VMContexts), len(c.txdata.Inputs))
		}
		if c.hash != hashes.ID {
			t.Errorf("case %d: got txid %x, want %x", i, hashes.ID[:], c.hash[:])
		}
	}
}

func BenchmarkHashEmptyTx(b *testing.B) {
	tx := &TxData{}
	for i := 0; i < b.N; i++ {
		_, err := TxHashes(tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHashNonemptyTx(b *testing.B) {
	tx := sampleTx()
	for i := 0; i < b.N; i++ {
		_, err := TxHashes(tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func mustDecodeHash(hash string) (h [32]byte) {
	if len(hash) != hex.EncodedLen(len(h)) {
		panic("wrong length hash")
	}
	_, err := hex.Decode(h[:], []byte(hash))
	if err != nil {
		panic(err)
	}
	return h
}

func sampleTx() *TxData {
	assetID := ComputeAssetID([]byte{1}, mustDecodeHash("03deff1d4319d67baa10a6d26c1fea9c3e8d30e33474efee1a610a9bb49d758d"), 1, EmptyStringHash)
	return &TxData{
		Version: 1,
		Inputs: []*TxInput{
			NewSpendInput(OutputID{mustDecodeHash("dd385f6fe25d91d8c1bd0fa58951ad56b0c5229dcc01f61d9f9e8b9eb92d3292")}, nil, assetID, 1000000000000, []byte{1}, []byte("input")),
			NewSpendInput(OutputID{Hash{17}}, nil, assetID, 1, []byte{2}, []byte("input2")),
		},
		Outputs: []*TxOutput{
			NewTxOutput(assetID, 600000000000, []byte{1}, nil),
			NewTxOutput(assetID, 400000000000, []byte{2}, nil),
		},
		MinTime:       1492590000,
		MaxTime:       1492590591,
		ReferenceData: []byte("distribution"),
	}
}
