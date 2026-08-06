package starter

import (
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubManager 以最小實作模擬 IProviderManager 的位點分派行為。
// 僅覆寫 loadProviderManagersAtLocation 所依賴的方法，其餘委派給框架基類。
type stubManager struct {
	fiberhouse.IProviderManager
	location fiberhouse.IProviderLocation
	ptype    fiberhouse.IProviderType
	loadErr  error
	loaded   bool
	injected any
}

func (s *stubManager) Location() fiberhouse.IProviderLocation { return s.location }
func (s *stubManager) Type() fiberhouse.IProviderType         { return s.ptype }

func (s *stubManager) LoadProvider(loadFunc ...fiberhouse.ProviderLoadFunc) (any, error) {
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

func TestLoadManagersAtLocation_LoadsOnlyMatchingLocation(t *testing.T) {
	loc := fiberhouse.ProviderLocationDefault().LocationCoreEngineInit
	other := fiberhouse.ProviderLocationDefault().LocationServerRun
	normal := fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType

	match := &stubManager{location: loc, ptype: normal}
	skip := &stubManager{location: other, ptype: normal}
	dep := "core-starter"

	handled, replaced, err := loadManagersAtLocation(
		[]fiberhouse.IProviderManager{match, skip}, loc, dep)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.False(t, replaced)
	assert.True(t, match.loaded, "同位點管理器應被載入")
	assert.False(t, skip.loaded, "不同位點管理器不應被載入")
	assert.Equal(t, dep, match.injected, "應注入核心啟動器實例")
}

// TestLoadManagersAtLocation_ExtendReplaceTakesOver 驗證擴展替代組會接管同位點的普通管理器，
// 這是框架讓外部擴展覆寫預設行為的關鍵能力。
func TestLoadManagersAtLocation_ExtendReplaceTakesOver(t *testing.T) {
	loc := fiberhouse.ProviderLocationDefault().LocationCoreEngineInit
	normal := &stubManager{location: loc, ptype: fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType}
	replacement := &stubManager{location: loc, ptype: fiberhouse.ProviderTypeDefault().GroupExtendReplace}

	handled, replaced, err := loadManagersAtLocation(
		[]fiberhouse.IProviderManager{normal, replacement}, loc, nil)

	require.NoError(t, err)
	assert.True(t, handled)
	assert.True(t, replaced)
	assert.True(t, replacement.loaded, "替代管理器應被載入")
	assert.False(t, normal.loaded, "被替代的普通管理器不應被載入")
}

func TestLoadManagersAtLocation_NoMatchReturnsNotHandled(t *testing.T) {
	loc := fiberhouse.ProviderLocationDefault().LocationCoreEngineInit
	other := &stubManager{
		location: fiberhouse.ProviderLocationDefault().LocationServerRun,
		ptype:    fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType,
	}

	handled, replaced, err := loadManagersAtLocation(
		[]fiberhouse.IProviderManager{other}, loc, nil)

	require.NoError(t, err)
	assert.False(t, handled)
	assert.False(t, replaced)
}

// TestLoadManagersAtLocation_AggregatesErrors 驗證多個管理器的錯誤被聚合而非只回傳第一個，
// 對齊框架 errors.Join 的行為。
func TestLoadManagersAtLocation_AggregatesErrors(t *testing.T) {
	loc := fiberhouse.ProviderLocationDefault().LocationCoreEngineInit
	normal := fiberhouse.ProviderTypeDefault().GroupMiddlewareRegisterType
	errA := errors.New("load failed A")
	errB := errors.New("load failed B")

	handled, _, err := loadManagersAtLocation([]fiberhouse.IProviderManager{
		&stubManager{location: loc, ptype: normal, loadErr: errA},
		&stubManager{location: loc, ptype: normal, loadErr: errB},
	}, loc, nil)

	assert.True(t, handled)
	require.Error(t, err)
	assert.ErrorIs(t, err, errA)
	assert.ErrorIs(t, err, errB)
}

// TestLoadManagersAtLocation_SkipsNilEntries 驗證 nil 管理器不會造成 panic
func TestLoadManagersAtLocation_SkipsNilEntries(t *testing.T) {
	loc := fiberhouse.ProviderLocationDefault().LocationCoreEngineInit

	handled, _, err := loadManagersAtLocation(
		[]fiberhouse.IProviderManager{nil}, loc, nil)

	require.NoError(t, err)
	assert.False(t, handled)
}
