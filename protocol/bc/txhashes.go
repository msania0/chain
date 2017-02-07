package bc

type (
	// TxHashes holds data needed for validation and state updates.
	TxHashes struct {
		ID Hash
		// contains OutputIDs and retirement hashes.
		// each OutputID is also the corresponding UnspentID
		ResultHashes []Hash
		Issuances    []struct {
			ID           Hash
			ExpirationMS uint64
		}
		VMContexts map[Hash]*VMContext // one per old-style Input
	}

	VMContext struct {
		TxRefDataHash Hash
		RefDataHash   Hash
		TxSigHash     Hash
		OutputID      *Hash
		EntryID       Hash
		NonceID       *Hash
	}
)

func (t TxHashes) SigHash(entryRef Hash) Hash {
	return t.VMContexts[entryRef].TxSigHash
}

// BlockHeaderHashFunc is initialized to a function in protocol/tx
// that can compute the hash of a blockheader. It is a variable here
// to avoid a circular dependency between the bc and tx packages.
var BlockHeaderHashFunc func(*BlockHeader) (Hash, error)
