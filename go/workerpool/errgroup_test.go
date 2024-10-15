package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestManagerEG_NoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)
	defer cancel()

	doFunc := task

	const workers = 10
	const inputs = 100

	got, err := managerEG(ctx, inputs, workers, doFunc)
	if err != nil {
		t.Fatalf("managerEG() %v", err)
	}

	if len(got) != inputs {
		t.Errorf("want: %d got: %d", inputs, len(got))
	}
}

func TestManagerEG_DoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)

	defer cancel()

	wantErr := "introduced error on input: 5"
	doFunc := func(ctx context.Context, input int) (int, error) {
		if input == 5 {
			return 0, fmt.Errorf("introduced error on input: %d", input)
		}

		return input * 5, nil
	}

	const workers = 10
	const inputs = 100

	_, err := managerEG(ctx, inputs, workers, doFunc)
	if err == nil {
		t.Fatal("expected error but was none")
	}

	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("want: '%s' got: %s", wantErr, err.Error())
	}
}

func TestManagerEG_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)

	defer cancel()

	doFunc := func(ctx context.Context, input int) (int, error) {
		if input == 5 {
			t.Logf("cancelling")
			// force cancellation mid processing.
			cancel()

			// some workers may have just sent result/balance. select ctx.Done() at top and return.
			// some workers may be sending on channel now. select ctx.Done() at top and return.
			// some workers will be blocked sending, so select ctx.Done() at bottom and return.
			// manager might have received some results on channel.
			// manager will select ctx.Done() and return.
			return -1, context.Canceled
		}

		return input * 5, nil
	}

	const workers = 10
	const inputs = 100

	_, err := managerEG(ctx, inputs, workers, doFunc)

	processedErrorFound, regexpErr := regexp.Match(
		"^processed up to result: [0-9]+, input: [0-9]+, err: context canceled$",
		[]byte(err.Error()),
	)

	switch {
	case processedErrorFound:
		t.Log("manager: `select { case wErr := <-errs: }`") // more common with larger channels
	case errors.Is(err, context.Canceled):
		t.Log("manager: `select { case <-ctx.Done(): }`")
	case regexpErr != nil:
		t.Errorf("unexpected regexp error: %v", regexpErr)
	default:
		t.Errorf("unexpected error: %v", err)
	}

	/* Quick frequency check for expected cases, results depend on channel capacities, below with 200:
	$ go test ./... -run=TestManager_ContextCancelled -count=100 -v | grep ctx.Done | wc -l
	86

	$ go test ./... -run=TestManager_ContextCancelled -count=100 -v | grep wErr | wc -l
		23
	*/
}

func BenchmarkManagerEG(b *testing.B) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		60*time.Second, // minimise chance of test hanging.
	)
	defer cancel()

	doFunc := task

	const workers = 10
	const inputs = 100

	b.ReportAllocs() // equivalent to -benchmem
	b.ResetTimer()   // minimal setup here but good if required.

	for range b.N {
		_, err := managerEG(ctx, inputs, workers, doFunc)
		if err != nil {
			fmt.Println("error during benchmark:", err)
			break
		}
	}
}
