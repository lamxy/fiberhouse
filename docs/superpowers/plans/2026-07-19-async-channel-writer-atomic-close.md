# AsyncChannelWriter Atomic Close Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make AsyncChannelWriter close safe without a mutex hot path and recover the synchronization cost by avoiding timer allocation on immediate sends.

**Architecture:** Use an atomic closed flag, atomic active-writer count, a capacity-one drain notification, and the existing channel-drain consumer. Close seals admission, waits active channel operations, then closes the data channel. Write tries a non-blocking send before allocating a timer.

**Tech Stack:** Go atomics, channels, race detector, Go benchmarks.

## Global Constraints

- Keep Write([]byte) (int, error), Close() error, and DroppedLogs() int64 unchanged.
- Do not add mutex/RWMutex to Write or add a new public API.
- Modify only the channel writer, focused tests/benchmarks, and these approved design records.
- Preserve input copying, one-second drop timeout, final drain/flush, and first-close error publication.
- Do not merge or synchronize to main.

---

### Task 1: Atomic admission, close safety, and fast send

**Files:**
- Modify: component/logging/writer/async_channel_writer.go
- Create: component/logging/writer/async_channel_writer_test.go
- Create: component/logging/writer/async_channel_writer_benchmark_test.go

**Interfaces:**
- Preserves: (*AsyncChannelWriter).Write([]byte) (int, error)
- Preserves: (*AsyncChannelWriter).Close() error
- Preserves: (*AsyncChannelWriter).DroppedLogs() int64

- [ ] **Step 1: Add deterministic close regressions**

Add real temporary-file tests for sequential/concurrent idempotent Close and concurrent Write/Close. Use an unbuffered channel, runtime.GOMAXPROCS(1), and bounded runtime.Stack condition polling to prove a writer is blocked inside the old send select before Close starts. Recover and report panics; join all goroutines.

- [ ] **Step 2: Verify RED on main implementation**

Run:

    GOCACHE=/tmp/fiberhouse-async-atomic-red rtk go test -race ./component/logging/writer -run AsyncChannelWriter -count=1

Expected: repeated Close fails with close of closed channel; concurrent Write/Close exposes send on closed channel and/or the race detector send/close conflict.

- [ ] **Step 3: Implement atomic admission and idempotent Close**

Use private fields: closed int32, activeWriters int64, writersDrained chan struct{} with capacity one, closeDone chan struct{}, and closeErr error.

Write increments activeWriters, observes closed, and either releases then rejects or copies and performs send/timeout. finishWrite decrements the count. If it reaches zero after Close marked closed, it performs a non-blocking send to writersDrained.

Close uses one CompareAndSwapInt32 to select the first closer, loops while activeWriters is nonzero receiving drain notifications, closes logChan, waits for consume/final Flush, closes lumberjack, stores closeErr, and closes closeDone.

- [ ] **Step 4: Verify safety GREEN and commit**

Run:

    GOCACHE=/tmp/fiberhouse-async-atomic-green rtk go test -race ./component/logging/writer -run AsyncChannelWriter -count=1
    GOCACHE=/tmp/fiberhouse-async-atomic-green rtk go test -race ./component/logging/writer -count=1

Commit: fix(logging): close async channel writer without locks.

- [ ] **Step 5: Add the timer-free immediate-send path and benchmarks**

Before creating a timer, perform a non-blocking select that sends to logChan and returns after finishWrite. Only the full-channel path creates and stops the one-second timer. Add serial and RunParallel 64-byte benchmarks using a large channel and buffer.

- [ ] **Step 6: Verify performance and commit**

Run the same external benchmark against main and the candidate with count 10 and cpu 1,8. Record raw results, medians, and allocations. The candidate must not regress serial or parallel median throughput and should reduce the immediate-send path from four allocations to approximately one.

Final commands:

    GOCACHE=/tmp/fiberhouse-async-atomic-final rtk go test -race ./component/logging/writer -count=1
    rtk git diff --check

Commit: perf(logging): avoid timer allocation on immediate send.
