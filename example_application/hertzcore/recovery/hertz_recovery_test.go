package recovery

import (
	"encoding/json"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonEncoder(v interface{}) ([]byte, error) { return json.Marshal(v) }

func TestHertzRecovery_GetParamsJson(t *testing.T) {
	reqCtx := app.NewContext(2)
	reqCtx.Params = param.Params{
		{Key: "id", Value: "42"},
		{Key: "name", Value: "hertz"},
	}

	raw := New(nil).GetParamsJson(hertzadaptor.WithHertzContext(reqCtx), nil, jsonEncoder, "trace-1")
	require.NotNil(t, raw)

	var got map[string]string
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]string{"id": "42", "name": "hertz"}, got)
}

func TestHertzRecovery_GetQueriesJson(t *testing.T) {
	reqCtx := app.NewContext(0)
	reqCtx.Request.SetRequestURI("/search?q=golang&page=2")

	raw := New(nil).GetQueriesJson(hertzadaptor.WithHertzContext(reqCtx), nil, jsonEncoder, "trace-2")
	require.NotNil(t, raw)

	var got map[string][]string
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, []string{"golang"}, got["q"])
	assert.Equal(t, []string{"2"}, got["page"])
}

// TestHertzRecovery_GetHeadersJsonMasksSensitiveValues 驗證敏感請求頭脫敏，
// 規則需與框架 Fiber/Gin 實作保持一致，避免憑證洩漏進日誌。
func TestHertzRecovery_GetHeadersJsonMasksSensitiveValues(t *testing.T) {
	reqCtx := app.NewContext(0)
	reqCtx.Request.Header.Set("Authorization", "Bearer super-secret-token-value")
	reqCtx.Request.Header.Set("X-Api-Key", "short")
	reqCtx.Request.Header.Set("X-Trace", "plain-value")

	raw := New(nil).GetHeadersJson(hertzadaptor.WithHertzContext(reqCtx), nil, jsonEncoder, "trace-3")
	require.NotNil(t, raw)

	var got map[string][]string
	require.NoError(t, json.Unmarshal(raw, &got))

	// 長度 > 8 的敏感值保留前 4 碼
	assert.Equal(t, []string{"Bear...***"}, got["Authorization"])
	// 長度 <= 8 的敏感值全遮蔽
	assert.Equal(t, []string{"***"}, got["X-Api-Key"])
	// 非敏感頭原樣保留
	assert.Equal(t, []string{"plain-value"}, got["X-Trace"])
}

func TestHertzRecovery_GetHeader(t *testing.T) {
	reqCtx := app.NewContext(0)
	reqCtx.Request.Header.Set("X-Custom", "custom-value")

	r := New(nil)
	assert.Equal(t, "custom-value", r.GetHeader(hertzadaptor.WithHertzContext(reqCtx), "X-Custom"))
	assert.Equal(t, "", r.GetHeader(hertzadaptor.WithHertzContext(reqCtx), "X-Absent"))
}

// TestHertzRecovery_TraceIDReadsFromContextKey 驗證 traceId 從 hertz 上下文鍵讀取，
// 且缺失時回傳空字串而非 panic。
func TestHertzRecovery_TraceIDReadsFromContextKey(t *testing.T) {
	reqCtx := app.NewContext(0)
	reqCtx.Set("traceId", "trace-abc")

	r := New(nil)
	assert.Equal(t, "trace-abc", r.TraceID(hertzadaptor.WithHertzContext(reqCtx)))

	empty := app.NewContext(0)
	assert.Equal(t, "", r.TraceID(hertzadaptor.WithHertzContext(empty)))
}

// TestHertzRecovery_WrongContextTypeIsTolerated 驗證傳入非 hertz 上下文時安全降級，
// 對齊框架 Fiber/Gin 實作的容錯行為，避免恢復流程本身再次 panic。
func TestHertzRecovery_WrongContextTypeIsTolerated(t *testing.T) {
	r := New(nil)
	foreign := foreignContext{}

	assert.Nil(t, r.GetParamsJson(foreign, nil, jsonEncoder, "t"))
	assert.Nil(t, r.GetQueriesJson(foreign, nil, jsonEncoder, "t"))
	assert.Nil(t, r.GetHeadersJson(foreign, nil, jsonEncoder, "t"))
	assert.Equal(t, "", r.GetHeader(foreign, "X-Any"))
}

// foreignContext 模擬非 hertz 的核心上下文實作
type foreignContext struct{}

func (foreignContext) GetCtx() interface{}                     { return struct{}{} }
func (foreignContext) GetHeader(string) string                 { return "" }
func (foreignContext) SetHeader(string, string)                {}
func (foreignContext) JSON(int, interface{}) error             { return nil }
func (foreignContext) Send(int, []byte) error                  { return nil }
