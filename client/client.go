package client

import (
	"context"

	"io"

	"github.com/kitelabs-io/aptstream/filter"
	v1 "github.com/kitelabs-io/aptstream/grpc/aptos/indexer/v1"
	"github.com/kitelabs-io/aptstream/stream"
	"github.com/kitelabs-io/aptstream/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type StreamFilter struct {
	// if FromVersion is nil and versionTracker.GetVersion() == 0, it will start from the latest version
	// if FromVersion is not nil, it will start from the version specified
	FromVersion *uint64

	// if Filter is nil, it will return all transactions
	Filter filter.Filter
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
	versionTracker version.VersionTracker

	// commitStrategy determines how transaction versions are committed; delegated to stream.
	commitStrategy stream.CommitStrategy
	logger         *zerolog.Logger
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
		c.versionTracker = &version.InMemoryVersionTracker{}
	}

	if c.logger == nil {
		logger := log.Logger.With().Str("client", "aptosstream").Logger()
		c.logger = &logger
	}
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
func (c *Client) GetTransactionStream(ctx context.Context, filter StreamFilter) (*stream.TransactionStream, error) {
	if c.outgoingHeader != nil {
		ctx = metadata.NewOutgoingContext(ctx, c.outgoingHeader)
	}
	// Create cancellable context for the stream
	ctx, cancel := context.WithCancel(ctx)
	txCh := make(chan *stream.CommittableTransaction, c.bufferSize)
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
	grpcStream, err := c.aptosRawDataClient.GetTransactions(ctx, &v1.GetTransactionsRequest{
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
			txns, err := grpcStream.Recv()
			if err != nil {
				finalErr = err
				break mainLoop
			}

			for _, tx := range txns.Transactions {
				if filter.Filter == nil || filter.Filter.Match(tx) {
					committableTx := stream.NewCommittableTransaction(tx, c.versionTracker.SetVersion)
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

	return stream.NewTransactionStream(txCh, errCh, cancel, c.commitStrategy, c.logger), nil
}
