package fiberhouse

import (
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingProvider 记录自身被初始化的次数
type countingProvider struct {
	IProvider
	initCount *int
}

func newCountingProvider(name string, pType IProviderType, counter *int) *countingProvider {
	son := &countingProvider{
		IProvider: NewProvider().
			SetName(name).
			SetTarget(pprofTargetAny).
			SetType(pType),
		initCount: counter,
	}
	son.MountToParent(son)
	return son
}

func (p *countingProvider) Initialize(IContext, ...ProviderInitFunc) (any, error) {
	*p.initCount++
	return nil, nil
}

// countingPManager 将其内提供者全部初始化的管理器
type countingPManager struct {
	IProviderManager
}

func newCountingPManager(name string, pType IProviderType, location IProviderLocation) *countingPManager {
	son := &countingPManager{
		IProviderManager: NewProviderManager(nil).
			SetName(name).
			SetType(pType),
	}
	son.MountToParent(son)
	son.SetOrBindToLocation(location)
	return son
}

func (m *countingPManager) LoadProvider(...ProviderLoadFunc) (any, error) {
	return loadPprofProviders(m)
}

// TestServerRunBeforeExecutesAllManagers 验证 ServerRunBefore 位点执行全部管理器。
//
// 该位点此前只执行 IsUnique() 的管理器且执行首个后即 break，
// 导致非唯一管理器被静默跳过。修改后应通过统一位点分派执行全部管理器。
func TestServerRunBeforeExecutesAllManagers(t *testing.T) {
	registry := ProviderTypeGen()
	typeA := registry.MustCustom("TestRunBeforeTypeA")
	typeB := registry.MustCustom("TestRunBeforeTypeB")

	var countA, countB int

	location := ProviderLocationGen().MustCustom("TestRunBeforeLocation")

	mgrA := newCountingPManager("RunBeforeMgrA", typeA, location)
	mgrB := newCountingPManager("RunBeforeMgrB", typeB, location)

	require.NoError(t, mgrA.Register(newCountingProvider("provA", typeA, &countA)))
	require.NoError(t, mgrB.Register(newCountingProvider("provB", typeB, &countB)))

	// 两个管理器均非 unique 模式
	require.False(t, mgrA.IsUnique())
	require.False(t, mgrB.IsUnique())

	handled, replaced, err := LoadProviderManagersAtLocation(
		[]IProviderManager{mgrA, mgrB}, location, nil)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.False(t, replaced)
	assert.Equal(t, 1, countA, "非 unique 管理器 A 应被执行")
	assert.Equal(t, 1, countB, "非 unique 管理器 B 同样应被执行，不应被 break 跳过")
}

// TestPprofProviderRoutedToCorrectManager 验证提供者按类型被分派到各自管理器。
//
// 复现框架 resolveManagerWithProviders 的分派逻辑：按类型 ID 匹配、首个匹配即 break。
func TestPprofProviderRoutedToCorrectManager(t *testing.T) {
	ctx := newPprofTestContext(t)

	startMgr := NewPprofStartPManager(ctx)
	shutdownMgr := NewPprofShutdownPManager(ctx)
	managers := []IProviderManager{startMgr, shutdownMgr}

	providers := []IProvider{NewPprofStartProvider(), NewPprofShutdownProvider()}

	// 复现框架的分派逻辑
	for _, pdr := range providers {
		matched := false
		for _, mgr := range managers {
			if pdr.Type().GetTypeID() == mgr.Type().GetTypeID() {
				require.NoError(t, mgr.Register(pdr))
				matched = true
				break
			}
		}
		require.True(t, matched, "提供者 %s 应能匹配到管理器", pdr.Name())
	}

	startList := startMgr.List()
	shutdownList := shutdownMgr.List()

	require.Len(t, startList, 1)
	require.Len(t, shutdownList, 1)
	assert.Equal(t, "PprofStartProvider", startList[0].Name(),
		"启动管理器只应持有启动提供者")
	assert.Equal(t, "PprofShutdownProvider", shutdownList[0].Name(),
		"关闭管理器只应持有关闭提供者")
}

// TestPprofDisabledProviderIsNoop 未启用时提供者初始化不应启动监听
func TestPprofDisabledProviderIsNoop(t *testing.T) {
	resetPprofServerInstance()
	t.Cleanup(resetPprofServerInstance)

	ctx := newPprofTestContext(t) // 未设置 application.pprof.enable，默认关闭

	provider := NewPprofStartProvider()
	_, err := provider.Initialize(ctx)
	require.NoError(t, err)

	server := resolvePprofServer(ctx)
	assert.False(t, server.cfg.Enable)
	assert.False(t, server.started, "未启用时不应标记为已启动")
}

// TestPprofEndToEndViaLocations 通过真实位点分派驱动 pprof 的启动与关闭全流程。
//
// 覆盖：启动位点拉起诊断服务 → 五类 profile 可访问 → 关闭位点释放端口。
func TestPprofEndToEndViaLocations(t *testing.T) {
	resetPprofServerInstance()
	t.Cleanup(resetPprofServerInstance)
	t.Cleanup(runtimeResetProfileRates)

	port := freePort(t)
	ctx := newPprofTestContext(t)
	ctx.GetConfig().LoadDefault(map[string]interface{}{
		"application.pprof.enable":        true,
		"application.pprof.host":          "127.0.0.1",
		"application.pprof.port":          port,
		"application.pprof.blockRate":     10000,
		"application.pprof.mutexFraction": 100,
	})

	startMgr := NewPprofStartPManager(ctx)
	shutdownMgr := NewPprofShutdownPManager(ctx)
	require.NoError(t, startMgr.Register(NewPprofStartProvider()))
	require.NoError(t, shutdownMgr.Register(NewPprofShutdownProvider()))

	all := []IProviderManager{startMgr, shutdownMgr}

	// 启动位点：拉起诊断服务
	handled, _, err := LoadProviderManagersAtLocation(
		all, ProviderLocationDefault().LocationServerRunBefore, nil)
	require.NoError(t, err)
	require.True(t, handled, "ServerRunBefore 位点应处理 pprof 启动管理器")

	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://127.0.0.1:" + strconv.Itoa(port) + defaultPprofPrefix

	// block 与 mutex 已配置采样率，此处应返回非空数据
	for _, p := range []string{"/heap", "/goroutine", "/block", "/mutex", "/allocs"} {
		resp, err := client.Get(base + p + "?debug=1")
		require.NoError(t, err, "profile %s 应可访问", p)
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, readErr)

		assert.Equal(t, http.StatusOK, resp.StatusCode, "profile %s", p)
		assert.NotEmpty(t, body, "profile %s 不应为空", p)
	}

	// 关闭位点：释放端口
	handled, _, err = LoadProviderManagersAtLocation(
		all, ProviderLocationDefault().LocationServerShutdownAfter, nil)
	require.NoError(t, err)
	require.True(t, handled, "ServerShutdownAfter 位点应处理 pprof 关闭管理器")

	_, err = client.Get(base + "/heap")
	assert.Error(t, err, "关闭后诊断端点应不可达")
}

// resetPprofServerInstance 复位进程内共享实例，避免测试间相互影响
func resetPprofServerInstance() {
	pprofServerMu.Lock()
	defer pprofServerMu.Unlock()
	pprofServerInstance = nil
}
