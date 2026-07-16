package context

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiberContext_RequestAndResponseContract(t *testing.T) {
	app := fiber.New()
	app.Get("/json", func(c *fiber.Ctx) error {
		wrapped := WithFiberContext(c)
		assert.Same(t, c, wrapped.GetCtx())
		assert.Equal(t, "request-value", wrapped.GetHeader("X-Request"))
		wrapped.SetHeader("X-Response", "response-value")
		return wrapped.JSON(fiber.StatusCreated, fiber.Map{"core": "fiber"})
	})
	app.Get("/raw", func(c *fiber.Ctx) error {
		wrapped := WithFiberContext(c)
		wrapped.SetHeader("Content-Type", "application/octet-stream")
		return wrapped.Send(fiber.StatusAccepted, []byte("raw-fiber"))
	})

	jsonRequest := httptest.NewRequest("GET", "/json", nil)
	jsonRequest.Header.Set("X-Request", "request-value")
	jsonResponse, err := app.Test(jsonRequest)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusCreated, jsonResponse.StatusCode)
	assert.Equal(t, "response-value", jsonResponse.Header.Get("X-Response"))
	assert.JSONEq(t, `{"core":"fiber"}`, readFiberResponseBody(t, jsonResponse))

	rawResponse, err := app.Test(httptest.NewRequest("GET", "/raw", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusAccepted, rawResponse.StatusCode)
	assert.Equal(t, "application/octet-stream", rawResponse.Header.Get("Content-Type"))
	assert.Equal(t, "raw-fiber", readFiberResponseBody(t, rawResponse))
}

func TestFiberContext_JSONPropagatesEncoderError(t *testing.T) {
	sentinel := errors.New("encode failed")
	var got error
	app := fiber.New(fiber.Config{
		JSONEncoder: func(interface{}) ([]byte, error) { return nil, sentinel },
		ErrorHandler: func(*fiber.Ctx, error) error {
			return nil
		},
	})
	app.Get("/", func(c *fiber.Ctx) error {
		got = WithFiberContext(c).JSON(fiber.StatusServiceUnavailable, fiber.Map{"bad": true})
		return got
	})

	_, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	assert.ErrorIs(t, got, sentinel)
}

func readFiberResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(body)
}
