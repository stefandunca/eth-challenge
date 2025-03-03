package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/stefandunca/eth-challenge/blockchain"
	"github.com/stefandunca/eth-challenge/processing"
)

const (
	maxRPCCallsPerSecond = 5
)

type LogData struct {
	Log        types.Log
	BlockTime  uint64
	ParentHash common.Hash
	L1InfoRoot []byte
}

type logDataOrError struct {
	logData *LogData
	err     *error
}

type rpcCall func(context.Context, *ethclient.Client) error

// main runs a processing pipeline
func main() {
	// Define flags
	hexAddress := flag.String("address", "", "contract HEX address")
	hexTopic := flag.String("topic", "", "topic HEX hash")
	rpcsParam := flag.String("rpcs", "https://rpc-sepolia.rockx.com/", "RPC endpoints")

	// Parse the flags
	flag.Parse()

	if *hexAddress == "" || *hexTopic == "" {
		fmt.Println("Error: You must specify a contract and a topic")
		os.Exit(1)
	}

	rpcURLs := strings.Split(*rpcsParam, ";")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requester := processing.RequestPool{}

	// Input RPC calls
	inRPC := make(chan rpcCall, len(rpcURLs))
	// Jobs to retry
	failedRPC := make(chan rpcCall, len(rpcURLs))
	// Result of input calls that trigger side RPC calls
	out := make(chan []types.Log, len(rpcURLs))
	// Side RPC calls resulted from out processing
	sideRPC := make(chan rpcCall, len(rpcURLs))
	res := make(chan *LogData, len(rpcURLs))

	contractAddr := common.HexToAddress(*hexAddress)
	topic := common.HexToHash(*hexTopic)

	//
	// Step 1: request the last block number
	inRPC <- func(cxt context.Context, c *ethclient.Client) error {
		// lastNo, err := c.BlockNumber(ctx)
		// if err != nil {
		// 	return err
		// }
		lastNo := uint64(500000)

		go filterLogsBatched(ctx, inRPC, out, contractAddr, topic, lastNo)
		return nil
	}

	var outDone int32
	//
	// Step 2: add workers to process RPC messages
	for _, url := range rpcURLs {
		requester.AddWorkerWithRetry(ctx, func(boff *backoff.ExponentialBackOff) (res struct{}, err error) {
			var job rpcCall
			var client *ethclient.Client
			for {
				// If no failed jobs or side jobs take it from the input
				select {
				case next := <-failedRPC:
					job = next
				case next, open := <-sideRPC:
					if !open {
						// Side finished, stop worker
						return struct{}{}, nil
					}
					job = next
				default:
					select {
					case next, open := <-inRPC:
						if !open {
							outDone := atomic.AddInt32(&outDone, 1)
							if outDone == 1 {
								// Signal finished to deliver results
								close(out)
							}
							time.Sleep(time.Millisecond)
							continue
						}
						job = next
					case <-time.After(time.Microsecond):
						continue
					}
				}

				if client == nil {
					client, err = blockchain.NewSepoliaClient(ctx, url, 11155111)
					if err != nil {
						log.Println("@dd new client FAILED; err", err)
						failedRPC <- job
						return struct{}{}, err
					}
				}

				log.Println("@dd processing", job)

				// TODO check lastSecondCallCount > maxRPCCallsPerSecond

				err := job(ctx, client)
				if err != nil {
					// If failed put it back
					failedRPC <- job
					return struct{}{}, err
				}

				// reset backoff timer in case of success
				boff.Reset()
			}
		}, 10)
	}

	//
	// Step 3: Process logs
	var outWG sync.WaitGroup
	for i := 0; i < len(rpcURLs); i++ {
		outWG.Add(1)
		go func() {
			// TODO: slice RPC batching
			// TODO: reduce calls to HeaderByNumber by catching
			for logs := range out {
				for _, l := range logs {
					sideRPC <- func(ctx context.Context, c *ethclient.Client) error {
						header, err := c.HeaderByNumber(ctx, big.NewInt(int64(l.BlockNumber)))
						if err != nil {
							return fmt.Errorf("block %d fetch error: %w", l.BlockNumber, err)
						}
						res <- &LogData{
							Log:        l,
							BlockTime:  header.Time,
							ParentHash: header.ParentHash,
							L1InfoRoot: l.Data,
						}
						return nil
					}
				}
			}
			outWG.Done()
		}()
	}
	go func() {
		outWG.Wait()

		// Signal results are finalized
		close(sideRPC)
		close(res)
	}()

	//
	// Step 4: Store results
	var resWG sync.WaitGroup
	resWG.Add(1)
	go func() {
		for r := range res {
			fmt.Println("@dd ", r)
		}
		resWG.Done()
	}()

	requester.Wait()
	resWG.Wait()
}

// filterLogsBatched adds RPC call jobs to in channel which upon finish adds to output channel
func filterLogsBatched(ctx context.Context, in chan rpcCall, out chan []types.Log, contractAddr common.Address, topic common.Hash, lastBlockNo uint64) {
	gen := blockchain.NewLogBatchGenerator(contractAddr, topic, 0, lastBlockNo)
	for {
		query, err := gen.Next()
		if err != nil {
			// ErrorNoMoreBlocks
			break
		}
		in <- func(ctx context.Context, client *ethclient.Client) error {
			logs, err := client.FilterLogs(ctx, *query)
			if err != nil {
				return fmt.Errorf("batch %d-%d failed: %w", query.FromBlock.Uint64(), query.ToBlock.Uint64(), err)
			}
			if len(logs) > 0 {
				log.Println("Found logs count:", len(logs))
				out <- logs
			} else {
				log.Println("@dd Nothing! from", query.FromBlock.Uint64(), "to", query.ToBlock.Uint64())
			}

			return nil
		}
	}

	// Signal all blocks fetched
	close(in)
}
