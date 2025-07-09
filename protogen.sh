#!/bin/bash
protodep up

GO_PACKAGE_PATH="github.com/kitelabs-io/aptstream/grpc"

cd grpc && protoc -I=./ \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go_opt=Maptos/util/timestamp/timestamp.proto=$GO_PACKAGE_PATH/aptos/util/timestamp \
  --go_opt=Maptos/transaction/v1/transaction.proto=$GO_PACKAGE_PATH/aptos/transaction/v1 \
  --go_opt=Maptos/indexer/v1/raw_data.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go_opt=Maptos/indexer/v1/grpc.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go_opt=Maptos/indexer/v1/filter.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go_opt=Maptos/internal/fullnode/v1/fullnode_data.proto=$GO_PACKAGE_PATH/aptos/internal/fullnode/v1 \
  --go_opt=Maptos/remote_executor/v1/network_msg.proto=$GO_PACKAGE_PATH/aptos/remote_executor/v1 \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  --go-grpc_opt=Maptos/util/timestamp/timestamp.proto=$GO_PACKAGE_PATH/aptos/util/timestamp \
  --go-grpc_opt=Maptos/transaction/v1/transaction.proto=$GO_PACKAGE_PATH/aptos/transaction/v1 \
  --go-grpc_opt=Maptos/indexer/v1/raw_data.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go-grpc_opt=Maptos/indexer/v1/grpc.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go-grpc_opt=Maptos/indexer/v1/filter.proto=$GO_PACKAGE_PATH/aptos/indexer/v1 \
  --go-grpc_opt=Maptos/internal/fullnode/v1/fullnode_data.proto=$GO_PACKAGE_PATH/aptos/internal/fullnode/v1 \
  --go-grpc_opt=Maptos/remote_executor/v1/network_msg.proto=$GO_PACKAGE_PATH/aptos/remote_executor/v1 \
  $(find . -name "*.proto")
