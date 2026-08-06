package fiberhouse

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLoaderManager 以最小实现模拟 IProviderManager 的位点分派行为，
// 仅覆写 LoadProviderManagersAtLocation 所依赖的方法。
type stubLoaderManager struct {
	IProviderManager
	location IProviderLocation
	ptype    IProviderType
	loadErr  error
	loaded   bool
	injected any
}

func (s *stubLoaderManager) Location() IProviderLocation { return s.location }
func (s *stubLoaderManager) Type() IProviderType         { return s.ptype }

func (s *stubLoaderManager) LoadProvider(loadFunc ...ProviderLoadFunc) (any, error) {
	s.loaded = true
	if len(loadFunc) > 0 {
		v, err := loadFunc[0](s)
		if err != nil {
			return nil, err
		}
		s.injected = v
	}
	return nil, s.loadErr
}

func TestLoadProviderManagersAtLocation_LoadsOnlyMatchingLocation(t *testing.T) {
	loc := ProviderLocationDefault().LocationCoreEngineInit
	normal := ProviderTypeDefault().GroupMiddlewareRegisterType

	match := &stubLoaderManager{location: loc, ptype: normal}
	skip := &stubLoaderManager{
		location: ProviderLocationDefault().LocationServerRun,
		ptype:    normal,
	}
	dep := "core-starter"

	handled, replaced, err := LoadProviderManagersAtLocation(
		[]IProviderManager{match, skip}, loc, dep)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.False(t, replaced)
	assert.True(t, match.loaded, "同位点管理器应被加载")
	assert.False(t, skip.loaded, "不同位点管理器不应被加载")
	assert.Equal(t, dep, match.injected, "应注入依赖实例")
}

// TestLoadProviderManagersAtLocation_ExtendReplaceTakesOver 验证扩展替代组会接管同位点的普通管理器，
// 这是框架允许外部扩展覆写默认行为的关键能力。
func TestLoadProviderManagersAtLocation_ExtendReplaceTakesOver(t *testing.T) {
	loc := ProviderLocationDefault().LocationCoreEngineInit
	normal := &stubLoaderManager{
		location: loc,
		ptype:    ProviderTypeDefault().GroupMiddlewareRegisterType,
	}
	replacement := &stubLoaderManager{
		location: loc,
		ptype:    ProviderTypeDefault().GroupExtendReplace,
	}

	handled, replaced, err := LoadProviderManagersAtLocation(
		[]IProviderManager{normal, replacement}, loc, nil)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.True(t, replaced)
	assert.True(t, replacement.loaded, "替代管理器应被加载")
	assert.False(t, normal.loaded, "被替代的普通管理器不应被加载")
}

func TestLoadProviderManagersAtLocation_NoMatchReturnsNotHandled(t *testing.T) {
	loc := ProviderLocationDefault().LocationCoreEngineInit
	other := &stubLoaderManager{
		location: ProviderLocationDefault().LocationServerRun,
		ptype:    ProviderTypeDefault().GroupMiddlewareRegisterType,
	}

	handled, replaced, err := LoadProviderManagersAtLocation(
		[]IProviderManager{other}, loc, nil)

	require.NoError(t, err)
	assert.False(t, handled)
	assert.False(t, replaced)
}

// TestLoadProviderManagersAtLocation_AggregatesErrors 验证多个管理器的错误被聚合而非只返回第一个
func TestLoadProviderManagersAtLocation_AggregatesErrors(t *testing.T) {
	loc := ProviderLocationDefault().LocationCoreEngineInit
	normal := ProviderTypeDefault().GroupMiddlewareRegisterType
	errA := errors.New("load failed A")
	errB := errors.New("load failed B")

	handled, _, err := LoadProviderManagersAtLocation([]IProviderManager{
		&stubLoaderManager{location: loc, ptype: normal, loadErr: errA},
		&stubLoaderManager{location: loc, ptype: normal, loadErr: errB},
	}, loc, nil)

	assert.True(t, handled)
	require.Error(t, err)
	assert.ErrorIs(t, err, errA)
	assert.ErrorIs(t, err, errB)
}

// TestLoadProviderManagersAtLocation_SkipsNilEntries 验证 nil 管理器不会造成 panic
func TestLoadProviderManagersAtLocation_SkipsNilEntries(t *testing.T) {
	handled, _, err := LoadProviderManagersAtLocation(
		[]IProviderManager{nil},
		ProviderLocationDefault().LocationCoreEngineInit,
		nil)

	require.NoError(t, err)
	assert.False(t, handled)
}
