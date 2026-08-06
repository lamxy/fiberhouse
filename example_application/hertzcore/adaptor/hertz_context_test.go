package adaptor

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHertzContext_RequestAndResponseContract(t *testing.T) {
	reqCtx := app.NewContext(0)
	reqCtx.Request.Header.Set("X-Request", "request-value")

	wrapped := WithHertzContext(reqCtx)
	assert.Same(t, reqCtx, wrapped.GetCtx())
	assert.Equal(t, "request-value", wrapped.GetHeader("X-Request"))

	wrapped.SetHeader("X-Response", "response-value")
	require.NoError(t, wrapped.JSON(consts.StatusCreated, map[string]string{"core": "hertz"}))

	assert.Equal(t, consts.StatusCreated, reqCtx.Response.StatusCode())
	assert.Equal(t, "response-value", reqCtx.Response.Header.Get("X-Response"))
	assert.JSONEq(t, `{"core":"hertz"}`, string(reqCtx.Response.Body()))
}

func TestHertzContext_SendWritesRawBytes(t *testing.T) {
	reqCtx := app.NewContext(0)
	wrapped := WithHertzContext(reqCtx)

	wrapped.SetHeader("Content-Type", "application/octet-stream")
	require.NoError(t, wrapped.Send(consts.StatusAccepted, []byte("raw-hertz")))

	assert.Equal(t, consts.StatusAccepted, reqCtx.Response.StatusCode())
	assert.Equal(t, "application/octet-stream", reqCtx.Response.Header.Get("Content-Type"))
	assert.Equal(t, "raw-hertz", string(reqCtx.Response.Body()))
}

// TestHertzContext_MissingHeaderReturnsEmpty 驗證缺失請求頭返回空字串，
// 因 hertz 的 GetHeader 返回 []byte(nil)，須確保轉換後不會產生非預期值。
func TestHertzContext_MissingHeaderReturnsEmpty(t *testing.T) {
	reqCtx := app.NewContext(0)
	assert.Equal(t, "", WithHertzContext(reqCtx).GetHeader("X-Absent"))
}

// TestHertzContext_ReleaseResetsPooledObject 驗證回收後不殘留上一個請求的上下文引用，
// 避免對象池復用時發生跨請求數據洩漏。
func TestHertzContext_ReleaseResetsPooledObject(t *testing.T) {
	reqCtx := app.NewContext(0)
	wrapped := WithHertzContext(reqCtx)

	releasable, ok := wrapped.(interface{ Release() })
	require.True(t, ok, "HertzContext 必須實作 Release() 以接入框架的對象池回收")
	releasable.Release()

	assert.Nil(t, wrapped.(*HertzContext).Ctx)
}
