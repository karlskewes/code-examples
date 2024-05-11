# runtime.GC()

> `runtime.GC()` runs a garbage collection and blocks the caller until the garbage collection is
> complete. It may also block the entire program.

The latency introduced by `runtime.GC()` may or may not be acceptable.
Calling `runtime.GC()` may be unnecessary if the function generating garbage can be invoked
multiple times before significant memory pressure occurs.

# GOMEMLIMIT

`GOMEMLIMIT=256MiB` sets a `256MB` target soft limit on heap memory usage, influencing when garbage
collection will run.
This may result in less frequent garbage collections than calling `runtime.GC()` after every
function invocation.

However, if there is a mismatch between automatic garbage collection runs and "heap-creating
function" invocations then `GOMEMLIMIT` may still be exceeded.
The same can occur with `runtime.GC()`, memory usage can spike up and then be reduced.

Consider the following strategy:
1. Confirm there is a real problem worth the time and expense to investigate.
1. Profile with `parca` or similar.
1. Reduce garbage creation, perhaps try allocate on the stack, re-use memory.
1. Set `GOMEMLIMIT`.
1. Finally consider `runtime.GC()` though unlikely.

# Getting Started

Run:
```sh
go run main.go
```

Execute `scrypt.Key()` operation in another terminal:
```sh
curl localhost:8080/

11:16:55 executing scrypt.Key() count: 1
```

Start [parca.dev](https://parca.dev) continuous profiling in another terminal:
```sh
parca
```

## Logging garbage collection statistics

Without `runtime.GC()` forced, note:
- heap size after GC is still 130MB.
```sh
GODEBUG=gctrace=1 go run main.go

2024/05/13 08:17:21 listening on: http://localhost:8080/
2024/05/13 08:17:21 executeScrypt() requester: startup
gc 1 @0.001s 0%: 0.022+17+0.021 ms clock, 0.18+0.13/0.16/0.065+0.17 ms cpu, 128->128->128 MB, 128 MB goal, 0 MB stacks, 0 MB globals, 8 P
```

With `runtime.GC()` forced, note:
- extra GC event `2`
- GC event `2` was `(forced)`
- heap size after GC reduces to `2MB`.
```sh
GODEBUG=gctrace=1 go run main.go -force

2024/05/13 08:17:51 listening on: http://localhost:8080/
2024/05/13 08:17:51 executeScrypt() requester: startup
gc 1 @0.000s 2%: 0.015+2.1+0.016 ms clock, 0.12+0.050/0.25/0.024+0.13 ms cpu, 128->129->128 MB, 128 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 2 @0.224s 0%: 0.010+0.20+0.008 ms clock, 0.080+0/0.14/0.062+0.067 ms cpu, 129->129->0 MB, 256 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
```

## Simulating load without forced garbage collection

Execute `scrypt.Key()` multiple times with the `-count N` flag and observe heap memory accumulates.

Run:
```sh
$ GODEBUG=gctrace=1 go run main.go -count 5
```

Execute `scrypt.Key()` operation 5 times in another terminal:
```sh
curl localhost:8080/

11:16:55 executing scrypt.Key() count: 5
```

NOTE: heap memory sitting at `512 MB` after operations complete.
```sh
2024/05/18 07:09:40 listening on: http://localhost:8080/
2024/05/18 07:09:40 executeScrypt() requester: startup
gc 1 @0.001s 5%: 0.017+0.27+0.035 ms clock, 0.13+0.21/0.33/0.021+0.28 ms cpu, 128->129->128 MB, 128 MB goal, 0 MB stacks, 0 MB globals, 8 P
2024/05/18 07:09:45 executeScrypt() requester: http at 07:09:45
2024/05/18 07:09:45 executeScrypt() requester: http at 07:09:45
2024/05/18 07:09:45 executeScrypt() requester: http at 07:09:45
2024/05/18 07:09:45 executeScrypt() requester: http at 07:09:45
2024/05/18 07:09:45 executeScrypt() requester: http at 07:09:45
gc 2 @4.955s 0%: 0.093+3.6+0.029 ms clock, 0.74+0.11/0.31/0+0.23 ms cpu, 256->384->256 MB, 384 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 3 @4.963s 0%: 1.1+7.8+0.048 ms clock, 9.0+0.14/0.27/0+0.38 ms cpu, 512->512->512 MB, 512 MB goal, 0 MB stacks, 0 MB globals, 8 P
```

Roughly 2 minutes later, garbage collection runs freeing heap memory:
```sh
GC forced
gc 13 @121.046s 0%: 0.073+2.0+0.005 ms clock, 0.58+0/3.7/0+0.046 ms cpu, 5->5->2 MB, 7 MB goal, 0 MB stacks, 0 MB globals, 8 P
GC forced
gc 3 @121.549s 0%: 0.028+0.16+0.002 ms clock, 0.22+0/0.17/0+0.016 ms cpu, 512->512->0 MB, 1024 MB goal, 0 MB stacks, 0 MB globals, 8 P
```

## Simulating load with forced garbage collection

Run:
```sh
$ GODEBUG=gctrace=1 go run main.go -count 5 -force
```

Execute:
```sh
$ curl localhost:8080
07:18:07 executing scrypt.Key() count: 5
```

Multiple GC events, heap memory spikes then returns to 0 MB:
```sh
2024/05/18 07:18:07 executeScrypt() requester: http at 07:18:07
2024/05/18 07:18:07 executeScrypt() requester: http at 07:18:07
2024/05/18 07:18:07 executeScrypt() requester: http at 07:18:07
2024/05/18 07:18:07 executeScrypt() requester: http at 07:18:07
2024/05/18 07:18:07 executeScrypt() requester: http at 07:18:07
gc 3 @20.037s 0%: 0.12+0.26+0.048 ms clock, 0.96+0.26/0.27/0+0.38 ms cpu, 128->256->256 MB, 256 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 4 @20.038s 0%: 0.071+3.1+3.9 ms clock, 0.57+0.10/0.30/0+31 ms cpu, 512->640->640 MB, 640 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 5 @20.288s 0%: 0.077+0.29+0.064 ms clock, 0.61+0/0.18/0+0.51 ms cpu, 640->640->512 MB, 1280 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
gc 6 @20.289s 0%: 0.030+0.31+0.18 ms clock, 0.24+0/0.14/0+1.4 ms cpu, 512->512->384 MB, 1024 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
gc 7 @20.335s 0%: 0.095+0.24+0.030 ms clock, 0.76+0/0.28/0.082+0.24 ms cpu, 384->384->256 MB, 768 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
gc 8 @20.382s 0%: 0.069+0.28+0.008 ms clock, 0.55+0/0.16/0.007+0.067 ms cpu, 256->256->128 MB, 512 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
gc 9 @20.416s 0%: 0.018+0.18+0.008 ms clock, 0.14+0/0.15/0.011+0.066 ms cpu, 128->128->0 MB, 256 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)
```

## Simulating load without forced garbage collection but a low GOMEMLIMIT

Execute `scrypt.Key()` multiple times with the `-count N` flag and observe heap memory accumulates.

Run:
```sh
$ GOMEMLIMIT=256MiB GODEBUG=gctrace=1 go run main.go -count 5
```

Execute `scrypt.Key()` operation 5 times in another terminal:
```sh
curl localhost:8080/

11:16:55 executing scrypt.Key() count: 5
```

Multiple GC events, heap memory spikes then reduces to below the soft limit:
```sh
2024/09/16 19:02:46 listening on: http://localhost:8080/
2024/09/16 19:02:46 executeScrypt() requester: startup
gc 1 @0.001s 23%: 0.43+0.43+0.033 ms clock, 3.4+0.066/0.31/0.16+0.26 ms cpu, 131->131->130 MB, 131 MB goal, 0 MB stacks, 0 MB globals, 8 P
2024/09/16 19:02:50 executeScrypt() requester: http at 19:02:50
2024/09/16 19:02:50 executeScrypt() requester: http at 19:02:50
2024/09/16 19:02:50 executeScrypt() requester: http at 19:02:50
2024/09/16 19:02:50 executeScrypt() requester: http at 19:02:50
gc 2 @3.920s 0%: 0.62+0.69+0.013 ms clock, 4.9+0.22/0.66/0+0.11 ms cpu, 259->387->258 MB, 130 MB goal, 0 MB stacks, 0 MB globals, 8 P
2024/09/16 19:02:50 executeScrypt() requester: http at 19:02:50
gc 3 @3.924s 0%: 0.039+1.2+0.13 ms clock, 0.31+0.20/0.76/0+1.0 ms cpu, 258->258->258 MB, 258 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 4 @3.926s 0%: 0.19+4.6+0.074 ms clock, 1.5+0.19/1.9/0+0.59 ms cpu, 258->642->642 MB, 258 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 5 @3.931s 0%: 0.21+6.4+0.31 ms clock, 1.6+0.13/1.1/0+2.4 ms cpu, 642->642->642 MB, 642 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 6 @3.940s 0%: 0.040+1.8+0.53 ms clock, 0.32+0.10/0.45/0.059+4.3 ms cpu, 642->642->642 MB, 642 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 7 @3.942s 0%: 0.25+2.1+0.037 ms clock, 2.0+0.084/0.49/0+0.29 ms cpu, 642->642->642 MB, 642 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 8 @4.005s 0%: 0.070+3.7+0.17 ms clock, 0.56+0.053/0.42/0+1.4 ms cpu, 642->642->642 MB, 642 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 9 @4.195s 0%: 0.072+0.43+0.068 ms clock, 0.57+0.029/0.31/0+0.54 ms cpu, 642->642->514 MB, 642 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 10 @4.203s 0%: 0.048+1.8+0.046 ms clock, 0.38+1.4/3.3/2.6+0.37 ms cpu, 514->514->386 MB, 514 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 11 @4.208s 0%: 0.036+0.91+0.018 ms clock, 0.29+0.75/1.5/1.8+0.15 ms cpu, 386->386->258 MB, 386 MB goal, 0 MB stacks, 0 MB globals, 8 P
gc 12 @4.277s 0%: 0.074+0.96+0.008 ms clock, 0.59+0.77/1.6/2.9+0.064 ms cpu, 258->258->130 MB, 258 MB goal, 0 MB stacks, 0 MB globals, 8 P
GC forced
gc 14 @121.047s 0%: 0.047+2.7+0.067 ms clock, 0.37+0/5.3/0+0.54 ms cpu, 4->4->2 MB, 6 MB goal, 0 MB stacks, 0 MB globals, 8 P
GC forced
gc 13 @124.352s 0%: 0.059+1.4+0.016 ms clock, 0.47+0/1.9/0+0.13 ms cpu, 131->131->2 MB, 239 MB goal, 0 MB stacks, 0 MB globals, 8 P
```

Interestingly `gc 14` was logged before `gc 13`.

## Useful links

- `GODEBUG=gctrace=1`: https://pkg.go.dev/runtime#hdr-Environment_Variables
- https://tip.golang.org/doc/gc-guide
- https://github.com/golang/go/issues/40687
- https://github.com/golang/go/issues/20000
- https://github.com/golang/go/issues/7168
- https://groups.google.com/g/golang-nuts/c/I9R9MKUS9bo
