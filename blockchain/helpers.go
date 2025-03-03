package blockchain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func NewSepoliaClient(ctx context.Context, rawURL string, chainID uint64) (*ethclient.Client, error) {
	client, err := ethclient.DialContext(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("RPC connection failed: %w", err)
	}

	clientChainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("eth_chainId failed: %w", err)
	}
	if clientChainID.Uint64() != chainID {
		return nil, errors.New("invalid Sepolia chain ID; expecting 11155111")
	}

	return client, nil
}

type LogBatchGenerator struct {
	queryTemplate ethereum.FilterQuery
	nextBlock     uint64
	lastBlock     uint64

	mu sync.Mutex
}

const MaxBlocksPerBatch = 100000 // Avoids RPC timeouts

var ErrorNoMoreBlocks = errors.New("no more blocks")

func NewLogBatchGenerator(address common.Address, topic common.Hash, fromBlock uint64, toBlock uint64) *LogBatchGenerator {
	return &LogBatchGenerator{
		queryTemplate: ethereum.FilterQuery{
			Addresses: []common.Address{address},
			Topics:    [][]common.Hash{{topic}},
		},
		nextBlock: fromBlock,
		lastBlock: toBlock,
	}
}

func (lb *LogBatchGenerator) Next() (*ethereum.FilterQuery, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	from := lb.nextBlock
	if from > lb.lastBlock {
		return nil, ErrorNoMoreBlocks
	}
	to := min(from+MaxBlocksPerBatch-1, lb.lastBlock)

	batchQuery := lb.queryTemplate
	batchQuery.FromBlock = new(big.Int).SetUint64(from)
	batchQuery.ToBlock = new(big.Int).SetUint64(to)

	lb.nextBlock = to + 1

	return &batchQuery, nil
}
