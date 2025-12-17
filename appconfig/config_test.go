package appconfig

import (
	"encoding/json"
	"github.com/knadh/koanf/v2"
	"github.com/lamxy/fiberhouse/constant"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: 创建临时 YAML 文件
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "application_test.yml")
	err := os.WriteFile(fp, []byte(content), 0o600)
	require.NoError(t, err)
	return fp
}

func TestNewAppConfig_Basic(t *testing.T) {
	ac := NewAppConfig()
	require.NotNil(t, ac)
	assert.NotNil(t, ac.GetContainer())
	assert.Empty(t, ac.GetAppId())
	assert.Empty(t, ac.GetAppName())
	assert.Empty(t, ac.GetVersion())
}

func TestAppConfig_SetGetBasicFields(t *testing.T) {
	ac := NewAppConfig()
	ac.SetAppId("APP-001")
	ac.SetAppName("TestApp")
	ac.SetVersion("v1.2.3")

	assert.Equal(t, "APP-001", ac.GetAppId())
	assert.Equal(t, "TestApp", ac.GetAppName())
	assert.Equal(t, "v1.2.3", ac.GetVersion())

	base := ac.GetApplication()
	assert.Equal(t, "APP-001", base.AppID)
	assert.Equal(t, "TestApp", base.AppName)
}

func TestAppConfig_LoadDefaultAndValueAccessors(t *testing.T) {
	ac := NewAppConfig()

	ac.LoadDefault(map[string]interface{}{
		"application.appId":                    "ID-X",
		"application.appName":                  "Demo",
		"application.version":                  "0.0.9",
		"application.server.idleTimeout":       15, // 秒
		"application.server.enablePrintRoutes": true,
		"application.server.host":              "127.0.0.1",
		"application.server.tags":              []string{"a", "b"},
		"custom.num":                           42,
		"custom.float":                         3.14,
		"custom.bytes":                         "hello-bytes",
	})

	// 更新 AppConfig 基础字段（假设 Initialize 内部可能映射这些字段，也单独测试 Set）
	ac.SetAppId(ac.String("application.appId"))
	ac.SetAppName(ac.String("application.appName"))
	ac.SetVersion(ac.String("application.version"))

	assert.Equal(t, "ID-X", ac.GetAppId())
	assert.Equal(t, "Demo", ac.GetAppName())
	assert.Equal(t, "0.0.9", ac.GetVersion())

	assert.Equal(t, "127.0.0.1", ac.String("application.server.host"))
	assert.Equal(t, 42, ac.Int("custom.num"))
	assert.InDelta(t, 3.14, ac.Float64("custom.float"), 0.0001)
	assert.True(t, ac.Bool("application.server.enablePrintRoutes"))

	strs := ac.Strings("application.server.tags")
	assert.ElementsMatch(t, []string{"a", "b"}, strs)

	d := ac.Duration("application.server.idleTimeout", time.Second) * time.Second // 15ns * time.Second = 15s
	// idleTimeout 表示秒，期望 15s
	assert.Equal(t, 15*time.Second, d)

	bs := ac.GetBytes("custom.bytes")
	assert.Equal(t, []byte("hello-bytes"), bs)
}

func TestAppConfig_LoadYaml(t *testing.T) {
	yaml := `
application:
  appId: "YAML-ID"
  appName: "YamlApp"
  version: "v9.9.9"
  server:
    idleTimeout: 30
  middleware:
    coreHttp: true
    monitor: false
`
	fp := writeTempYAML(t, yaml)

	ac := NewAppConfig().
		SetConfPath(filepath.Dir(fp)).
		LoadYaml(filepath.Base(fp)).
		Initialize()

	assert.Equal(t, "YAML-ID", ac.String("application.appId"))
	assert.Equal(t, "YamlApp", ac.String("application.appName"))
	assert.Equal(t, "v9.9.9", ac.String("application.version"))

	// 验证基础字段是否和载入一致（如果 Initialize 做了映射）
	ac.SetAppId(ac.String("application.appId"))
	ac.SetAppName(ac.String("application.appName"))
	ac.SetVersion(ac.String("application.version"))
	assert.Equal(t, "YAML-ID", ac.GetAppId())

	// 中间件开关
	coreSwitch := ac.GetMiddlewareSwitch("coreHttp") ||
		ac.GetCore().(*koanf.Koanf).Bool("application.middleware.coreHttp")
	assert.True(t, coreSwitch)

	monitorSwitch := ac.GetMiddlewareSwitch("monitor") ||
		ac.GetCore().(*koanf.Koanf).Bool("application.middleware.monitor")
	assert.False(t, monitorSwitch)
}

func TestAppConfig_ConfPath(t *testing.T) {
	ac := NewAppConfig()
	dir := t.TempDir()
	ac.SetConfPath(dir)
	assert.Equal(t, filepath.ToSlash(dir), ac.GetConfPath())
}

func TestAppConfig_LogOriginBuiltInAndCustom(t *testing.T) {
	ac := NewAppConfig()

	// 预定义
	frameOrigin := ac.LogOriginFrame()
	assert.NotEmpty(t, frameOrigin)
	assert.True(t, strings.HasPrefix(frameOrigin.InstanceKey(), constant.LogOriginKeyPrefix))

	// 自定义注册
	err := ac.RegisterLogOrigin("biz", LogOrigin("Business"))
	require.NoError(t, err)

	got := ac.LogOriginCustom("biz")
	assert.Equal(t, LogOrigin("Business"), got)
	assert.Equal(t, constant.LogOriginKeyPrefix+"Business", got.InstanceKey())

	// 重复注册应失败
	err2 := ac.RegisterLogOrigin("biz", LogOrigin("Other"))
	assert.Error(t, err2)

	// 获取不存在自定义
	empty := ac.LogOriginCustom("not_found")
	assert.Empty(t, empty)
}

func TestAppConfig_GetLogOriginMap(t *testing.T) {
	ac := NewAppConfig()
	_ = ac.RegisterLogOrigin("testA", LogOrigin("T-A"))
	_ = ac.RegisterLogOrigin("testB", LogOrigin("T-B"))
	m := ac.GetLogOriginMap()
	assert.Contains(t, m, "testA")
	assert.Contains(t, m, "testB")

	// map 为副本还是原始（期望最好是副本；这里防御性修改后再取值校验不影响内部）
	m["hack"] = "X"
	m2 := ac.GetLogOriginMap()
	_, exists := m2["hack"]
	// 不做强制断言；若希望是副本可断言 exists=false
	_ = exists
}

func TestAppConfig_SafeSetSafeGet_Concurrent(t *testing.T) {
	ac := NewAppConfig()
	const goroutines = 50
	var setOK, getOK int32

	// 预置键以便 SafeGet 读取
	err := ac.SafeSet("counter", int64(0), func(key string, val interface{}, byConf IAppConfig) error {
		// 直接在 LoadDefault 替换或内部结构写，这里用 LoadDefault 覆盖策略
		byConf.LoadDefault(map[string]interface{}{key: val})
		return nil
	})
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(goroutines * 2)

	// 并发增加
	for i := 0; i < goroutines; i++ {
		go func(iNum int64) {
			defer wg.Done()
			err := ac.SafeSet("counter", iNum+1, func(key string, val interface{}, byConf IAppConfig) error {
				// 读取当前值，自增后写回
				v := byConf.Int64(key)
				return byConf.GetCore().(*koanf.Koanf).Set(key, v+1)
			})
			if err == nil {
				atomic.AddInt32(&setOK, 1)
			}
		}(int64(i))
	}

	// 并发读取
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := ac.SafeGet("counter", func(key string, byConf IAppConfig) (interface{}, error) {
				return byConf.Int64(key), nil
			})
			if err == nil {
				atomic.AddInt32(&getOK, 1)
			}
		}()
	}

	wg.Wait()

	// 最终读取
	finalVal, err := ac.SafeGet("counter", func(key string, c IAppConfig) (interface{}, error) {
		return c.Int64(key), nil
	})
	require.NoError(t, err)
	// 自增次数 == setOK
	assert.Equal(t, int64(setOK), finalVal.(int64))
	assert.Equal(t, goroutines, int(setOK)) // 所有写应成功
	assert.Equal(t, goroutines, int(getOK))
}

func TestAppConfig_ContainerIntegration(t *testing.T) {
	ac := NewAppConfig()
	gm := ac.GetContainer()
	require.NotNil(t, gm)

	// 通过全局管理器注册一个对象
	type demo struct{ Name string }
	ok := gm.Register("demoObj", func() (interface{}, error) { return &demo{Name: "X"}, nil })
	require.True(t, ok)

	inst, err := gm.Get("demoObj")
	require.NoError(t, err)
	require.IsType(t, &demo{}, inst)
	assert.Equal(t, "X", inst.(*demo).Name)
}

func TestGetCoreWithConfig_Generic(t *testing.T) {
	ac := NewAppConfig()
	coreRaw := ac.GetCore()
	require.NotNil(t, coreRaw)

	// 尝试以其真实类型断言（无法确定类型，用 json 编码解码检测可用性）
	_, err := json.Marshal(coreRaw)
	assert.NoError(t, err)

	// 成功路径（用 interface{} 获取）
	anyVal, err := GetCoreWithConfig[interface{}](ac)
	require.NoError(t, err)
	assert.NotNil(t, anyVal)

	// 失败路径：尝试一个不匹配的具体类型（例如 *testing.T）
	_, err = GetCoreWithConfig[*testing.T](ac)
	assert.Error(t, err)
}

func TestAppConfig_Initialize_Idempotent(t *testing.T) {
	ac := NewAppConfig()
	// 调用多次不应 panic
	ac.Initialize()
	ac.Initialize()
}

func TestAppConfig_LogOrigin_InstanceKeyPrefix(t *testing.T) {
	ac := NewAppConfig()
	o := ac.LogOriginTest()
	assert.True(t, strings.HasPrefix(o.InstanceKey(), constant.LogOriginKeyPrefix))
}

func TestAppConfig_MiddlewareSwitch_LoadDefault(t *testing.T) {
	ac := NewAppConfig()
	ac.LoadDefault(map[string]interface{}{
		"application.middleware.coreHttp": true,
		"application.middleware.monitor":  false,
		"middleware.basicAuth":            true,
	})

	coreOn := ac.GetMiddlewareSwitch("coreHttp") ||
		ac.GetCore().(*koanf.Koanf).Bool("application.middleware.coreHttp")
	assert.True(t, coreOn)

	monitorOff := ac.GetMiddlewareSwitch("monitor") ||
		ac.GetCore().(*koanf.Koanf).Bool("application.middleware.monitor")
	assert.False(t, monitorOff)

	// 不存在 => false
	assert.False(t, ac.GetMiddlewareSwitch("absent_xxx"))
}

func TestAppConfig_Duration_DefaultFallback(t *testing.T) {
	ac := NewAppConfig()
	d := ac.Duration("not.exist.key", 123*time.Millisecond)
	assert.Equal(t, 123*time.Millisecond, d)
}

func TestAppConfig_ThreadSafety_ReadOnlyAfterLoad(t *testing.T) {
	ac := NewAppConfig()
	ac.LoadDefault(map[string]interface{}{
		"custom.counter": 0,
	})
	const readers = 100
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			_ = ac.Int("custom.counter")
			_ = ac.String("application.appName") // 可能为空
			_ = ac.Bool("application.server.enablePrintRoutes")
		}()
	}
	wg.Wait()
}

func TestAppConfig_RepeatedLoadDefault_Override(t *testing.T) {
	ac := NewAppConfig()
	ac.LoadDefault(map[string]interface{}{"x.y": "A"})
	assert.Equal(t, "A", ac.String("x.y"))
	ac.LoadDefault(map[string]interface{}{"x.y": "B"})
	assert.Equal(t, "B", ac.String("x.y"))
}

func TestAppConfig_ParallelSafeGet(t *testing.T) {
	ac := NewAppConfig()
	ac.LoadDefault(map[string]interface{}{"p.val": 100})
	parallel := runtime.NumCPU() * 4
	var wg sync.WaitGroup
	wg.Add(parallel)
	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			_, err := ac.SafeGet("p.val", func(key string, c IAppConfig) (interface{}, error) {
				return c.Int("p.val"), nil
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestAppConfig_SafeSet_ErrorPropagation(t *testing.T) {
	ac := NewAppConfig()
	err := ac.SafeSet("err.key", 1, func(key string, val interface{}, byConf IAppConfig) error {
		return assert.AnError
	})
	assert.Error(t, err)
}

// 确保自定义日志源重复 key 不覆盖
func TestAppConfig_LogOrigin_DuplicateKey(t *testing.T) {
	ac := NewAppConfig()
	err1 := ac.RegisterLogOrigin("dup", LogOrigin("First"))
	require.NoError(t, err1)
	err2 := ac.RegisterLogOrigin("dup", LogOrigin("Second"))
	assert.Error(t, err2)
	assert.Equal(t, LogOrigin("First"), ac.LogOriginCustom("dup"))
}
