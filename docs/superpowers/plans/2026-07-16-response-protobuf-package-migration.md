# Response Protobuf Package Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Protobuf response schema and generated Go message from the misleading `rpc/protosrc` package to the cohesive `response/pb` package without retaining a compatibility shim.

**Architecture:** `response/pb` owns the wire schema and generated `responsepb` Go package. The hand-written `response` package imports this child package and continues to expose the existing `IResponse`/`RespInfoPB` behavior, while the protobuf message full name and field numbers remain unchanged.

**Tech Stack:** Go 1.25, Protocol Buffers compiler 33.0, `protoc-gen-go` 1.36.10, `google.golang.org/protobuf` 1.36.11.

## Global Constraints

- Breaking removal of `github.com/lamxy/fiberhouse/rpc/protosrc` is allowed; do not add a compatibility package.
- Keep protobuf package `response`, message name `RespInfoProto`, and field numbers/types unchanged.
- Declare the generated Go package as `github.com/lamxy/fiberhouse/response/pb;responsepb`.
- Regenerate `resp_info.pb.go`; do not maintain generated code by hand.
- Preserve the unrelated existing modification to `plugins/README.md`.
- The pre-change full suite has unrelated failures in `bootstrap` and `component/writer`; scoped migration verification must pass independently.

---

### Task 1: Lock the new descriptor location with a failing test

**Files:**
- Create: `response/response_proto_impl_test.go`

**Interfaces:**
- Consumes: `GetRespInfoPB() IResponse`, `RespInfoPB.pb`, and the generated protobuf reflection API.
- Produces: `TestRespInfoPBUsesResponsePBDescriptor`, covering the descriptor path, stable message full name, and wire round-trip.

- [x] **Step 1: Write the migration regression test**

```go
package response

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestRespInfoPBUsesResponsePBDescriptor(t *testing.T) {
	resp := GetRespInfoPB().(*RespInfoPB)
	defer resp.Release()

	resp.Reset(7, "moved", map[string]any{"ok": true})
	descriptor := resp.pb.ProtoReflect().Descriptor()
	if got, want := descriptor.ParentFile().Path(), "response/pb/resp_info.proto"; got != want {
		t.Fatalf("descriptor path = %q, want %q", got, want)
	}
	if got, want := string(descriptor.FullName()), "response.RespInfoProto"; got != want {
		t.Fatalf("message full name = %q, want %q", got, want)
	}

	body, err := proto.Marshal(resp.pb)
	if err != nil {
		t.Fatalf("marshal RespInfoProto: %v", err)
	}
	decoded := resp.pb.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(body, decoded); err != nil {
		t.Fatalf("unmarshal RespInfoProto: %v", err)
	}
	message := decoded.ProtoReflect()
	if got := message.Get(descriptor.Fields().ByName("code")).Int(); got != 7 {
		t.Fatalf("round-trip code = %d, want 7", got)
	}
	if got := message.Get(descriptor.Fields().ByName("msg")).String(); got != "moved" {
		t.Fatalf("round-trip msg = %q, want %q", got, "moved")
	}
}
```

- [x] **Step 2: Verify the test fails for the old descriptor path**

Run: `env GOCACHE=/tmp/fiberhouse-gocache go test ./response -run '^TestRespInfoPBUsesResponsePBDescriptor$' -count=1`

Expected: FAIL with `descriptor path = "rpc/protosrc/resp_info.proto", want "response/pb/resp_info.proto"`.

### Task 2: Move and regenerate the protobuf package

**Files:**
- Create: `response/pb/resp_info.proto`
- Create: `response/pb/generate.go`
- Generate: `response/pb/resp_info.pb.go`
- Delete: `rpc/protosrc/resp_info.proto`
- Delete: `rpc/protosrc/resp_info.pb.go`
- Modify: `response/response_proto_impl.go:9-18`

**Interfaces:**
- Consumes: `google.protobuf.Any` and the stable protobuf contract `response.RespInfoProto`.
- Produces: importable Go package `github.com/lamxy/fiberhouse/response/pb` with Go package name `responsepb` and type `RespInfoProto`.

- [x] **Step 1: Create the canonical schema at the new location**

```protobuf
syntax = "proto3";

package response;

import "google/protobuf/any.proto";

option go_package = "github.com/lamxy/fiberhouse/response/pb;responsepb";

// RespInfoProto 对应 RespInfo 的 protobuf 消息定义
message RespInfoProto {
  int32 code = 1;
  string msg = 2;
  google.protobuf.Any data = 3;
}
```

- [x] **Step 2: Install pinned generators outside the repository and regenerate**

Run:

```bash
curl -fsSL -o /tmp/fiberhouse-protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v33.0/protoc-33.0-linux-x86_64.zip
unzip -oq /tmp/fiberhouse-protoc.zip -d /tmp/fiberhouse-protoc
env GOBIN=/tmp/fiberhouse-tools GOMODCACHE=/tmp/fiberhouse-gomodcache go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
/tmp/fiberhouse-protoc/bin/protoc -I . -I /tmp/fiberhouse-protoc/include --plugin=protoc-gen-go=/tmp/fiberhouse-tools/protoc-gen-go --go_out=. --go_opt=paths=source_relative response/pb/resp_info.proto
```

Expected: `response/pb/resp_info.pb.go` declares `package responsepb`, source path `response/pb/resp_info.proto`, and `File_response_pb_resp_info_proto`.

- [x] **Step 3: Add the durable generation entry point**

```go
// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT

//go:generate protoc -I ../.. --go_out=../.. --go_opt=paths=source_relative ../../response/pb/resp_info.proto

// Package responsepb contains the generated Protobuf representation of unified responses.
package responsepb
```

Run: `GOCACHE=/tmp/fiberhouse-gocache PATH=/tmp/fiberhouse-protoc/bin:/tmp/fiberhouse-tools:$PATH go generate ./response/pb`

Expected: generation succeeds and leaves the descriptor path as `response/pb/resp_info.proto`.

- [x] **Step 4: Point the hand-written response implementation at the new package**

Use this import in `response/response_proto_impl.go`:

```go
pb "github.com/lamxy/fiberhouse/response/pb"
```

- [x] **Step 5: Remove the old schema and generated package**

Delete both files under `rpc/protosrc`; the empty root `rpc` directory must disappear from the tracked tree.

- [x] **Step 6: Verify the migration test is green**

Run: `env GOCACHE=/tmp/fiberhouse-gocache go test ./response ./response/pb -count=1`

Expected: PASS for both packages.

### Task 3: Update durable documentation and verify the repository

**Files:**
- Modify: `docs/reference/feature-status.md`
- Modify: `docs/reference/components.md`
- Modify: `docs/guides/response-and-serialization.md`
- Modify: `.codegraph-qa-out/entrypoints.md`
- Modify: `.codegraph-qa-out/codebase_summary.md`

**Interfaces:**
- Consumes: the final `response/pb` path and the current distinction between HTTP response serialization and RPC.
- Produces: documentation that no longer presents root `rpc/protosrc` as RPC support.

- [x] **Step 1: Replace current-state references to the old package**

Apply these exact current-state documentation changes:

- In `docs/reference/feature-status.md`, describe RPC evidence as `` `component/rpc` 只有空占位文件；统一响应的 Protobuf schema 与生成代码位于 `response/pb` `` and replace the review-source reference to `` `rpc/` `` with `` `response/pb` ``.
- In `docs/reference/components.md`, change the `component/rpc` lifecycle statement to ``无 RPC client/server；`response/pb` 仅提供 HTTP 统一响应的 Protobuf 数据契约，不提供 RPC 生命周期``.
- In `docs/guides/response-and-serialization.md`, identify the Protobuf structure as `` `response/pb.RespInfoProto{code,msg,data}` ``.
- In `.codegraph-qa-out/entrypoints.md`, point the generated descriptor initializer to `response/pb/resp_info.pb.go`.
- In `.codegraph-qa-out/codebase_summary.md`, replace the claimed RPC capability with Protobuf response encoding under `response/pb` and remove the obsolete root `rpc/` directory row.

- [x] **Step 2: Check for stale current-state references**

Run: `rg -n 'rpc/protosrc|github.com/lamxy/fiberhouse/rpc/protosrc' . --glob '!docs/superpowers/**' --glob '!.git/**'`

Expected: no matches. Historical implementation plans/specifications may retain historical wording.

- [x] **Step 3: Format and run scoped verification**

Run:

```bash
gofmt -w response/response_proto_impl.go response/response_proto_impl_test.go response/pb/resp_info.pb.go
env GOCACHE=/tmp/fiberhouse-gocache go test ./response ./response/pb -count=1
env GOCACHE=/tmp/fiberhouse-gocache go test ./... -run '^$'
```

Expected: formatting produces no semantic diff; scoped tests pass; repository-wide compile-only test exits successfully.

- [x] **Step 4: Re-run the full suite and compare with baseline**

Run: `env GOCACHE=/tmp/fiberhouse-gocache go test ./... -count=1`

Expected: no migration-related failures. Existing `bootstrap` and `component/writer` failures may remain and must be reported exactly rather than attributed to this change.

- [x] **Step 5: Review the final patch**

Run: `git diff --check` and `git status --short`.

Expected: no whitespace errors; `plugins/README.md` remains modified but unchanged by this work; all migration files are visible as intended.
