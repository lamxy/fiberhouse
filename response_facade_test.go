package fiberhouse

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	adaptorctx "github.com/lamxy/fiberhouse/adaptor/context"
	responsepb "github.com/lamxy/fiberhouse/response/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

type task5HTTPResponse struct {
	status      int
	contentType string
	body        []byte
}

func runTask5ResponseRequest(t *testing.T, core, requestContentType, accept string, send func(adaptorctx.ICoreContext) error) task5HTTPResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/response", nil)
	if requestContentType != "" {
		request.Header.Set("Content-Type", requestContentType)
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}

	switch core {
	case "fiber":
		app := fiber.New()
		app.Get("/response", func(c *fiber.Ctx) error {
			return send(adaptorctx.WithFiberContext(c))
		})
		response, err := app.Test(request)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return task5HTTPResponse{status: response.StatusCode, contentType: response.Header.Get("Content-Type"), body: body}
	case "gin":
		preserveTask4GinMode(t)
		gin.SetMode(gin.TestMode)
		engine := gin.New()
		engine.GET("/response", func(c *gin.Context) {
			require.NoError(t, send(adaptorctx.WithGinContext(c)))
		})
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return task5HTTPResponse{status: recorder.Code, contentType: recorder.Header().Get("Content-Type"), body: recorder.Body.Bytes()}
	default:
		t.Fatalf("unknown core %q", core)
		return task5HTTPResponse{}
	}
}

func TestExtractPrimaryMimeType_FirstValueContract(t *testing.T) {
	for _, testCase := range []struct {
		accept string
		want   string
	}{
		{"", ""},
		{"application/json", "application/json"},
		{" application/msgpack; q=0.4, application/json; q=1.0", "application/msgpack"},
		{"*/*;q=0.8", "*/*"},
		{",application/json", ",application/json"},
	} {
		assert.Equal(t, testCase.want, extractPrimaryMimeType(testCase.accept))
	}
}

func TestResponseFacade_JSONFallbackBinaryDisabledAndCustomStatus(t *testing.T) {
	ctx := newTask5AppContext(t, false, false)
	installTask5ResponseManager(t, ctx)

	for _, core := range []string{"fiber", "gin"} {
		t.Run(core, func(t *testing.T) {
			var wrapper *ResponseWrap
			var inner interface{ GetCode() int }
			result := runTask5ResponseRequest(t, core, "application/msgpack", "", func(c adaptorctx.ICoreContext) error {
				wrapper = Response()
				inner = wrapper.IResponse
				wrapper.Reset(701, "json fallback", map[string]interface{}{"core": core})
				return wrapper.SendWithCtx(c, http.StatusCreated)
			})
			assert.Equal(t, http.StatusCreated, result.status)
			assert.Contains(t, result.contentType, "application/json")
			assert.JSONEq(t, `{"code":701,"msg":"json fallback","data":{"core":"`+core+`"}}`, string(result.body))
			assert.Nil(t, wrapper.IResponse, "wrapper must release its request-scoped response exactly once")
			assert.Zero(t, inner.GetCode(), "inner response must be reset when ownership is released")
		})
	}
}

func TestResponseFacade_BinaryNegotiationAndUnknownMimeFallback(t *testing.T) {
	ctx := newTask5AppContext(t, false, true)
	installTask5ResponseManager(t, ctx)

	tests := []struct {
		name               string
		core               string
		contentType        string
		accept             string
		wantResponseType   string
		assertBodyContract func(*testing.T, []byte)
	}{
		{
			name:             "fiber msgpack from first Accept value",
			core:             "fiber",
			accept:           "application/msgpack;q=0.1, application/json;q=1.0",
			wantResponseType: "application/msgpack",
			assertBodyContract: func(t *testing.T, body []byte) {
				var envelope map[string]interface{}
				require.NoError(t, msgpack.Unmarshal(body, &envelope))
				assert.EqualValues(t, 702, envelope["code"])
				assert.Equal(t, "binary", envelope["msg"])
			},
		},
		{
			name:             "gin protobuf selected by Content-Type before Accept",
			core:             "gin",
			contentType:      "application/x-protobuf",
			accept:           "application/msgpack",
			wantResponseType: "application/x-protobuf",
			assertBodyContract: func(t *testing.T, body []byte) {
				envelope := &responsepb.RespInfoProto{}
				require.NoError(t, proto.Unmarshal(body, envelope))
				assert.Equal(t, int32(702), envelope.Code)
				assert.Equal(t, "binary", envelope.Msg)
			},
		},
		{
			name:             "unknown mime falls back to JSON",
			core:             "fiber",
			accept:           "application/unknown",
			wantResponseType: "application/json",
			assertBodyContract: func(t *testing.T, body []byte) {
				var envelope map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &envelope))
				assert.EqualValues(t, 702, envelope["code"])
			},
		},
		{
			name:             "wildcard remains JSON",
			core:             "gin",
			accept:           "application/*;q=0.5",
			wantResponseType: "application/json",
			assertBodyContract: func(t *testing.T, body []byte) {
				var envelope map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &envelope))
				assert.EqualValues(t, 702, envelope["code"])
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := runTask5ResponseRequest(t, testCase.core, testCase.contentType, testCase.accept, func(c adaptorctx.ICoreContext) error {
				return Response().Reset(702, "binary", map[string]interface{}{"ok": true}).SendWithCtx(c, http.StatusAccepted)
			})
			assert.Equal(t, http.StatusAccepted, result.status)
			assert.Contains(t, result.contentType, testCase.wantResponseType)
			testCase.assertBodyContract(t, result.body)
		})
	}
}
