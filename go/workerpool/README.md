# worker pool

There are multiple reasons why concurrency can be helpful and often just as many why it can be a
sub-optimal solution.

My checklist is:

- doing no work is faster than doing some work. Can the task be avoided?
- is the bottleneck here or elsewhere? "local optima" versus system constraint
- benefit(s) outweigh the costs? latency improvement vs concurrency complexity, benchmark!

After going through the checklist, I found myself in the situation where it was still advantageous
to use concurrency. The solution required fanning out, performing multiple requests within limits.

This resulted in code shaped like the example here.

# Getting Started

Run:

```sh
go run main.go

# output:
output: map[0:0 1:5 2:10 3:15 4:20 5:25 6:30 7:35 8:40 9:45 10:50 11:55 12:60 13:65 14:70 15:75 16:80 17:85 18:90 19:95 20:100 21:105 22:110 23:115 24:120 25:125 26:130 27:135 28:140 29:145 30:150 31:155 32:160 33:165 34:170 35:175 36:180 37:185 38:190 39:195 40:200 41:205 42:210 43:215 44:220 45:225 46:230 47:235 48:240 49:245 50:250 51:255 52:260 53:265 54:270 55:275 56:280 57:285 58:290 59:295 60:300 61:305 62:310 63:315 64:320 65:325 66:330 67:335 68:340 69:345 70:350 71:355 72:360 73:365 74:370 75:375 76:380 77:385 78:390 79:395 80:400 81:405 82:410 83:415 84:420 85:425 86:430 87:435 88:440 89:445 90:450 91:455 92:460 93:465 94:470 95:475 96:480 97:485 98:490 99:495]
```

Test:

```sh
go test ./... -v

# output
=== RUN   TestManager_NoError
--- PASS: TestManager_NoError (1.00s)
=== RUN   TestManager_DoError
--- PASS: TestManager_DoError (0.00s)
=== RUN   TestManager_ContextCanceled
    main_test.go:127: manager: `select { case <-ctx.Done(): }`
--- PASS: TestManager_ContextCanceled (0.00s)
=== RUN   ExampleMain
--- PASS: ExampleMain (1.00s)
PASS
ok  	example	2.010s
```

Benchmark:

```sh
go test ./... -run=^$ -bench=. -memprofile mem.out -cpuprofile cpu.out -count=5

# output
go test ./... -run=^$ -bench=. -memprofile mem.out -cpuprofile cpu.out -count=5
goos: linux
goarch: arm64
pkg: example
BenchmarkManager-8   	      1	1004306757 ns/op	  41688 B/op	    408 allocs/op
BenchmarkManager-8   	      1	1003950811 ns/op	  49880 B/op	    376 allocs/op
BenchmarkManager-8   	      1	1003552409 ns/op	  34280 B/op	    374 allocs/op
BenchmarkManager-8   	      1	1005868618 ns/op	  33544 B/op	    365 allocs/op
BenchmarkManager-8   	      1	1007290192 ns/op	  27960 B/op	    347 allocs/op
PASS
ok  	example	5.048s


# view profiles
go tool pprof -http localhost:6060 mem.out
go tool pprof -http localhost:6060 cpu.out
```

## Flow

"Happy path" without errors:

1. `manager` starts `n` `workers`
2. `workers` listen indefinitely on `jobs` channel
3. `manager` feeds `workers` by sending `input` to be processed on `jobs` channel
4. `workers` receive on `jobs` channel and process task
5. `workers` send each job `result` back to manager on `results` channel
6. `manager` receives on `jobs` channel and aggregates `results` into a map
7. `manager` signals no more work to do by closing `jobs` channel
8. `workers` detect `jobs` channel closed and break infinite loop, then terminate
9. `manager` logs aggregated results (does something with results)

## Optimizations and Caveats

#### long lived workers

Workers process many jobs.

This reduces goroutine scheduling work for the runtime compared to alternative
approaches like spinning up a goroutine per job.

#### `nil` channels

Sending or Receiving on a `nil` channel blocks indefinitely.

Checking `jobs != nil` is a great way to stop the indefinite `for { select { case <-jobs: } }` loop
and then complete any final operations.

In the example code there are no final operations, but these could be something like aggregating
results, cleaning up, or logging.

The 100 Go Mistakes book by Teiva Harsanyi has a great section: [Not using nil channels #66](https://100go.co/#not-using-nil-channels-66).

#### channel capacities

Channels may be unbuffered or buffered with different capacities, e.g: capacity of 0, 1,
number of workers, number of inputs, other.

Some options:

- throttle with `capacity = no. workers`. The number of workers is already a throttle, limiting
  concurrency so a specific channel capacity isn't required for throttling.
- synchronization requirement.
  In the example code there's no need for producer and consumer to sync. Both jobs and results channels
  could build up at different rates. It may be fastest to have both channels be `capacity = no. inputs`.
- memory usage.
  If each job input is large, or the result is large, it may be necessary to reduce the channel capacity to
  minimize memory in use, perhaps to `capacity = no. workers` or `1`.
- queues typically operate near empty or near capacity. If near capacity then a larger queue (channel)
  size may only make a longer line! or in our case, more memory usage without any benefit, suggesting
  an optimal size of `capacity = 1`.

Then, for performance we may want to consider alternative approaches:

- instead of a `jobs <-chan`, pass each worker an array slice, e.g: `inputs[i:i+10]`, sharing memory.
- instead of a `results chan<-`, allocate a results array of full capacity and have each worker insert
  non-overlapping positions in the array.
- batch results and send batch over channel before returning, with the manager to aggregate batches.

Best to create a benchmark test with `*testing.B` and capture profiles to understand where time and
allocations are spent.

The book [Efficient Go](https://www.oreilly.com/library/view/efficient-go/9781098105709/) by
Bartlomiej Plotka is a great read.

#### context.Context

In the example there are two main uses of `context.Context`:

1. If the whole operation is to stop (e.g: `ctrl+c`), then we want to stop all work,
   limiting compute/IO/etc and return. All memory including the goroutines can be freed during GC.
2. If there is an error, we want to stop all work and return the error (and partial results).
   The desired semantics could be different, for example a slice of errors could be returned.

The key method for achieving both is via `select`:

```go
select {
  case <-ctx.Done():
  case wErr := <-errs:
  case result := <-results:
}
```

`select` performs pseudo random selection and in practice when a context is canceled, the
`case <-ctx.Done():` will be selected quickly.

Secondly, sending to a full or unbuffered channel will block until the consumer receives.
So if we do receive an error `case wErr := <-errs:`, then before we return we need to cancel
the workers `context.Context` via `workersCancel()` otherwise they will hang indefinitely trying
to send on the `results` or `errs` channel.

Cancellation could also be done with the use of a "notification channel" and selecting either send
result or receiving "stop" notification.

## Useful links

- [100 Go Mistakes](https://100go.co/book/) by [Teiva Harsanyi](https://teivah.dev/)
- [Efficient Go](https://www.oreilly.com/library/view/efficient-go/9781098105709/) by
[Bartlomiej Plotka](https://www.bwplotka.dev/)
- [Effective Go - channels](https://go.dev/doc/effective_go#channels)
