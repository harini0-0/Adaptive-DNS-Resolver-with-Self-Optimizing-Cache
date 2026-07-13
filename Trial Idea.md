# Caching DNS Resolver in Go

A concurrent DNS resolver with a TTL aware cache, Prometheus instrumentation, and a containerized observability stack. Built in Go, deployed with Docker Compose, monitored with Prometheus and Grafana.

## What this document is

This is the engineering specification and build plan for the project. It is written to double as the repository's own documentation, so it lives in the repo root as `PROJECT_SPEC.md` and the README links to it. Each stage below is a working checkpoint: the project is demonstrable and defensible at the end of every stage, so if the timeline compresses, you stop at the last completed stage with a real, finished thing rather than a half built one.

## Engineering framing

Most student projects are a single commit of "it works on my laptop." This one is scoped to look like production infrastructure at small scale. The point is not that DNS resolution is hard. The point is to demonstrate three things an infrastructure or backend reviewer actually looks for: correct concurrency under real load, a caching design with tradeoffs you chose deliberately and can defend, and operational maturity in the form of metrics, containerization, and a running monitoring stack. A resolver is a good vehicle because it is naturally a concurrent request/response system with a built in reason to cache (records carry their own TTL), so nothing in the design is bolted on for show.

The measurable outcome you are building toward is a single defensible claim: a concurrent DNS resolver serving thousands of queries per second, with a TTL aware LRU cache raising hit rate and cutting tail latency, instrumented end to end and deployed as a monitored container stack. Every stage exists to make one part of that sentence true and provable.

## Non goals

Stating these upfront keeps scope honest and prevents overclaiming in interviews.

- This is not a low latency systems project. Go's runtime manages memory and scheduling, so this does not demonstrate manual memory ordering, cache line control, or lock free programming. It is aimed at backend, infrastructure, and general software engineering roles, not high frequency trading seats.
- This is not an authoritative DNS server. It is a recursive/forwarding resolver with a cache. It does not host zones or answer as the source of truth for a domain.
- This is not a Kubernetes project. Single node Docker Compose is the deployment target. Kubernetes would be over scoping for a solo build and easy to overclaim.
- Full DNSSEC validation is out of scope. It can be named as a future extension, not claimed as built.

## Architecture overview

The system is a UDP listener feeding a bounded worker pool, each worker resolving through a shared concurrency safe cache with an upstream forwarder behind it, with a metrics endpoint scraped by Prometheus and visualized in Grafana.

```
                        +-------------------+
   DNS client  ---UDP-->|  UDP listener     |
                        |  (receive loop)   |
                        +---------+---------+
                                  |  enqueue query
                                  v
                        +-------------------+
                        |  worker pool      |   bounded number of goroutines
                        |  (goroutines)     |
                        +---------+---------+
                                  |  lookup
                                  v
                        +-------------------+     miss     +------------------+
                        |  TTL aware LRU    |------------->|  upstream        |
                        |  cache (RWMutex)  |<-------------|  forwarder (UDP) |
                        +---------+---------+   fill+TTL   +------------------+
                                  |  hit or filled
                                  v
                        +-------------------+
                        |  response writer  |---UDP--> DNS client
                        +-------------------+

   Every component increments Prometheus metrics.
   /metrics endpoint <---scrape--- Prometheus <---query--- Grafana dashboards
```

## Technology choices and justification

Each tool is listed with the specific reason it is in the stack, because "why did you use X" is a standard interview follow up and the answer should never be "it was in a tutorial."

| Layer | Tool | Why this one |
|---|---|---|
| Language | Go | First class concurrency primitives (goroutines, channels), a strong standard library for UDP and HTTP, and static binaries that containerize cleanly. Matches backend and infrastructure role expectations. |
| DNS protocol | `miekg/dns` (optional) or hand rolled parser | Hand rolling the wire format in the early stage teaches the protocol and is more defensible in interviews. `miekg/dns` is the industry standard library and is a reasonable choice once you have shown you understand the format. Pick one and be able to justify it. |
| Concurrency | Go stdlib (`sync`, `context`, channels) | No external dependency needed. The worker pool, cancellation, and cache locking all use standard primitives, which is what a reviewer wants to see you reason about directly. |
| Metrics | Prometheus Go client (`prometheus/client_golang`) | The de facto standard for service metrics. Pull based scraping model, native histogram support for latency percentiles. |
| Dashboards | Grafana | Standard visualization layer on top of Prometheus. Turns metrics into a screenshotable artifact for a portfolio. |
| Packaging | Docker (multi stage build) | Multi stage build ships a tiny final image from a scratch or distroless base, which is a real best practice you can explain. |
| Orchestration | Docker Compose | Runs resolver, Prometheus, and Grafana together as one local stack. Demonstrates multi service orchestration without the overhead of Kubernetes. |
| Load testing | `dnsperf` or a custom Go load client | Generates the query volume needed to produce real throughput and latency numbers rather than guessed ones. |

## Build stages

Five stages, each a working checkpoint with its own deliverable, tools, and acceptance criteria. Estimated total effort is three to five weeks part time. The stages are ordered so the highest signal core (a correct concurrent resolver with a real cache) is done by Stage 2, and everything after adds operational polish.

---

### Stage 0: Foundations and scaffolding

**Goal.** A Go module that compiles, a project layout, and a git history that looks intentional.

**Work.**
- Initialize the Go module and a clean package layout: a `cmd/resolver` entrypoint, an internal package for the DNS logic, an internal package for the cache, and an internal package for metrics.
- Set up a Makefile or task runner with `build`, `run`, `test`, and `lint` targets.
- Configure `golangci-lint` and commit the config.
- Write a README stub that links to this spec.

**Tools.** Go toolchain, `golangci-lint`, git.

**Deliverable.** `go build ./...` succeeds, `go vet` and the linter pass, repository has a clear structure.

**Acceptance criteria.** A reviewer cloning the repo can build and run a placeholder binary in one command.

---

### Stage 1: UDP resolver that forwards and answers

**Goal.** A working DNS resolver: it receives a query over UDP, forwards it to an upstream resolver, and returns the answer to the client. No cache yet.

**Work.**
- Open a UDP socket and run a receive loop (`net.ListenUDP`, `ReadFromUDP`).
- Parse the incoming DNS query: header (ID, flags, counts) and the question section (name, type, class). If hand rolling, implement name parsing including compression pointer handling. If using `miekg/dns`, use its unpack and be ready to explain the wire format anyway.
- Forward the query to a configurable upstream resolver (for example 1.1.1.1 or 8.8.8.8) over UDP and read the response.
- Write the response back to the original client address.
- Handle the basics: request timeouts with `context`, malformed packet rejection, and the fact that a UDP response over 512 bytes signals truncation and a TCP retry (implement TCP fallback or explicitly document it as deferred).

**Tools.** Go stdlib `net`, `context`. Testing with `dig @localhost -p <port> example.com`.

**Deliverable.** `dig` against your resolver returns correct answers for common record types (A, AAAA, CNAME).

**Acceptance criteria.** Correct answers for at least A, AAAA, and CNAME queries, graceful handling of a malformed packet without crashing, and a request timeout that returns rather than hanging.

**Interview value.** UDP socket programming, binary protocol parsing, the TCP vs UDP tradeoff in DNS, request timeout handling.

---

### Stage 2: Concurrent TTL aware LRU cache

**Goal.** The core of the project. A concurrency safe cache that respects each record's TTL and evicts by least recently used when full, with a measurable hit rate. This is the stage that turns a forwarder into a real resolver and produces the strongest resume bullet.

**Work.**
- Implement an LRU cache: a hash map for O(1) lookup plus a doubly linked list for O(1) move to front and eviction. Do not import a cache library for the core; hand roll it so you can explain the data structure. A library is acceptable only after you have demonstrated the hand rolled version.
- Make it TTL aware: each entry stores the record's TTL and an insertion timestamp. On lookup, expired entries are treated as a miss and evicted. The remaining TTL is what you return to the client, decremented by time spent in cache, which is what a correct resolver does.
- Make it concurrency safe: guard the cache with a `sync.RWMutex` so many readers can hit concurrently while writes are exclusive. Be ready to discuss why `RWMutex` over a plain `Mutex` here, and the tradeoff of `sync.Map` as an alternative.
- Introduce the bounded worker pool: a fixed number of goroutines consuming queries from a channel, so concurrency is capped rather than spawning an unbounded goroutine per packet under load.
- Prove correctness under concurrency: run the cache's tests with `go test -race` and make the race detector clean a hard requirement.

**Tools.** Go stdlib `sync`, `container/list` (or a hand rolled list), `testing`, the race detector.

**Deliverable.** A cache with unit tests, a clean `-race` run, and an instrumented hit rate you can observe (even if just logged at this stage).

**Acceptance criteria.** Repeated queries for the same name are served from cache and are measurably faster than a forwarded miss; expired entries are refetched; the cache never exceeds its capacity bound; `go test -race ./...` is clean.

**Interview value.** LRU design with the map plus linked list mechanism, TTL expiry logic, `RWMutex` vs `Mutex` vs `sync.Map` tradeoff, bounded worker pool, race free concurrency proven with tooling. This is the stage you talk about most.

---

### Stage 3: Prometheus instrumentation

**Goal.** The resolver reports what it is doing. Metrics for volume, cache effectiveness, and latency distribution, exposed on a scrapeable endpoint.

**Work.**
- Add the Prometheus Go client and register metrics: counters for total queries and cache hits and misses, a histogram for resolution latency (so you get p50, p99, p999), and gauges for current cache size and in flight requests.
- Expose `/metrics` over HTTP on a separate port from the DNS listener.
- Instrument the actual code paths: increment the hit or miss counter in the cache lookup, observe latency around the full resolve, update the size gauge on insert and evict.
- Understand and be able to explain the pull model: Prometheus scrapes your `/metrics` endpoint on an interval, you do not push.

**Tools.** `prometheus/client_golang`, Go stdlib `net/http`.

**Deliverable.** A live `/metrics` endpoint exposing query volume, hit rate, and a latency histogram.

**Acceptance criteria.** Hitting `/metrics` shows real counters incrementing as queries flow, the latency histogram populates with buckets, and the cache hit ratio is derivable from the exposed counters.

**Interview value.** Service instrumentation, the difference between counters, gauges, and histograms, why histograms are how you get percentiles, the Prometheus pull model.

---

### Stage 4: Containerization and the monitoring stack

**Goal.** The whole system runs as a monitored stack with one command. This is the operational maturity layer that most student projects lack entirely.

**Work.**
- Write a multi stage Dockerfile: build the binary in a full Go image, then copy just the static binary into a `scratch` or distroless final image. Be able to explain why the final image is a few megabytes and why that matters (attack surface, pull speed).
- Write a `docker-compose.yml` bringing up three services: your resolver, Prometheus (configured to scrape the resolver's `/metrics`), and Grafana (provisioned with Prometheus as a data source).
- Commit a Prometheus scrape config and a Grafana dashboard definition so the monitoring comes up preconfigured rather than needing manual clicking.
- Build a Grafana dashboard showing query rate, cache hit ratio over time, and the latency percentiles. Screenshot it for the README and your portfolio.
- Add container health checks and document how to run the stack.

**Tools.** Docker (multi stage build), Docker Compose, Prometheus, Grafana.

**Deliverable.** `docker compose up` brings up the resolver plus a live Prometheus and a preprovisioned Grafana dashboard.

**Acceptance criteria.** One command starts everything, the Grafana dashboard shows live traffic when you run queries against the resolver, and the final resolver image is small (single digit to low double digit megabytes).

**Interview value.** Multi stage Docker builds and why they matter, multi service orchestration with Compose, the Prometheus plus Grafana observability pattern, infrastructure as committed config rather than manual setup.

---

### Stage 5: Load testing and the numbers

**Goal.** Replace guesses with measured facts. This stage produces the actual figures that go on your resume, so it is not optional if you want a defensible bullet.

**Work.**
- Drive load with `dnsperf` or a custom concurrent Go client that fires a realistic mix of queries (some repeated to exercise the cache, some unique to force misses).
- Measure, at increasing concurrency levels: throughput in queries per second, latency percentiles (p50, p99, p999), and cache hit rate.
- Record a clear before and after: latency and throughput with the cache disabled versus enabled, which is the single most compelling comparison and mirrors how strong resume bullets are framed.
- Write the results into the repo (a `BENCHMARKS.md` with a short methodology note, the hardware you ran on, and a results table), because undocumented numbers are not credible.

**Tools.** `dnsperf` or a custom Go load client, the metrics from Stage 3 to read the results.

**Deliverable.** A benchmarks document with a reproducible methodology and a cache on versus cache off comparison.

**Acceptance criteria.** You can state throughput and tail latency with a real number and explain how you measured it, including the methodology and its limits.

**Interview value.** Benchmarking discipline, the meaning of tail latency, honest measurement including stating what your test does not cover.

---

## Deliverables checklist

By the end, the repository contains:

- A working Go DNS resolver (`cmd/resolver`).
- A hand rolled, concurrency safe, TTL aware LRU cache with unit tests and a clean race detector run.
- A bounded worker pool for capped concurrency.
- Prometheus metrics on a `/metrics` endpoint (counters, gauges, latency histogram).
- A multi stage Dockerfile producing a small image.
- A `docker-compose.yml` bringing up resolver, Prometheus, and Grafana.
- A committed Grafana dashboard and Prometheus scrape config.
- A `BENCHMARKS.md` with a cache on versus off comparison and methodology.
- This `PROJECT_SPEC.md` and a README linking to it.

## How to talk about this in an interview

Lead with the shape of the system, then go deep on the cache, because that is where the real engineering decisions are. Be ready to defend three choices specifically: why `RWMutex` over `Mutex` (or when `sync.Map` would win), why LRU over LFU or plain TTL eviction, and why a bounded worker pool over a goroutine per request. Have your benchmark numbers and their methodology ready, and be honest about what the numbers do not prove (single node, synthetic load, no real network variance). If asked what you would do next, the strongest answers are multi threaded upstream fan out, DNSSEC validation, or an eviction policy comparison, all of which show you understand the current design's limits.

## Candidate resume bullets

Drafts to adapt once you have real numbers. Keep them mechanism plus measured outcome, no invented figures.

- Built a concurrent DNS resolver in Go with a bounded worker pool and a hand rolled TTL aware LRU cache (hash map plus doubly linked list, O(1) lookup and eviction), guarded by a read write mutex and verified race free with the Go race detector.
- Instrumented the resolver end to end with Prometheus counters, gauges, and latency histograms, and deployed it as a Docker Compose stack with Prometheus and a preprovisioned Grafana dashboard.
- Load tested with a concurrent query client and measured a cache on versus off improvement of [fill in]x throughput and [fill in] reduction in p99 latency at [fill in] queries per second.
