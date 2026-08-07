# JobQueue

A small but fairly complete asynchronous job queue written in Go.

I built this project to explore the parts of backend systems that usually get hidden behind libraries: queueing, workers, retries, failure states, graceful shutdown, recovery after restarts, and keeping PostgreSQL as the source of truth while still using an in-memory queue for fast dispatch.

The current demo workload is image processing, but the queue itself is generic. Jobs are stored with a type and JSON payload, then routed to a handler that knows how to validate and process that job.

## What it does

- Accepts jobs over HTTP
- Stores job state in PostgreSQL
- Enqueues work in memory with `high`, `normal`, and `low` priorities
- Processes jobs concurrently with a worker pool
- Retries ordinary failures with backoff
- Marks validation errors and panics as failed jobs
- Recovers queued/interrupted jobs on restart
- Exposes Prometheus metrics
- Includes unit tests and PostgreSQL integration tests

## Why this project exists

The point was not to wrap a queue library and call it a day.

I wanted to build the mechanics myself:

- how workers safely claim jobs
- how priority affects dispatch
- how retries should update durable state
- what happens when the process dies mid-job
- how handlers should fail without taking the whole worker pool down
- how to test concurrent code without relying on luck and sleeps everywhere

It is intentionally small enough to read, but complete enough to behave like a real backend service.

## Architecture

```text
HTTP API
   |
   v
PostgreSQL  <---- recovery / state transitions
   |
   v
In-memory priority queue
   |
   v
Worker pool
   |
   v
Job handlers
```

PostgreSQL owns the durable state. The in-memory queue is only used to dispatch job IDs quickly to workers.

That means if the process restarts, the app can rebuild the queue from the database instead of losing track of work.

## Job lifecycle

```text
queued -> processing -> success
   |          |
   |          v
   |       queued        retryable failure
   |
   v
 fail          validation error, panic, unsupported type, or max retries
```

Workers claim jobs atomically from PostgreSQL before processing them. A job only moves to `processing` if it was still `queued`, which prevents multiple workers from processing the same job.

## Supported jobs

### Image processing

Job type:

```json
"image.processing"
```

Supported image operations:

- `resize`
- `grayscale`
- `compress`

Example payload:

```json
{
  "job_type": "image.processing",
  "priority": "high",
  "payload": {
    "image_job_type": "resize",
    "image_path": "./input/photo.jpg",
    "params": {
      "width": 800,
      "height": 600
    }
  }
}
```

### Flaky job

Job type:

```json
"flaky"
```

This handler is mostly useful for testing retries, timeouts, failures, and panic recovery.

Example payload:

```json
{
  "job_type": "flaky",
  "priority": "normal",
  "payload": {
    "fail_rate": 0.5,
    "delay_sec": 2
  }
}
```

## HTTP API

### Create a job

```http
POST /jobs
Content-Type: application/json
```

```json
{
  "job_type": "image.processing",
  "priority": "normal",
  "payload": {
    "image_job_type": "grayscale",
    "image_path": "./input/photo.jpg",
    "params": {
      "quality": 0.8
    }
  }
}
```

Response:

```json
{
  "job_id": 1
}
```

If priority is omitted, it defaults to `normal`.

### Get job state

```http
GET /jobs/{id}
```

### Get job result

```http
GET /jobs/{id}/results
```

Returns:

- `200 OK` when the job succeeded
- `202 Accepted` while the job is still queued or processing
- `410 Gone` when the job failed
- `404 Not Found` when the job does not exist

### Metrics

```http
GET /metrics
```

Prometheus metrics are exposed through the default Prometheus HTTP handler.

## Running locally

You need:

- Go
- PostgreSQL
- Goose or any other way to apply the SQL migrations

Create a `.env` file:

```env
DB_URL=postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable
```

Apply the migrations in order from:

```text
internal/db/migrations
```

Then run:

```bash
go run ./cmd/server
```

## Running tests

Unit tests do not need PostgreSQL:

```bash
go test ./...
```

Race-enabled test run:

```bash
go test -race ./...
```

Integration tests run only when `TEST_DATABASE_URL` is set:

```bash
TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobqueue_test?sslmode=disable go test ./...
```

The integration helpers refuse to clean a database unless its name clearly looks like a test database.

## CI

The GitHub Actions workflow starts PostgreSQL, applies migrations, then runs:

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go vet ./...
```

Coverage is reported, not used as an arbitrary gate.

## Project layout

```text
cmd/server              app entrypoint
internal/api            HTTP handlers and routes
internal/app            app startup, recovery, and shutdown
internal/db             SQL queries, migrations, and generated sqlc code
internal/images         image decode/save helpers
internal/jobs           job registry and job types
internal/jobs/handler   image and flaky job handlers
internal/queue          priority queue and scheduler
internal/workers        worker pool and job processing
internal/testutil       PostgreSQL integration-test helpers
```

## Notes

This project intentionally uses strict priority ordering, so lower-priority jobs can starve if high-priority work keeps arriving. That tradeoff is tested and treated as part of the design.

The queue capacity is also a soft admission limit. Recovery can force jobs back into the queue because durable correctness matters more than temporary in-memory limits after a restart.

## Status

This is a portfolio/backend systems project, not a production queue replacement. The interesting part is the implementation: concurrency, durable job state, retries, recovery, cancellation, and test coverage around those behaviors.
