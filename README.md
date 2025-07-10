# Aptos Stream Client for Go

A Go client for streaming transactions from the Aptos blockchain via gRPC. This library provides a simple and efficient way to subscribe to and process Aptos transactions in real-time, with support for filtering, custom commit strategies, and automatic version tracking.

## Features

-   **Real-time Transaction Streaming**: Connect to an Aptos Indexer gRPC endpoint and receive transactions as they are confirmed.
-   **Powerful Filtering**: Filter transactions by type, success status, sender address, function calls, and events.
-   **At-Least-Once Delivery**: Choose between `ManualCommit` and `AutoCommit` strategies to control when a transaction version is marked as processed, ensuring you don't miss data.
-   **Automatic Reconnection & Resumption**: The client can automatically resume from the last committed transaction version upon restart (when using a persistent `VersionTracker`).
-   **Simple API**: A clean and easy-to-use API for getting started quickly.

## Prerequisites

-   Go 1.22 or later.
-   An API key for an Aptos Indexer service that supports the streaming gRPC API.

## Installation

```sh
go get github.com/kitelabs-io/aptstream
```

## Usage & Example

Below is a complete example of how to use the client to stream successful user transactions, and manually commit each one after processing.

### Example Code

```go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/kitelabs-io/aptstream/client"
	"github.com/kitelabs-io/aptstream/filter"
	"github.com/kitelabs-io/aptstream/stream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// Replace with your Aptos Indexer gRPC endpoint
	grpcEndpoint = "your-grpc-endpoint:9001"
	apiKey       = "your-api-key"
)

func main() {
	// Establish a gRPC connection to the Aptos Indexer.
	// In production, you should use TLS credentials.
	conn, err := grpc.NewClient(
		grpcEndpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(30000000)), // Increase max message size
	)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Create a new Aptos Stream client.
	aptosClient := client.NewClient(
		conn,
		client.WithApiKey(apiKey),
		// Use ManualCommit for stronger at-least-once delivery guarantees.
		// The version is only saved when you explicitly call tx.Commit().
		client.WithCommitStrategy(stream.ManualCommit),
	)

	var success = true
	var sender = "0xa"
	var startVersion = uint64(2930428174)

	// Define a filter to get only successful user transactions from a specific sender.
	streamFilter := client.StreamConfig{
		FromVersion: &startVersion,
		Filter: &filter.And{
			Filters: []filter.Filter{
				&filter.TransactionRootFilter{
					Success: &success,
				},
				&filter.UserTransactionFilter{
					Sender: &sender,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the transaction stream.
	txStream, err := aptosClient.GetTransactionStream(ctx, streamFilter)
	if err != nil {
		log.Fatalf("failed to get transaction stream: %v", err)
	}
	defer txStream.Close()

	fmt.Println("Waiting for transactions...")

	// Loop to receive transactions from the stream.
	for {
		// Get() blocks until a transaction is received or the stream is closed.
		committableTx, err := txStream.Get()
		if err != nil {
			// io.EOF indicates a clean shutdown of the stream.
			if err == io.EOF {
				fmt.Println("Stream finished gracefully.")
			} else {
				fmt.Printf("Stream error: %v\n", err)
			}
			return
		}

		tx := committableTx.Transaction
		fmt.Printf(
			"Received transaction version %d, Hash: %s\n",
			tx.GetVersion(),
			tx.GetInfo().GetHash(),
		)

		// Your processing logic here...

		// Manually commit the transaction to update the version tracker.
		if err := committableTx.Commit(); err != nil {
			log.Printf("failed to commit transaction %d: %v", tx.GetVersion(), err)
			// Decide how to handle commit failures. You might retry or terminate.
			return
		}
		fmt.Printf("Committed version %d\n", tx.GetVersion())
	}
}
```

### Explanation

1.  **`grpc.NewClient`**: Establishes the connection to your Aptos gRPC provider.
    *   `grpc.WithTransportCredentials(credentials.NewTLS(nil))`: Configures a secure TLS connection, which is recommended for production environments.
    *   `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(30000000))`: This option increases the maximum message size the client can receive. It's important for Aptos streams, as transaction batches can be large.
2.  **`client.NewClient`**: Creates the stream client instance.
    *   **`client.WithApiKey`**: Sets the required authentication for the service.
    *   **`client.WithCommitStrategy`**: We use `stream.ManualCommit`. This means we are responsible for calling `committableTx.Commit()` after we have successfully processed a transaction. If our application crashes before we commit, it will restart from the last committed version, preventing data loss. The default is `stream.AutoCommit`, where the commit happens automatically before `Get()` returns.
3.  **`client.StreamConfig`**: Defines what transactions we are interested in.
    *   **`FromVersion`**: We specify a `startVersion`, so the stream will begin from that transaction. If nil, it starts from the latest or resumes from a `VersionTracker`.
    *   **`Filter`**: A powerful way to select transactions. Here we use `filter.And` to combine multiple criteria: we request transactions that are both successful (`Success: true`) AND sent by a specific address (`Sender: "0xa"`). You can combine many filters this way.
4.  **`aptosClient.GetTransactionStream`**: Initiates the stream with the server.
5.  **`txStream.Get()`**: Waits for the next transaction. It returns an `io.EOF` error when the stream is closed normally (e.g., if you set a `WithTransactionsCount` limit or the server closes the connection).
6.  **`committableTx.Commit()`**: Since we used `ManualCommit`, we must call this to acknowledge the transaction. This updates the internal `VersionTracker`.
7.  **`txStream.Close()`**: It's good practice to `defer` the `Close()` call to ensure the stream's context is canceled on exit.

## API

For more details on available options and filters, see the package documentation:

-   `client`: For client creation and configuration.
-   `stream`: For stream interaction and commit strategies.
-   `filter`: For all available transaction filters.
-   `version`: For version tracking mechanisms.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

This project is licensed under the MIT License. 
