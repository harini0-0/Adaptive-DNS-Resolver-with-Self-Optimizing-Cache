# Project Helix

## Adaptive Telemetry-Driven DNS Resolver

A concurrent DNS resolver written in Go featuring a thread-safe TTL-aware cache, bounded worker pool, Prometheus-based observability, Dockerized deployment, and an adaptive optimization engine that continuously improves cache behavior through telemetry-driven machine learning.

---

# Vision

Most student networking projects end once they correctly answer DNS queries.

Project Helix takes a different approach.

The objective is not to build "yet another DNS resolver." The objective is to build a production-inspired backend service capable of serving large volumes of DNS requests while continuously learning from its own operational telemetry to improve cache efficiency over time.

The resolver begins with deterministic engineering principles—a concurrent architecture, a bounded worker pool, a thread-safe TTL-aware LRU cache, comprehensive instrumentation, and reproducible deployment. Once this baseline is established and benchmarked, a telemetry-driven optimization layer is introduced.

Instead of replacing deterministic logic with machine learning, Helix augments it.

The resolver exports detailed operational metrics into Prometheus. Historical telemetry becomes training data for an offline machine learning pipeline. Every week, the system builds a candidate prediction model capable of identifying cache entries likely to be requested again and forecasting short-term traffic spikes. Candidate models are evaluated against the current production model and promoted only if they demonstrate measurable improvements.

The resolver therefore evolves over time without sacrificing correctness, predictability, or safety.

The project demonstrates modern backend engineering, distributed systems concepts, observability, performance engineering, and practical MLOps within a single cohesive system.

---

# Engineering Philosophy

The project follows four principles.

## 1. Correctness before Intelligence

The resolver must remain completely functional without machine learning.

Machine learning is an optimization layer, not a dependency.

If the prediction engine fails, the resolver immediately falls back to deterministic TTL-aware LRU behavior.

---

## 2. Measure Before Optimizing

Every optimization must be justified by data.

Performance improvements are measured through:

- throughput
- cache hit ratio
- p50 latency
- p95 latency
- p99 latency
- upstream DNS requests
- memory utilization

Nothing is claimed without benchmark evidence.

---

## 3. Operational Transparency

Every major subsystem exposes metrics.

The resolver should always be able to answer questions such as:

- How many requests were received?
- How many cache hits occurred?
- Which domains dominate traffic?
- How long does resolution take?
- How many upstream queries were avoided?
- Is cache efficiency improving over time?

---

## 4. Continuous Improvement

The resolver is designed to improve itself gradually.

Telemetry becomes historical knowledge.

Historical knowledge becomes better prediction models.

Better prediction models improve cache efficiency.

Improved cache efficiency generates better telemetry.

This creates the Helix feedback cycle.

---

# Project Goals

The resolver demonstrates:

- Concurrent UDP networking
- Thread-safe shared state
- TTL-aware caching
- LRU cache implementation
- Worker pool architecture
- Service instrumentation
- Production monitoring
- Containerized deployment
- Benchmark-driven optimization
- Machine learning assisted cache optimization
- Continuous model retraining

---

# Non Goals

Project Helix does not attempt to become:

- an authoritative DNS server
- a DNSSEC validating resolver
- a globally distributed resolver
- a Kubernetes deployment
- a lock-free systems project
- an AI-first networking project

The objective is engineering realism rather than feature completeness.

---

# High-Level Architecture

```
                    DNS Clients
                         │
                         ▼
                UDP Listener (Go)
                         │
                Incoming Request Queue
                         │
                         ▼
                Bounded Worker Pool
                         │
                         ▼
              Adaptive Cache Engine
          (TTL + LRU + ML Optimizer)
                 │              │
          Cache Hit         Cache Miss
                 │              │
                 │              ▼
                 │      Upstream Resolver
                 │      (Cloudflare/Google)
                 └──────────────┘
                         │
                         ▼
                Response Writer
                         │
                         ▼
                    DNS Client

------------------------------------------------------

Every request emits metrics

             Prometheus
                  │
                  ▼
          Historical Telemetry
                  │
                  ▼
         Weekly Training Pipeline
                  │
                  ▼
         Candidate ML Model
                  │
        Offline Evaluation
                  │
        Better than Current?
             │          │
            Yes         No
             │
             ▼
      Deploy New Model
```

---

# Core Components

## UDP Listener

Responsible for:

- receiving DNS packets
- validating packets
- extracting client information
- placing requests into the processing queue

The listener performs no heavy computation.

Its only responsibility is accepting traffic efficiently.

---

## Worker Pool

Instead of creating unlimited goroutines, Helix uses a bounded worker pool.

Benefits:

- predictable memory usage
- controlled concurrency
- improved stability
- reduced scheduler overhead

Incoming requests are placed into a channel.

Workers continuously consume requests from the queue.

---

## Adaptive Cache Engine

This is the heart of the project.

The cache consists of three layers.

### Layer 1

TTL validation

Each DNS record stores:

- domain
- record type
- response
- insertion timestamp
- original TTL

Expired records are automatically invalidated.

---

### Layer 2

LRU eviction

The cache is implemented using:

- Hash Map
- Doubly Linked List

Complexities:

Lookup

O(1)

Insertion

O(1)

Eviction

O(1)

---

### Layer 3

Prediction Engine

When eviction is required, the cache may consult the prediction engine.

If predictions are unavailable,

standard LRU behavior is used.

This guarantees deterministic correctness.

---

# Cache Decision Flow

```
Lookup

↓

Exists?

↓

No

↓

Forward upstream

↓

Insert

↓

Need Eviction?

↓

Prediction Available?

↓

Yes ------------------------ No

↓                           ↓

ML Score                Standard LRU

↓

Evict Lowest Score

↓

Insert
```

---

# Machine Learning Philosophy

Machine learning does **not** replace DNS logic.

It improves two decisions.

1. Which cache entries should remain?

2. Which DNS records should be prefetched?

Everything else remains deterministic.

---

# Adaptive Cache Retention

Traditional cache

```
Cache Full

↓

Remove Least Recently Used
```

Helix

```
Cache Full

↓

Predict future usefulness

↓

Keep entries with highest probability

↓

Evict lowest probability entry
```

The ML model predicts:

> "Will this DNS record likely be requested again within the next prediction window?"

---

# Predictive Prefetching

Traffic often follows patterns.

Examples:

Morning

- office365.com
- outlook.office.com

Evening

- youtube.com
- netflix.com

Work hours

- github.com
- aws.amazon.com

The prediction engine forecasts short-term traffic spikes.

Before users request these domains, the resolver refreshes them proactively.

This reduces cache misses during demand bursts.

---

# Telemetry Pipeline

The resolver continuously exports metrics.

```
Resolver

↓

Prometheus

↓

Historical Metrics

↓

Feature Engineering

↓

Training Dataset

↓

Weekly Retraining

↓

Candidate Model

↓

Evaluation

↓

Deployment
```

---

# Prometheus Metrics

The resolver exports:

## Traffic

- total queries
- queries per second
- upstream requests

---

## Cache

- hits
- misses
- hit ratio
- eviction count
- expired entries
- current size

---

## Performance

- latency histogram
- p50
- p95
- p99
- worker utilization

---

## Resource Usage

- active workers
- queue length
- goroutines
- memory usage

---

# Feature Engineering

Each cache entry becomes a training example.

Example features

| Feature | Description |
|----------|-------------|
| Domain | Requested domain |
| TTL Remaining | Remaining lifetime |
| Time Since Last Access | Seconds |
| Total Access Count | Lifetime accesses |
| Access Frequency | Requests/minute |
| Moving Average | Rolling window |
| Hour of Day | Encoded feature |
| Day of Week | Encoded feature |
| Burst Score | Sudden traffic increase |
| Cache Hit Ratio | Historical usefulness |
| Worker Queue Size | Current load |

Target label

```
Was this record requested again

within

next 60 seconds?

Yes / No
```

---

# Model Selection

The project intentionally avoids deep learning.

DNS telemetry is structured tabular data.

Tree-based methods perform exceptionally well.

### Baseline

Logistic Regression

Purpose

Simple interpretable baseline.

---

### Production Candidate

Random Forest

Advantages

- interpretable
- nonlinear
- robust
- fast inference

---

### Advanced Candidate

XGBoost

Advantages

- excellent performance
- handles feature interactions
- widely used in industry
- efficient

Primary production model:

XGBoost

Baseline comparison:

Random Forest

Reference comparison:

Logistic Regression

---

# Why Not Reinforcement Learning?

Although adaptive cache replacement resembles reinforcement learning conceptually, RL introduces unnecessary complexity.

Reasons:

- enormous data requirements
- unstable convergence
- exploration reduces cache efficiency
- difficult evaluation

Instead,

Helix performs periodic supervised retraining using historical telemetry.

This provides:

- reproducibility
- deterministic deployment
- measurable improvements

---

# Weekly Learning Cycle

Every Saturday morning

```
Collect Previous Week

↓

Generate Dataset

↓

Train Candidate

↓

Cross Validation

↓

Compare

↓

Current Production Model

↓

Better?

↓

YES

↓

Promote

↓

NO

↓

Discard
```

No model is deployed automatically without validation.

---

# Model Registry

Each model receives metadata.

Example

| Version | Accuracy | Hit Rate Improvement | Date |
|----------|-----------|---------------------|------|
| v1 | 82% | Baseline | Week 1 |
| v2 | 86% | +2.1% | Week 2 |
| v3 | 88% | +3.5% | Week 3 |

This enables rollback if necessary.

---

# Safety Mechanisms

Machine learning is optional.

Fallback order

1. Current Production Model

2. Previous Stable Model

3. Standard TTL-aware LRU

Resolver correctness is never compromised.

---

# Observability Stack

Docker Compose launches

- Resolver
- Prometheus
- Grafana

Grafana dashboards include

- Query throughput
- Cache hit ratio
- Upstream requests
- Worker utilization
- Queue length
- Latency percentiles
- Memory usage
- Cache occupancy
- Prediction effectiveness

---

# Benchmark Methodology

The resolver will be tested under increasing concurrency.

Metrics recorded:

- Throughput
- Cache hit rate
- Cache miss rate
- Upstream request reduction
- Average latency
- p95 latency
- p99 latency
- CPU utilization
- Memory utilization

Comparisons

1. No Cache

2. TTL-aware LRU

3. Adaptive Cache

4. Adaptive Cache + Prefetching

This quantifies the value of each optimization independently.

---

# Repository Structure

```
cmd/
    resolver/

internal/
    dns/
    cache/
    workerpool/
    metrics/
    predictor/
    prefetch/
    telemetry/

ml/
    feature_engineering/
    training/
    evaluation/
    registry/

docker/

grafana/

prometheus/

benchmarks/

docs/
```

---

# Development Roadmap

## Stage 0

Repository setup

---

## Stage 1

Concurrent UDP resolver

---

## Stage 2

TTL-aware LRU cache

---

## Stage 3

Worker pool

Concurrency validation

Race detection

---

## Stage 4

Prometheus instrumentation

Grafana dashboards

---

## Stage 5

Docker deployment

Benchmarking

Baseline measurements

---

## Stage 6

Adaptive cache retention model

Feature engineering

Offline training pipeline

Prediction integration

---

## Stage 7

Traffic prediction

Predictive prefetching

Benchmark comparison

---

## Stage 8

Continuous learning pipeline

Weekly retraining

Candidate evaluation

Model promotion

Model registry

---

# Future Extensions

Possible future work

- DNSSEC validation
- Multi-upstream load balancing
- Cache stampede prevention
- Singleflight request coalescing
- Distributed cache synchronization
- Adaptive cache sizing
- Online learning experiments
- Kubernetes deployment
- gRPC management API
- Distributed telemetry aggregation

---

# Expected Engineering Outcomes

By completion, Project Helix demonstrates

- High-performance concurrent networking
- Safe shared-memory concurrency
- Production-quality caching
- Operational observability
- Infrastructure automation
- Benchmark-driven engineering
- Data-driven cache optimization
- Practical MLOps
- Continuous system improvement

---

# Interview Narrative

Project Helix is not presented as an AI project.

It is presented as an adaptive infrastructure service.

A concise description:

> Project Helix is a concurrent DNS resolver written in Go that combines a thread-safe TTL-aware LRU cache, bounded worker pool, Prometheus-based observability, and an adaptive optimization layer trained from historical operational telemetry. The resolver continuously exports performance metrics, which are transformed into weekly training datasets used to build candidate machine learning models for intelligent cache retention and predictive prefetching. New models are benchmarked against the current production model and promoted only if they improve cache efficiency, while deterministic TTL-aware LRU remains the fallback policy to guarantee correctness.

---

# Candidate Resume Bullets

- Designed and implemented **Project Helix**, an adaptive concurrent DNS resolver in Go featuring a thread-safe TTL-aware cache, bounded worker pool, Prometheus instrumentation, and Dockerized deployment.
- Developed a telemetry-driven cache optimization framework that trains predictive models from historical Prometheus metrics to improve cache retention and proactively prefetch high-demand DNS records.
- Built an automated model evaluation and promotion pipeline that periodically retrains candidate models, benchmarks them against the current production policy, and safely falls back to deterministic TTL-aware LRU when predictions are unavailable.
- Benchmarked resolver performance under synthetic load, comparing uncached resolution, deterministic TTL-aware LRU, and telemetry-driven adaptive caching using throughput, latency percentiles, and cache hit ratio as evaluation metrics.