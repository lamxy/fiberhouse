# Gin TLS Minimal Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing Gin TLS configuration load valid certificates and make the existing runtime use TLS when that configuration is present.

**Architecture:** Keep `CoreStarter`, `CoreWithGin`, `InitCoreApp`, and `AppCoreRun` unchanged as interfaces and lifecycle entry points. Patch only the unreachable certificate-loading path and the existing server call selection; preserve current logging and invalid-certificate fallback behavior.

**Tech Stack:** Go 1.25, `crypto/x509`, `encoding/pem`, `net/http/httptest`, `testify`, CodeGraph, ast-grep.

## Global Constraints

- Do not add `Run(ctx context.Context) error` or any equivalent entry point.
- Do not extract lifecycle helpers, add server interfaces, or reorganize Gin startup code.
- Do not change public APIs or Fiber behavior.
- Do not change existing behavior for missing or invalid certificate files.
- Production changes stay inside `core_gin_starter_impl.go`.

---

### Task 1: Repair the Existing Gin TLS Path

**Files:**
- Modify: `core_gin_starter_impl.go:156-193,318-342`
- Test: `core_starter_init_test.go`

**Interfaces:**
- Consumes: `(*CoreWithGin).initHttpServer(appconfig.IAppConfig)` and `(*CoreWithGin).AppCoreRun(...IProviderManager)`.
- Produces: the same methods and signatures, with a populated `http.Server.TLSConfig` for valid files and `ListenAndServeTLS("", "")` selected when that field is non-nil.

- [x] **Step 1: Write the failing certificate-loading regression test**

  Create a temporary certificate/key pair by copying the DER certificate and private key from `httptest.NewTLSServer` into PEM files. Configure:

  ```go
  values := map[string]interface{}{
      "application.plugins.engine.servers.gin.tls.enable":   true,
      "application.plugins.engine.servers.gin.tls.certFile": certFile,
      "application.plugins.engine.servers.gin.tls.keyFile":  keyFile,
  }
  ```

  Call `InitCoreApp(&task4Frame{}, task4GoodCodecManager())` and require that it does not panic, `core.httpServer.TLSConfig` is non-nil, and it contains one certificate.

- [x] **Step 2: Confirm the regression test is red**

  Run:

  ```bash
  go test . -run '^TestCoreInit_GinTLSLoadsConfiguredCertificate$' -count=1
  ```

  Expected before the fix: FAIL because the valid non-empty paths reach `panic(msg)`.

- [x] **Step 3: Apply the minimal production patch**

  Delete only the `panic(msg)` after the existing “Enabling TLS/HTTPS” log so `tls.LoadX509KeyPair` becomes reachable. In the existing `AppCoreRun` goroutine, select the serve call locally:

  ```go
  var err error
  if app.httpServer.TLSConfig != nil {
      err = app.httpServer.ListenAndServeTLS("", "")
  } else {
      err = app.httpServer.ListenAndServe()
  }
  if err != nil && !errors.Is(err, http.ErrServerClosed) {
      // retain the existing fatal log body unchanged
  }
  ```

- [x] **Step 4: Verify behavior and scope**

  Run:

  ```bash
  go test . -run '^TestCoreInit_GinTLSLoadsConfiguredCertificate$' -count=1
  go test ./... -count=1
  ast-grep run --pattern '$S.ListenAndServeTLS("", "")' --lang go core_gin_starter_impl.go
  ast-grep run --pattern '$S.ListenAndServe()' --lang go core_gin_starter_impl.go
  git diff --check
  git diff --stat
  ```

  Expected: both test commands pass; the TLS and plaintext calls each occur once in the same existing startup block; production diff is limited to `core_gin_starter_impl.go`; no exported declaration changes.

- [x] **Step 5: Commit the isolated patch**

  ```bash
  git add core_gin_starter_impl.go core_starter_init_test.go docs/superpowers/plans/2026-07-18-gin-tls-minimal-fix.md
  git commit -m "fix: enable configured Gin TLS"
  ```
