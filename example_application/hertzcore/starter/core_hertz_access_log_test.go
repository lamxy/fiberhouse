package starter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	ctxpkg "context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
)

// Hertz 访问日志与框架 Fiber/Gin 同一规则：5xx、4xx 记 WARN，其余记 INFO。
func TestHertzAccessLogLevels(t *testing.T) {
	cfg := appconfig.NewAppConfig()
	cfg.LoadDefault(map[string]interface{}{"application.middleware.coreHttp": true})
	cfg.Initialize()
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)
	ctx := fiberhouse.NewAppContext(cfg, bootstrap.NewLoggerWrap(&logger))
	core := NewCoreWithHertz(ctx).(*CoreWithHertz)

	engine := route.NewEngine(config.NewOptions(nil))
	engine.Use(core.loggerMiddleware())
	engine.GET("/status/:code", func(_ ctxpkg.Context, c *app.RequestContext) {
		code, _ := strconv.Atoi(c.Param("code"))
		c.SetStatusCode(code)
	})

	for _, tc := range []struct {
		code  int
		level zerolog.Level
	}{
		{http.StatusInternalServerError, zerolog.WarnLevel},
		{http.StatusServiceUnavailable, zerolog.WarnLevel},
		{http.StatusNotFound, zerolog.WarnLevel},
		{http.StatusOK, zerolog.InfoLevel},
	} {
		output.Reset()
		ut.PerformRequest(engine, http.MethodGet, "/status/"+strconv.Itoa(tc.code), nil)
		var record map[string]any
		require.NoError(t, json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record), "status %d: %s", tc.code, output.String())
		assert.Equal(t, "HTTP Request", record["message"], "status %d", tc.code)
		assert.Equal(t, tc.level.String(), record["level"], "status %d", tc.code)
	}
}
