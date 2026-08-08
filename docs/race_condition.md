# The race condition I found (and fixed)

Found while building the `/stats` endpoint, a counter of completed and
failed jobs since startup.

**Note on how this came about:** I used a plain `int` counter knowing it
would race under concurrent access, specifically to have a real bug to
find and fix with the race detector. The bug itself is realistic though:
using a plain variable for a counter shared across goroutines, instead of
`sync/atomic` or a mutex, is a genuinely common mistake in real Go code.
I just introduced it deliberately here instead of hitting it by accident.

## The bug

```go
current.JobsCompletedSinceStartup++
```

Called from every worker goroutine whenever a job finishes. `count++` is
actually three steps: read, add one, write back. Two goroutines running
this at the same time can both read the same value and both write back
the same result, silently losing one increment.

## Catching it

```
go test -race ./internal/stats
```

A test firing many goroutines at the counters concurrently caught the
race immediately, with as few as 2 goroutines doing 1 increment each.

## The fix

```go
var (
    completed atomic.Int64
    failed    atomic.Int64
)

func IncrementCompleted() {
    completed.Add(1)
}
```

`Add(1)` is a single atomic operation, no read-modify-write sequence to
collide on. Used atomics instead of a mutex since each counter is
independent, no multi-step operation across fields that needs locking.

Also updated the test to check the exact final count, not just whether it
ran without racing, since a race can lose updates and still produce a
number that looks plausible but is wrong. `go test -race` now passes
cleanly with the correct count.