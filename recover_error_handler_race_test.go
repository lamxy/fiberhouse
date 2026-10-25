package fiberhouse

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DefaultStackTraceHandler 由 Fiber/Gin 的全局 ErrorHandler 在每个错误请求上调用，
// 并发请求必须不共享可写状态。用 -race 运行才能暴露问题。
func TestErrorHandler_DefaultStackTraceHandler_ConcurrentCallsDoNotRace(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	handler := &ErrorHandler{
		AppCtx: ctx,
		recoverManager: &task6NilProviderRecoverManager{
			IProviderManager: NewProviderManager(ctx),
		},
	}

	const workers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			handler.DefaultStackTraceHandler(nil, errors.New("boom"))
		}()
	}
	close(start)
	wg.Wait()
}

// TraceID 未传 flag 时应使用 trace 配置的字段名，而不是曾被错误处理器改写的包级变量。
func TestFiberRecovery_TraceID_UsesConfiguredRequestIDKey(t *testing.T) {
	ctx := newTask5AppContext(t, false, false) // application.trace.requestID = X-Trace-ID
	recovery := NewFiberRecovery(ctx)

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		c.Locals("X-Trace-ID", "trace-123")
		wrapped := adaptorctx.WithFiberContext(c)
		defer wrapped.(*adaptorctx.FiberContext).Release()
		assert.Equal(t, "trace-123", recovery.TraceID(wrapped))
		return c.SendStatus(http.StatusNoContent)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	resp.Body.Close()
}
