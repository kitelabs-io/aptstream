package client

import (
	"github.com/kitelabs-io/aptstream/stream"
	"github.com/kitelabs-io/aptstream/version"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/metadata"
)

type opt func(*Client)

func WithApiKey(apiKey string) opt {
	return func(c *Client) {
		c.outgoingHeader = metadata.MD{
			"authorization": {"Bearer " + apiKey},
		}
	}
}

func WithVersionTracker(versionTracker version.VersionTracker) opt {
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

func WithCommitStrategy(strategy stream.CommitStrategy) opt {
	return func(c *Client) {
		c.commitStrategy = strategy
	}
}

func WithLogger(logger *zerolog.Logger) opt {
	return func(c *Client) {
		c.logger = logger
	}
}
