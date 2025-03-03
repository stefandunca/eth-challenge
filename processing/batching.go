package processing

import (
	"context"
	"log"
	"sync"

	"github.com/cenkalti/backoff/v5"
)

// Job defines an interface for a task that can be executed.
type Job interface {
	Execute() error
}

// RequestPool is a pool of worker functions that
type RequestPool struct {
	wg sync.WaitGroup
}

func (rp *RequestPool) Wait() {
	rp.wg.Wait()
}

// AddWorkerWithRetry launches a worker in a new goroutine that executes the workerFn.
// It uses an exponential backoff strategy with a limit on the maximum number of retries.
func (rp *RequestPool) AddWorkerWithRetry(ctx context.Context, workerFn func(*backoff.ExponentialBackOff) (struct{}, error), retryCount uint) {
	boff := backoff.NewExponentialBackOff()

	rp.wg.Add(1)
	go func() {
		// TODO
		/*res*/
		_, err := backoff.Retry(ctx, func() (struct{}, error) {
			return workerFn(boff)
		}, backoff.WithBackOff(boff), backoff.WithMaxTries(retryCount))
		if err != nil {
			log.Printf("Worker failed with %d retries; err %v\n", retryCount, err)
		}
		rp.wg.Done()
	}()
}
