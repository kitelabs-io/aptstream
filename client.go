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
	txCh   <-chan *transaction.Transaction
	errCh  <-chan error
	cancel context.CancelFunc
}

// Get returns the next transaction or an error. io.EOF indicates normal completion.
// If the stream was canceled via Close(), Get returns the context error (context.Canceled).
func (s *TransactionStream) Get() (*transaction.Transaction, error) {
	select {
	case tx, ok := <-s.txCh:
		if ok {
			return tx, nil
		}
		// txCh closed: read final error
		err := <-s.errCh
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	case err := <-s.errCh:
		return nil, err
	}
}

// Close cancels the underlying stream context. Pending or future Get() calls will return the context error.
func (s *TransactionStream) Close() {
	s.cancel()
}

// GetTransactionStream starts fetching transactions matching the provided filter and returns a TransactionStream.
//
// IMPORTANT: This function uses the provided VersionTracker to determine the starting version for the stream.
// However, it does NOT automatically update the version tracker as transactions are received.
// It is the caller's responsibility to process transactions and then call versionTracker.SetVersion()
// to ensure at-least-once processing semantics and prevent data loss on client restart.
//
// Use Get() to receive transactions and errors, and Close() to cancel the stream at any time.
func (c *Client) GetTransactionStream(ctx context.Context, filter TransactionFilter) (*TransactionStream, error) {
	if c.outgoingHeader != nil {
		ctx = metadata.NewOutgoingContext(ctx, c.outgoingHeader)
	}
	// Create cancellable context for the stream
	ctx, cancel := context.WithCancel(ctx)
	txCh := make(chan *transaction.Transaction, c.bufferSize)
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
		defer close(txCh)
		defer close(errCh)
		for {
			txns, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					errCh <- nil
				} else {
					errCh <- err
				}
				return
			}

			for _, tx := range txns.Transactions {
				if filter.Filter == nil || filter.Filter.match(tx) {
					select {
					case txCh <- tx:
					case <-ctx.Done():
						errCh <- ctx.Err()
						return
					}
				}
			}
		}
	}()

	return &TransactionStream{txCh: txCh, errCh: errCh, cancel: cancel}, nil
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

func WithLogger(logger *zerolog.Logger) opt {
	return func(c *Client) {
		c.logger = logger
	}
}
