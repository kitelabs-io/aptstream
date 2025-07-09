package aptosstream

import (
	"context"

	"io"

	v1 "github.com/kitelabs-io/aptstream/grpc/aptos/indexer/v1"
	transaction "github.com/kitelabs-io/aptstream/grpc/aptos/transaction/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type CommitStrategy int

const (
	// AutoCommit automatically commits the transaction version before it is returned from Get().
	// This is the default strategy.
	AutoCommit CommitStrategy = iota
	// ManualCommit requires the user to explicitly call Commit() on a transaction.
	ManualCommit
)

type TransactionFilter struct {
	// if FromVersion is nil and versionTracker.GetVersion() == 0, it will start from the latest version
	// if FromVersion is not nil, it will start from the version specified
	FromVersion *uint64

	// if Filter is nil, it will return all transactions
	Filter txnFilter
}

var (
	defaultBatchSize  = uint64(1000)
	defaultBufferSize = uint64(1000)
)

type Client struct {
	aptosRawDataClient v1.RawDataClient
	outgoingHeader     metadata.MD
	// number of transactions to return in each stream
	// if TransactionsCount is nil, it will return an infinite stream of transactions
	transactionsCount *uint64

	// max number of transactions returned a turn
	// if BatchSize is nil, it will use the default batch size 1000.
	batchSize *uint64

	// buffer size for the stream
	// if BufferSize is nil, it will use the default buffer size 1000.
	bufferSize uint64

	// versionTracker is used to get the latest tracked version
	// in case consume from the latest version, versionTracker.GetVersion() must return 0
	versionTracker VersionTracker

	// commitStrategy determines how transaction versions are committed.
	// The default is AutoCommit.
	commitStrategy CommitStrategy

	logger *zerolog.Logger
}

func NewClient(cc grpc.ClientConnInterface, opts ...opt) *Client {
	c := &Client{
		aptosRawDataClient: v1.NewRawDataClient(cc),
	}
	c.applyOpts(opts...)
	return c
}

func (c *Client) applyOpts(opts ...opt) {
	for _, opt := range opts {
		opt(c)
	}

	// assign default values to the fields
	if c.batchSize == nil || *c.batchSize == 0 {
		c.batchSize = &defaultBatchSize
	}

	if c.bufferSize == 0 {
		c.bufferSize = defaultBufferSize
	}

	if c.versionTracker == nil {
		c.versionTracker = &InMemoryVersionTracker{}
	}

	if c.logger == nil {
		logger := log.Logger.With().Str("client", "aptosstream").Logger()
		c.logger = &logger
	}
}

// TransactionStream wraps a stream of transactions and potential errors with cancellation support.
// Use Get() to retrieve the next *transaction.Transaction or an error (io.EOF indicates end of stream).
// Call Close() to cancel the stream; subsequent Get() calls will return the context error.
type TransactionStream struct {
	txCh   <-chan *CommittableTransaction
	errCh  <-chan error
	cancel context.CancelFunc
	client *Client
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

	// If we got a transaction, process it.
	if s.client.commitStrategy == AutoCommit {
		// In AutoCommit mode, we commit *before* handing the transaction to the user.
		// This provides at-least-once semantics. If the client crashes after
		// receiving but before processing, the transaction will be re-sent on restart.
		if err := tx.Commit(); err != nil {
			// A commit failure is critical. We log it and forward the error to the user.
			s.client.logger.Error().Err(err).Msg("auto-commit failed")
			// Return both the transaction and the commit error. The user might still
			// want to process the transaction data.
			return tx, err
		}
	}
	return tx, nil
}

// Close cancels the underlying stream context. Pending or future Get() calls will return the context error.
func (s *TransactionStream) Close() {
	s.cancel()
}

// GetTransactionStream starts fetching transactions and returns a TransactionStream.
//
// The client can be configured with two commit strategies using WithCommitStrategy():
//   - AutoCommit (default): The stream automatically calls Commit() on a transaction *before* it is
//     returned by Get(). This is convenient but has slightly weaker guarantees. If the
//     client crashes after Get() returns but before processing is complete, the transaction
//     may be skipped on restart.
//   - ManualCommit: The caller is responsible for calling Commit() on each
//     received CommittableTransaction. This provides stronger "at-least-once" processing guarantees.
//     The VersionTracker is only updated when Commit() is called, ensuring that if the
//     client restarts, it will resume from the last explicitly committed transaction.
//
// It is crucial to handle the returned error from Get(), which may indicate a stream
// failure or a commit failure in AutoCommit mode.
func (c *Client) GetTransactionStream(ctx context.Context, filter TransactionFilter) (*TransactionStream, error) {
	if c.outgoingHeader != nil {
		ctx = metadata.NewOutgoingContext(ctx, c.outgoingHeader)
	}
	// Create cancellable context for the stream
	ctx, cancel := context.WithCancel(ctx)
	txCh := make(chan *CommittableTransaction, c.bufferSize)
	errCh := make(chan error, 1)

	// Initialize FromVersion if not set
	if filter.FromVersion == nil {
		version, err := c.versionTracker.GetVersion()
		if err != nil {
			cancel()
			return nil, err
		}
		if version > 0 {
			filter.FromVersion = &version
		}
	}

	// Open the gRPC stream
	stream, err := c.aptosRawDataClient.GetTransactions(ctx, &v1.GetTransactionsRequest{
		StartingVersion:   filter.FromVersion,
		TransactionsCount: c.transactionsCount,
		BatchSize:         c.batchSize,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		var finalErr error
	mainLoop:
		for {
			txns, err := stream.Recv()
			if err != nil {
				finalErr = err
				break mainLoop
			}

			for _, tx := range txns.Transactions {
				if filter.Filter == nil || filter.Filter.match(tx) {
					committableTx := &CommittableTransaction{
						Transaction: tx,
						commitFn:    c.versionTracker.SetVersion,
					}
					select {
					case txCh <- committableTx:
					case <-ctx.Done():
						finalErr = ctx.Err()
						break mainLoop
					}
				}
			}
		}

		// After the producer loop exits, we must close the transaction channel
		// to signal to the consumer that no more items are coming.
		close(txCh)

		// Then, we send the final error status. This is read by the consumer
		// only after it has drained the transaction channel.
		if finalErr != io.EOF {
			errCh <- finalErr
		} else {
			errCh <- nil // Clean EOF
		}
		close(errCh)
	}()

	return &TransactionStream{txCh: txCh, errCh: errCh, cancel: cancel, client: c}, nil
}

type opt func(*Client)

func WithApiKey(apiKey string) opt {
	return func(c *Client) {
		c.outgoingHeader = metadata.MD{
			"authorization": {"Bearer " + apiKey},
		}
	}
}

func WithVersionTracker(versionTracker VersionTracker) opt {
	return func(c *Client) {
		c.versionTracker = versionTracker
	}
}

// if transactionsCount is 0, it will return an infinite stream of transactions.
func WithTransactionsCount(transactionsCount uint64) opt {
	return func(c *Client) {
		if transactionsCount == 0 {
			c.transactionsCount = nil
		} else {
			c.transactionsCount = &transactionsCount
		}
	}
}

// if batchSize is 0, it will use the default batch size 1000.
func WithBatchSize(batchSize uint64) opt {
	return func(c *Client) {
		c.batchSize = &batchSize
	}
}

func WithBufferSize(bufferSize uint64) opt {
	return func(c *Client) {
		c.bufferSize = bufferSize
	}
}

func WithCommitStrategy(strategy CommitStrategy) opt {
	return func(c *Client) {
		c.commitStrategy = strategy
	}
}

func WithLogger(logger *zerolog.Logger) opt {
	return func(c *Client) {
		c.logger = logger
	}
}
