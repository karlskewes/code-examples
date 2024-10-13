package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type result struct {
	input  int
	output int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	const workers = 10
	const inputs = 100

	output, err := manager(ctx, inputs, workers, task)
	if err != nil {
		fmt.Printf("output: %v\nerror: %v", output, err)
		os.Exit(1)
	}

	fmt.Printf("output: %v\n", output)
}

func manager(
	ctx context.Context,
	inputs int,
	workers int,
	do func(context.Context, int) (int, error),
) (map[int]int, error) {
	var (
		output                   = make(map[int]int)          // map[<input>]<result>
		jobs                     = make(chan int, 1)          // Consider len(<inputs>)
		results                  = make(chan result, workers) // GC to clean up.
		errs                     = make(chan error)           // GC to clean up.
		workerCtx, workersCancel = context.WithCancel(ctx)
	)

	defer workersCancel()

	// start workers
	for wID := range workers {
		go worker(workerCtx, wID, jobs, results, errs, do)
	}

	// feed workers
	// Can do without goroutine but typically slower time to first result.
	feed := func(ctx context.Context, inputs int, jobs chan<- int) {
		defer close(jobs) // no more work to do, signals workers to return.

		for id := range inputs {
			select {
			case <-ctx.Done():
				return // and close(jobs) signaling workers.
			case jobs <- id:
			}
		}
	}

	go feed(ctx, inputs, jobs)

	// aggregate responses into output
	for i := range inputs {
		select {
		case <-ctx.Done():
			// context cancelled upstream, workers will exit at receive or send
			// channel selects.
			return output, ctx.Err() // output, fmt.Errorf(...)

		case wErr := <-errs:
			workersCancel() // notify other workers to stop. Assuming any error means stop all work.

			return output, fmt.Errorf("processed up to result: %d, %v", i, wErr)

		case result := <-results:
			output[result.input] = result.output
		}
	}

	return output, nil
}

func worker(
	ctx context.Context,
	_ int, // id
	jobs <-chan int,
	results chan<- result,
	errs chan<- error,
	do func(context.Context, int) (int, error),
) {
	var received, completed, sent int

	for jobs != nil && results != nil && errs != nil {
		select {
		case <-ctx.Done():
			jobs = nil
			break // return if nothing to do at end of for loop.

		case input, ok := <-jobs:
			if !ok {
				// jobs channel closed, no more work to do.
				jobs = nil
				break // return if nothing to do at end of for loop.
			}

			received++

			output, err := do(ctx, input)
			if err != nil {
				select {
				case <-ctx.Done():
				case errs <- fmt.Errorf("input: %d, err: %v", input, err):
				}

				jobs = nil
				break // return if nothing to do at end of for loop.
			}

			completed++

			select {
			case <-ctx.Done():
				jobs = nil
				break // return if nothing to do at end of for loop.
			case results <- result{
				input:  input,
				output: output,
			}:
				sent++
			}
		}
	}

	// any final actions, in real life use dependency injection for logger/clients/etc.
	// type worker struct {
	//   logger slog.Logger
	// }
	// log.Printf("worker: %d received: %d completed: %d sent: %d", id, received, completed, sent)
}

// task represents some work that needs to be done by each worker. It honors context cancellation,
// returning an error if canceled.
// This function is substituted in tests to force certain code paths in this example code.
// In practice may use an Interface at domain boundary and a fake implementation or mock in tests.
func task(ctx context.Context, input int) (int, error) {
	select {
	case <-ctx.Done():
		return input, ctx.Err()
	case <-time.After(100 * time.Millisecond): // simulate RPC/IO/compute latency
		return input * 5, nil
	}
}
