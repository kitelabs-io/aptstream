package stream

import (
	"context"
	"io"

	transaction "github.com/kitelabs-io/aptstream/grpc/aptos/transaction/v1"
	"github.com/rs/zerolog"
)

// CommitStrategy determines how transaction versions are committed in the stream.
type CommitStrategy int

const (
	// AutoCommit automatically commits the transaction before returning it in Get().
	AutoCommit CommitStrategy = iota
	// ManualCommit requires the caller to explicitly call Commit() on the received transaction.
	ManualCommit
)

// TransactionStream wraps a stream of transactions and potential errors with cancellation support.
// Use Get() to retrieve the next *transaction.Transaction or an error (io.EOF indicates end of stream).
// Call Close() to cancel the stream; subsequent Get() calls will return the context error.
type TransactionStream struct {
	txCh   <-chan *CommittableTransaction
	errCh  <-chan error
	cancel context.CancelFunc
	// commitStrategy controls auto-commit behavior in Get()
	commitStrategy CommitStrategy
	// logger for error logging during auto-commit
	logger *zerolog.Logger
}

// CommittableTransaction wraps a transaction with a Commit method to acknowledge its processing.
type CommittableTransaction struct {
	Transaction *transaction.Transaction
	committed   bool
	commitFn    func(version uint64) error
}

// Commit informs the VersionTracker that the transaction has been successfully processed.
// It is safe to call multiple times.
func (c *CommittableTransaction) Commit() error {
	if c.committed {
		return nil
	}
	err := c.commitFn(c.Transaction.GetVersion())
	if err == nil {
		c.committed = true
	}
	return err
}

// Get returns the next transaction or an error. io.EOF indicates normal completion.
// If the stream was canceled via Close(), Get returns the context error (context.Canceled).
func (s *TransactionStream) Get() (*CommittableTransaction, error) {
	// Block and read only from the transaction channel.
	tx, ok := <-s.txCh
	if !ok {
		// The channel is closed. This is the ONLY time we check for a terminal error.
		// This read will not block because the producer guarantees to send an error
		// or nil before it finishes.
		err := <-s.errCh
		if err == nil {
			return nil, io.EOF // Clean shutdown
		}
		return nil, err
	}
	// Auto-commit logic: commit before handing transaction to user
	if s.commitStrategy == AutoCommit {
		if err := tx.Commit(); err != nil {
			s.logger.Error().Err(err).Msg("auto-commit failed")
			return tx, err
		}
	}
	return tx, nil
}

// Close cancels the underlying stream context. Pending or future Get() calls will return the context error.
func (s *TransactionStream) Close() {
	s.cancel()
}

// NewTransactionStream creates a new TransactionStream with provided channels, cancel function, commit strategy, and logger.
func NewTransactionStream(txCh <-chan *CommittableTransaction, errCh <-chan error, cancel context.CancelFunc, commitStrategy CommitStrategy, logger *zerolog.Logger) *TransactionStream {
	return &TransactionStream{txCh: txCh, errCh: errCh, cancel: cancel, commitStrategy: commitStrategy, logger: logger}
}

// NewCommittableTransaction creates a CommittableTransaction with given transaction and commit function.
func NewCommittableTransaction(tx *transaction.Transaction, commitFn func(version uint64) error) *CommittableTransaction {
	return &CommittableTransaction{Transaction: tx, commitFn: commitFn}
}
