package context

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingGinResponseWriter struct {
	gin.ResponseWriter
	err error
}

func (w *failingGinResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestGinContext_RequestAndResponseContract(t *testing.T) {
	oldMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(oldMode) })
	gin.SetMode(gin.TestMode)

	jsonRecorder := httptest.NewRecorder()
	jsonCtx, _ := gin.CreateTestContext(jsonRecorder)
	jsonCtx.Request = httptest.NewRequest("GET", "/json", nil)
	jsonCtx.Request.Header.Set("X-Request", "request-value")
	jsonWrapped := WithGinContext(jsonCtx)
	assert.Same(t, jsonCtx, jsonWrapped.GetCtx())
	assert.Equal(t, "request-value", jsonWrapped.GetHeader("X-Request"))
	jsonWrapped.SetHeader("X-Response", "response-value")
	require.NoError(t, jsonWrapped.JSON(201, gin.H{"core": "gin"}))
	assert.Equal(t, 201, jsonRecorder.Code)
	assert.Equal(t, "response-value", jsonRecorder.Header().Get("X-Response"))
	assert.JSONEq(t, `{"core":"gin"}`, jsonRecorder.Body.String())

	rawRecorder := httptest.NewRecorder()
	rawCtx, _ := gin.CreateTestContext(rawRecorder)
	rawCtx.Request = httptest.NewRequest("GET", "/raw", nil)
	rawWrapped := WithGinContext(rawCtx)
	rawWrapped.SetHeader("Content-Type", "application/octet-stream")
	require.NoError(t, rawWrapped.Send(202, []byte("raw-gin")))
	assert.Equal(t, 202, rawRecorder.Code)
	assert.Equal(t, "application/octet-stream", rawRecorder.Header().Get("Content-Type"))
	assert.Equal(t, "raw-gin", rawRecorder.Body.String())
}

func TestGinContext_SendPropagatesWriterError(t *testing.T) {
	oldMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(oldMode) })
	gin.SetMode(gin.TestMode)
	sentinel := errors.New("write failed")
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = httptest.NewRequest("GET", "/", nil)
	ginCtx.Writer = &failingGinResponseWriter{ResponseWriter: ginCtx.Writer, err: sentinel}

	err := WithGinContext(ginCtx).Send(503, []byte("unwritten"))
	assert.ErrorIs(t, err, sentinel)
}
