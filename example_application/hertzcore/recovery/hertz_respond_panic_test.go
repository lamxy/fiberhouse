package recovery

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/lamxy/fiberhouse"
	"github.com/lamxy/fiberhouse/appconfig"
	"github.com/lamxy/fiberhouse/bootstrap"
	hertzadaptor "github.com/lamxy/fiberhouse/example_application/hertzcore/adaptor"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// 对齐框架 recover_internal.go：包装过的业务错误按业务错误响应 400，而不是落到 error 分支的 500。
func TestRespondPanic_WrappedBusinessErrorRespondsBadRequest(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int
	}{
		{"exception", exception.New(4101, "business rule"), http.StatusBadRequest},
		{"wrapped exception", fmt.Errorf("query: %w", exception.New(4101, "business rule")), http.StatusBadRequest},
		{"wrapped validate exception", fmt.Errorf("bind: %w", exception.NewVE(4201, "invalid input")), http.StatusBadRequest},
		{"plain error", fmt.Errorf("boom"), http.StatusInternalServerError},
	}
	// 统一响应依赖已挂载的响应信息管理器
	logger := zerolog.Nop()
	ctx := fiberhouse.NewAppContext(appconfig.NewAppConfig(), bootstrap.NewLoggerWrap(&logger))
	ctx.RegisterBootConfig(&fiberhouse.BootConfig{})
	fiberhouse.NewRespInfoPManager(ctx)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqCtx := app.NewContext(0)
			respondPanic(hertzadaptor.WithHertzContext(reqCtx), tc.value, false)
			assert.Equal(t, tc.want, reqCtx.Response.StatusCode())
		})
	}
}
