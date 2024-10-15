package main

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// managerEG is an alternative implementation similar to https://pkg.go.dev/golang.org/x/sync/errgroup#example-Group-Pipeline.
func managerEG(
	ctx context.Context,
	inputs int,
	workers int,
	do func(context.Context, int) (int, error),
) (map[int]int, error) {
	var (
		output                   = make(map[int]int)          // map[<input>]<result>
		jobs                     = make(chan int, 1)          // Consider len(<inputs>)
		results                  = make(chan result, workers) // GC to clean up.
		workerCtx, workersCancel = context.WithCancel(ctx)
	)

	defer workersCancel()

	g, ctx := errgroup.WithContext(context.Background()) // enable cancellation on error

	// Feeder
	// Can do without goroutine but typically slower time to first result.
	g.Go(func() error {
		defer close(jobs) // no more work to do, signals workers to return.

		for id := range inputs {
			select {
			case <-ctx.Done():
				return nil // and close(jobs) signaling workers.
			case jobs <- id:
			}
		}
		return nil
	})

	// Workers
	for wID := range workers {
		g.Go(func() error {
			return workerEG(workerCtx, wID, jobs, results, do)
		})
	}

	// Waiter - waits for completion of feed + worker
	go func() {
		_ = g.Wait()   // disregard error, will check it in main goroutine.
		close(results) // signal main aggregator all results in
	}()

	// aggregate responses into output
	for result := range results {
		output[result.input] = result.output
	}

	err := g.Wait()
	if err != nil {
		return output, fmt.Errorf("processed up to result: %d, %v", len(output), err)
	}

	return output, nil
}

// workerEG is an alternative implementation similar to https://pkg.go.dev/golang.org/x/sync/errgroup#example-Group-Pipeline.
func workerEG(
	ctx context.Context,
	_ int, // id
	jobs <-chan int,
	results chan<- result,
	do func(context.Context, int) (int, error),
) error {
	var received, completed, sent int
	var err error

OuterLoop:
	for input := range jobs {
		var output int
		received++

		output, err = do(ctx, input)
		if err != nil {
			err = fmt.Errorf("input: %d, err: %v", input, err)

			break OuterLoop // return err if nothing to do at end of for loop.
		}

		completed++

		select {
		case <-ctx.Done():
			err = ctx.Err()
			break OuterLoop // return ctx.Err() if nothing to do at end of for loop.
		case results <- result{
			input:  input,
			output: output,
		}:
			sent++
		}
	}

	// any final actions, in real life use dependency injection for logger/clients/etc.
	// type worker struct {
	//   logger slog.Logger
	// }
	// log.Printf("worker: %d received: %d completed: %d sent: %d", id, received, completed, sent)

	return err
}
