# Go Profiling Runbook

## Overview

The keep-core binary exposes Go runtime profiling endpoints via the
`clientinfo` HTTP server. The `/debug/pprof/...` endpoints are gated by
`EnablePprof` in configuration: they are only registered when
`EnablePprof: true` is set explicitly under `[ClientInfo]`, and are
not served otherwise. Profiles are served at `/debug/pprof/` on the
same port as metrics and diagnostics (`ClientInfo.Port`).

## Security Warning

The clientinfo HTTP server binds to all interfaces (`0.0.0.0`). The
`EnablePprof` flag is the only thing that prevents the `/debug/pprof/`
endpoints from being reachable on that port: **leave `EnablePprof`
unset or `false` on any node whose `ClientInfo.Port` is reachable
beyond a trusted network.** Leaving the flag at its default (off) is
the secure posture; flipping it on a node exposed to untrusted
networks exposes CPU profiles, heap dumps, and goroutine traces that
can leak sensitive runtime state.

Safe access patterns when profiling is genuinely needed:
- Run on a private/firewalled network
- Use an SSH tunnel: `ssh -L 9601:localhost:9601 node-host`
- Restrict at the network layer (security group, firewall rule)
- Profile briefly (e.g. `-seconds=` on the CPU profile endpoint) and
  set `EnablePprof: false` again when finished

## Enabling Profiling

Profiling is disabled by default. To enable it, set `EnablePprof: true`
in your config (TOML example):

```toml
[ClientInfo]
  Port = 9601
  EnablePprof = true
```

Or pass via environment / flag if your deployment uses those overrides.
Set `EnablePprof` back to `false` (or remove it) as soon as you are
finished profiling so the endpoints stop being served.

## Standard Commands

Replace `9601` with your configured `ClientInfo.Port`.

### CPU profile (30 seconds)

```sh
go tool pprof http://localhost:9601/debug/pprof/profile?seconds=30
```

### Heap profile

```sh
go tool pprof http://localhost:9601/debug/pprof/heap
```

### Goroutine dump (text)

```sh
curl -s http://localhost:9601/debug/pprof/goroutine?debug=2
```

### Trace (5 seconds)

```sh
curl -o /tmp/trace.out http://localhost:9601/debug/pprof/trace?seconds=5
go tool trace /tmp/trace.out
```

### Mutex contention

```sh
# Enable mutex profiling first (runtime call or startup flag):
#   runtime.SetMutexProfileFraction(1)
go tool pprof http://localhost:9601/debug/pprof/mutex
```

## Benchmark + Profile Workflow

To identify hot paths found by benchmarks:

Note: `-bench=` accepts a Go regular expression that substring-matches
benchmark names; the patterns below intentionally match every
size-suffixed variant of the named benchmark (e.g.
`BenchmarkGetRecentWindows_100Windows`, `BenchmarkComputeSignatureHashes_5Inputs`).

```sh
# Run benchmark and write CPU profile
go test ./pkg/tbtc/... -run=^$ -bench=BenchmarkGetRecentWindows \
  -cpuprofile=/tmp/cpu.pprof -benchtime=5s

# Inspect interactively
go tool pprof /tmp/cpu.pprof
(pprof) top10
(pprof) web   # requires graphviz
```

For memory allocation hot paths:

```sh
go test ./pkg/bitcoin/... -run=^$ -bench=BenchmarkComputeSignatureHashes \
  -memprofile=/tmp/mem.pprof -benchtime=5s
go tool pprof /tmp/mem.pprof
(pprof) alloc_space
(pprof) top10
```

## Comparing Benchmarks Across Commits

```sh
# Baseline (main branch)
git stash
go test ./pkg/... -run=^$ -bench=. -count=6 | tee /tmp/baseline.txt

# Candidate (your branch)
git stash pop
go test ./pkg/... -run=^$ -bench=. -count=6 | tee /tmp/candidate.txt

benchstat /tmp/baseline.txt /tmp/candidate.txt
```

Install `benchstat`: `go install golang.org/x/perf/cmd/benchstat@latest`

## Available Endpoints

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/` | Index of available profiles |
| `/debug/pprof/cmdline` | Process command line |
| `/debug/pprof/profile` | CPU profile (30s default) |
| `/debug/pprof/symbol` | Symbol lookup |
| `/debug/pprof/trace` | Execution trace |
| `/debug/pprof/goroutine` | Goroutine stacks |
| `/debug/pprof/heap` | Heap allocations |
| `/debug/pprof/allocs` | Allocation samples |
| `/debug/pprof/block` | Goroutine blocking events |
| `/debug/pprof/mutex` | Mutex contention |

## Notes

- CPU profiling adds ~5% overhead to the profiled binary during the sampling
  window. It is safe to run against a live node for short durations.
- Heap and goroutine profiles are sampled snapshots; a single sample may
  miss transient allocations. Take multiple profiles under load.

