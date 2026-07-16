package errorhandler

import (
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiberErrorHandler_UsesCallbackResultAndPreservesResponse(t *testing.T) {
	sentinel := errors.New("route failed")
	callback := func(ctx adaptorctx.ICoreContext, got error) error {
		assert.ErrorIs(t, got, sentinel)
		return ctx.JSON(fiber.StatusTeapot, fiber.Map{"handled": true})
	}
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler(callback)})
	app.Get("/", func(*fiber.Ctx) error { return sentinel })

	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusTeapot, response.StatusCode)
	assert.JSONEq(t, `{"handled":true}`, string(body))
}

func TestGinErrorHandler_MatchesFiberHandledResponse(t *testing.T) {
	oldMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(oldMode) })
	gin.SetMode(gin.TestMode)
	sentinel := errors.New("route failed")
	callback := func(ctx adaptorctx.ICoreContext, got error) error {
		assert.ErrorIs(t, got, sentinel)
		return ctx.JSON(fiber.StatusTeapot, gin.H{"handled": true})
	}
	engine := gin.New()
	engine.Use(GinErrorHandler(callback))
	engine.GET("/", func(c *gin.Context) { _ = c.Error(sentinel) })
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, fiber.StatusTeapot, recorder.Code)
	assert.JSONEq(t, `{"handled":true}`, recorder.Body.String())
}

func TestFiberErrorHandler_PropagatesCallbackError(t *testing.T) {
	sentinel := errors.New("route failed")
	callbackErr := errors.New("callback failed")
	handler := FiberErrorHandler(func(adaptorctx.ICoreContext, error) error { return callbackErr })
	var got error
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		got = handler(c, sentinel)
		return nil
	})

	_, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	assert.ErrorIs(t, got, callbackErr)
}
