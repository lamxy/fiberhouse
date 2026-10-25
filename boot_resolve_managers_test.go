package fiberhouse

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 示例应用的装配方式：WithPManagers 传入的默认集合里已有一个 DefaultPManager，
// RunServer 不带参数时 resolveManagerWithProviders 又新建一个。两个都没有提供者，
// 「没有提供者可加载」是正常状态，不应记 ERROR（此前启动时固定出现两条）。
func TestResolveManagerWithProviders_EmptyManagersAreNotErrors(t *testing.T) {
	ctx, output := newTask4LoggingAppContext(t, nil)
	house := &FiberHouse{AppCtx: ctx}
	house.WithPManagers(NewDefaultPManager(ctx))

	house.resolveManagerWithProviders()

	assert.NotContains(t, output.String(), "no providers registered", "logs: %s", output.String())
	assert.Equal(t, 0, bytes.Count(output.Bytes(), []byte(`"level":"error"`)), "logs: %s", output.String())
}
