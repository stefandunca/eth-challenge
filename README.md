# Challenge

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

