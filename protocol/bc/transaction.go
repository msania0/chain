package bc

import (
	"io"

	"chain/crypto/sha3pool"
	"chain/encoding/blockchain"
)

type Transaction struct {
	Header      *EntryRef
	Issuances   []*EntryRef
	Spends      []*EntryRef
	Outputs     []*EntryRef
	Retirements []*EntryRef
}

func NewTransaction(hdrRef *EntryRef) *Transaction {
	hdr := hdrRef.Entry.(*Header)
	spends, issuances := hdr.Inputs()
	tx := &Transaction{
		Header:    hdrRef,
		Issuances: issuances,
		Spends:    spends,
	}
	for _, r := range hdr.Results() {
		switch r.Entry.(type) {
		case *Output:
			tx.Outputs = append(tx.Outputs, r)
		case *Retirement:
			tx.Retirements = append(tx.Retirements, r)
		}
	}
	return tx
}

func (tx *Transaction) ID() Hash {
	return tx.Header.Hash()
}

func (tx *Transaction) SigHash(inpHash Hash) (hash Hash) {
	hasher := sha3pool.Get256()
	defer sha3pool.Put256(hasher)

	hasher.Write(inpHash[:])
	hash = tx.ID()
	hasher.Write(hash[:])
	hasher.Read(hash[:])
	return hash
}

func (tx *Transaction) Results() []*EntryRef {
	return tx.Header.Entry.(*Header).Results()
}

func (tx *Transaction) Version() uint64 {
	return tx.Header.Entry.(*Header).Version()
}

func (tx *Transaction) Data() Hash {
	return tx.Header.Entry.(*Header).Data()
}

func (tx *Transaction) MinTimeMS() uint64 {
	return tx.Header.Entry.(*Header).MinTimeMS()
}

func (tx *Transaction) MaxTimeMS() uint64 {
	return tx.Header.Entry.(*Header).MaxTimeMS()
}

// writeTx writes the Header to w, followed by a varint31 count of
// entries, followed by all the entries reachable from the Header.
func (tx *Transaction) writeTo(w io.Writer) error {
	var entries []*EntryRef
	h := tx.Header.Entry.(*Header)
	h.Walk(func(e *EntryRef) error {
		if e.Entry != h {
			entries = append(entries, e)
		}
		return nil
	})
	err := serializeEntry(w, h)
	if err != nil {
		return err
	}
	_, err = blockchain.WriteVarint31(w, uint64(len(entries)))
	for _, e := range entries {
		err = e.writeEntry(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// readTx reads the output of writeTx and populates entry pointers in
// the Header and the entries reachable from it.
func (tx *Transaction) readFrom(r io.Reader) error {
	var h Header
	err := deserialize(r, &h.body)
	if err != nil {
		return err
	}
	// xxx also deserialize into h.witness, eventually
	n, _, err := blockchain.ReadVarint31(r)
	if err != nil {
		return err
	}
	entries := make(map[Hash]*EntryRef, n)
	for i := uint32(0); i < n; i++ {
		var ref EntryRef
		err = ref.readEntry(r)
		if err != nil {
			return err
		}
		entries[ref.Hash()] = &ref
	}
	return h.Walk(func(e *EntryRef) error {
		if other, ok := entries[e.Hash()]; ok {
			e.Entry = other.Entry
		}
		return nil
	})
	newTx := NewTransaction(&EntryRef{Entry: &h})
	*tx = *newTx
	return nil
}
