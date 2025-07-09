package aptosstream

import (
	"context"

	"log"

	v1 "github.com/kitelabs-io/aptstream/grpc/aptos/indexer/v1"
	transaction "github.com/kitelabs-io/aptstream/grpc/aptos/transaction/v1"
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
}

func (c *Client) GetTransaction(ctx context.Context, filter TransactionFilter) (chan *transaction.Transaction, error) {
	if c.outgoingHeader != nil {
		ctx = metadata.NewOutgoingContext(ctx, c.outgoingHeader)
	}

	result := make(chan *transaction.Transaction, c.bufferSize)

	if filter.FromVersion == nil {
		version, err := c.versionTracker.GetVersion()
		if err != nil {
			return nil, err
		}
		if version > 0 {
			filter.FromVersion = &version
		}
	}

	stream, err := c.aptosRawDataClient.GetTransactions(ctx, &v1.GetTransactionsRequest{
		StartingVersion:   filter.FromVersion,
		TransactionsCount: c.transactionsCount,
		BatchSize:         c.batchSize,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(result)
		for {
			txns, err := stream.Recv()
			if err != nil {
				log.Printf("error receiving transactions: %v", err)
				return
			}
			var receivedVersion uint64
			for _, tx := range txns.Transactions {
				if filter.Filter == nil || filter.Filter.match(tx) {
					result <- tx
				}
				receivedVersion = tx.GetVersion()
			}
			if err := c.versionTracker.SetVersion(receivedVersion); err != nil {
				log.Printf("error setting version: %v", err)
				return
			}
		}
	}()

	return result, nil
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
