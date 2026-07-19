# AsyncChannelWriter Atomic Close Design

## Goal

Fix repeated/concurrent Close and concurrent Write/Close panics while preserving or improving the normal Write hot path. No mutex or RWMutex may be used by Write.

## Scope

- Modify only component/logging/writer/async_channel_writer.go.
- Add focused tests and benchmarks under component/logging/writer/.
- Preserve all public signatures and the existing one-second full-channel drop behavior.
- Do not change logger wiring, callers, configuration, or other writer implementations.

## Protocol

activeWriters counts writes that may still send to logChan. A write increments the counter before observing closed; a post-close write decrements immediately and returns (0, error). An admitted write copies and sends, or times out, then decrements before drop diagnostics.

Close atomically changes closed from open to closed, waits until activeWriters reaches zero, then closes logChan. A capacity-one writersDrained notification prevents busy waiting. Later Close calls wait on closeDone and return the first result.

This ordering covers all races:

- Write increments first and observes open: Close sees it as active and waits.
- Close marks closed first: a later Write observes closed and never sends.
- Close observes zero before a later rejected Write increments: that Write still observes closed and never sends.

## Fast Send

After copying, Write first attempts a non-blocking channel send. Only a full channel creates the one-second timer. This removes three timer allocations from the common path and offsets the unavoidable active-writer atomic accounting.

## Acceptance

- No mutex/RWMutex in the Write hot path.
- Deterministic RED on the original implementation for repeated Close and concurrent Write/Close.
- Focused and package race tests pass.
- Full writer benchmarks cover serial and parallel 64-byte writes.
- Candidate median throughput must not regress versus main; common-path allocations should fall from four to approximately one per write.
- Any unmet safety or performance condition stops the task without widening the architecture.
