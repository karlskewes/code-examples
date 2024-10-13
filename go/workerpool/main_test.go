package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func ExampleMain() {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)
	defer cancel()

	doFunc := task

	const workers = 10
	const inputs = 100

	output, err := manager(ctx, inputs, workers, doFunc)
	if err != nil {
		fmt.Printf("manager() %v", err)
		os.Exit(1)
	}

	fmt.Printf("output: %v\n", output)
	// Output:
	// output: map[0:0 1:5 2:10 3:15 4:20 5:25 6:30 7:35 8:40 9:45 10:50 11:55 12:60 13:65 14:70 15:75 16:80 17:85 18:90 19:95 20:100 21:105 22:110 23:115 24:120 25:125 26:130 27:135 28:140 29:145 30:150 31:155 32:160 33:165 34:170 35:175 36:180 37:185 38:190 39:195 40:200 41:205 42:210 43:215 44:220 45:225 46:230 47:235 48:240 49:245 50:250 51:255 52:260 53:265 54:270 55:275 56:280 57:285 58:290 59:295 60:300 61:305 62:310 63:315 64:320 65:325 66:330 67:335 68:340 69:345 70:350 71:355 72:360 73:365 74:370 75:375 76:380 77:385 78:390 79:395 80:400 81:405 82:410 83:415 84:420 85:425 86:430 87:435 88:440 89:445 90:450 91:455 92:460 93:465 94:470 95:475 96:480 97:485 98:490 99:495]
}

func TestManager_NoError(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)
	defer cancel()

	doFunc := task

	const workers = 10
	const inputs = 100

	got, err := manager(ctx, inputs, workers, doFunc)
	if err != nil {
		t.Fatalf("manager() %v", err)
	}

	if len(got) != inputs {
		t.Errorf("want: %d got: %d", inputs, len(got))
	}
}

func TestManager_DoError(t *testing.T) {
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

	_, err := manager(ctx, inputs, workers, doFunc)
	if err == nil {
		t.Fatal("expected error but was none")
	}

	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("want: '%s' got: %s", wantErr, err.Error())
	}
}

func TestManager_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second, // minimise chance of test hanging.
	)

	defer cancel()

	doFunc := func(ctx context.Context, input int) (int, error) {
		if input == 5 {
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

	_, err := manager(ctx, inputs, workers, doFunc)

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

func BenchmarkManager(b *testing.B) {
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
		_, err := manager(ctx, inputs, workers, doFunc)
		if err != nil {
			fmt.Println("error during benchmark:", err)
			break
		}
	}
}
