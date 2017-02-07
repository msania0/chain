package txbuilder

import (
	"bytes"
	"context"

	"chain/core/rpc"
	"chain/errors"
	"chain/protocol"
	"chain/protocol/bc"
	"chain/protocol/validation"
	"chain/protocol/vm"
)

var (
	// ErrRejected means the network rejected a tx (as a double-spend)
	ErrRejected = errors.New("transaction rejected")

	ErrMissingRawTx        = errors.New("missing raw tx")
	ErrBadInstructionCount = errors.New("too many signing instructions in template")
)

// Submitter submits a transaction to the generator so that it may
// be confirmed in a block.
type Submitter interface {
	Submit(ctx context.Context, tx *bc.Transaction) error
}

// FinalizeTx validates a transaction signature template,
// assembles a fully signed tx, and stores the effects of
// its changes on the UTXO set.
func FinalizeTx(ctx context.Context, c *protocol.Chain, s Submitter, tx *bc.Transaction) error {
	err := checkTxSighashCommitment(tx)
	if err != nil {
		return err
	}

	// Make sure there is at least one block in case client is trying to
	// finalize a tx before the initial block has landed
	<-c.BlockWaiter(1)

	// If this transaction is valid, ValidateTxCached will store it in the cache.
	err = c.ValidateTxCached(tx)
	if errors.Root(err) == validation.ErrBadTx {
		return errors.Sub(ErrRejected, err)
	}
	if err != nil {
		return errors.Wrap(err, "tx rejected")
	}

	err = s.Submit(ctx, tx)
	return errors.Wrap(err)
}

var (
	// ErrNoTxSighashCommitment is returned when no input commits to the
	// complete transaction.
	// To permit idempotence of transaction submission, we require at
	// least one input to commit to the complete transaction (what you get
	// when you build a transaction with allow_additional_actions=false).
	ErrNoTxSighashCommitment = errors.New("no commitment to tx sighash")

	// ErrNoTxSighashAttempt is returned when there was no attempt made to sign
	// this transaction.
	ErrNoTxSighashAttempt = errors.New("no tx sighash attempted")

	// ErrTxSignatureFailure is returned when there was an attempt to sign this
	// transaction, but it failed.
	ErrTxSignatureFailure = errors.New("tx signature was attempted but failed")
)

func checkTxSighashCommitment(tx *bc.Transaction) error {
	var lastError error

	check := func(args [][]byte, inpHash bc.Hash) error {
		// xxx what's the difference between the three errors returned here?
		switch len(args) {
		case 0:
			return ErrNoTxSighashAttempt
		case 1, 2:
			return ErrTxSignatureFailure
		}
		prog := args[len(args)-1]
		if len(prog) != 35 {
			return ErrNoTxSighashCommitment
		}
		if prog[0] != byte(vm.OP_DATA_32) {
			return ErrNoTxSighashCommitment
		}
		if !bytes.Equal(prog[33:], []byte{byte(vm.OP_TXSIGHASH), byte(vm.OP_EQUAL)}) {
			return ErrNoTxSighashCommitment
		}
		h := tx.SigHash(inpHash)
		if !bytes.Equal(h[:], prog[1:33]) {
			return ErrNoTxSighashCommitment
		}
		return nil
	}

	for _, spRef := range tx.Spends {
		sp := spRef.Entry.(*bc.Spend)
		err := check(sp.Arguments(), spRef.Hash())
		if err == nil {
			return nil
		}
		lastError = err
	}

	for _, issRef := range tx.Issuances {
		iss := issRef.Entry.(*bc.Spend)
		err := check(iss.Arguments(), issRef.Hash())
		if err == nil {
			return nil
		}
		lastError = err
	}

	return lastError
}

// RemoteGenerator implements the Submitter interface and submits the
// transaction to a remote generator.
// TODO(jackson): This implementation maybe belongs elsewhere.
type RemoteGenerator struct {
	Peer *rpc.Client
}

func (rg *RemoteGenerator) Submit(ctx context.Context, tx *bc.Transaction) error {
	err := rg.Peer.Call(ctx, "/rpc/submit", tx, nil)
	err = errors.Wrap(err, "generator transaction notice")
	return err
}
