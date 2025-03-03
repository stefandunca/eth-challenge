# Challenge

Run example

```sh
go run main.go --address=0x761d53b47334bee6612c0bd1467fb881435375b2 --topic=0x3e54d0825ed78523037d00a81759237eb436ce774bd546993ee67a1b67b6e766 --rpcs="https://eth-sepolia-testnet.rpc.grove.city/v1/<TODO: API_KEY>;https://sepolia.infura.io/v3/<TODO: API_KEY>;https://rpc-sepolia.rockx.com/"
```

## Design Considerations

Fetching:

- Connect to multiple nodes and request information in parallel using a pool of workers
  - goroutines should be enough as long as we don't reach the max connections limit
- Ensure node RPC limits are not abused: have node specific limits and request with exponential backoff and jitter for transient errors
- Use an interval queue and command pattern to manage the requests in parallel
- Configuration for ease of debugging and testing

Store:

- Options
  - **BoltDB**: ACID compliance, high read throughput
  - **LevelDB**: Atomic batches, hig write throughput
  - **BadgerDB**: Fastest write throughput
  - **Redis**: In-memory, blazing fast read/write throughput
- Indexing

## Future improvements

- [ ] handled gracefully service interruption and fetching resumption
- [ ] integration tests
- [ ] basic observability
- [ ] CI to run linting and test on PRs
- [ ] address edge-cases
  - all workers are stuck or stalled in retrying
