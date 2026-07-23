package fiberhouse

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type coordinatedServerStarter struct {
	ApplicationStarter
	runStarted    chan struct{}
	runReleased   chan struct{}
	shutdownCalls int
}

func (s *coordinatedServerStarter) AppCoreRun(...IProviderManager) error {
	if s.runStarted != nil {
		close(s.runStarted)
	}
	if s.runReleased != nil {
		<-s.runReleased
	}
	return nil
}

func (s *coordinatedServerStarter) Shutdown(...IProviderManager) error {
	s.shutdownCalls++
	if s.runReleased != nil {
		close(s.runReleased)
	}
	return nil
}

type lifecycleRecordingStarter struct {
	ApplicationStarter
	stages       []string
	managerCalls map[string][]IProviderManager
}

func (s *lifecycleRecordingStarter) record(stage string, managers []IProviderManager) {
	s.stages = append(s.stages, stage)
	if managers != nil {
		s.managerCalls[stage] = managers
	}
}

func (s *lifecycleRecordingStarter) RegisterToCtx(ApplicationStarter) {
	s.record("RegisterToCtx", nil)
}

func (s *lifecycleRecordingStarter) RegisterApplicationGlobals(managers ...IProviderManager) {
	s.record("RegisterApplicationGlobals", managers)
}

func (s *lifecycleRecordingStarter) InitCoreApp(_ FrameStarter, managers ...IProviderManager) {
	s.record("InitCoreApp", managers)
}

func (s *lifecycleRecordingStarter) GetFrameApp() FrameStarter { return s }

func (s *lifecycleRecordingStarter) RegisterAppHooks(_ FrameStarter, managers ...IProviderManager) {
	s.record("RegisterAppHooks", managers)
}

func (s *lifecycleRecordingStarter) RegisterAppMiddleware(_ FrameStarter, managers ...IProviderManager) {
	s.record("RegisterAppMiddleware", managers)
}

func (s *lifecycleRecordingStarter) RegisterModuleInitialize(_ FrameStarter, managers ...IProviderManager) {
	s.record("RegisterModuleInitialize", managers)
}

func (s *lifecycleRecordingStarter) RegisterModuleSwagger(_ FrameStarter, managers ...IProviderManager) {
	s.record("RegisterModuleSwagger", managers)
}

func (s *lifecycleRecordingStarter) RegisterTaskServer(managers ...IProviderManager) {
	s.record("RegisterTaskServer", managers)
}

func (s *lifecycleRecordingStarter) RegisterGlobalsKeepalive(managers ...IProviderManager) {
	s.record("RegisterGlobalsKeepalive", managers)
}

func (s *lifecycleRecordingStarter) AppCoreRun(managers ...IProviderManager) error {
	s.record("AppCoreRun", managers)
	return nil
}

func TestRunApplicationStarter_UsesDocumentedOrderAndSameManagers(t *testing.T) {
	recorder := &lifecycleRecordingStarter{
		managerCalls: make(map[string][]IProviderManager),
	}
	first := NewProviderManager(nil).SetName("first")
	second := NewProviderManager(nil).SetName("second")
	managers := []IProviderManager{first, second}

	RunApplicationStarter(recorder, managers...)

	assert.Equal(t, []string{
		"RegisterToCtx",
		"RegisterApplicationGlobals",
		"InitCoreApp",
		"RegisterAppHooks",
		"RegisterAppMiddleware",
		"RegisterModuleInitialize",
		"RegisterModuleSwagger",
		"RegisterTaskServer",
		"RegisterGlobalsKeepalive",
		"AppCoreRun",
	}, recorder.stages)
	for stage, got := range recorder.managerCalls {
		require.Len(t, got, len(managers), stage)
		for i := range managers {
			assert.Same(t, managers[i], got[i], "%s manager %d", stage, i)
		}
	}
}

func TestFiberHouse_FluentCollectionsPreserveAppendOrder(t *testing.T) {
	house := &FiberHouse{}
	frameFirst := func(FrameStarter) {}
	frameSecond := func(FrameStarter) {}
	coreFirst := func(CoreStarter) {}
	providerFirst := NewProvider().SetName("first")
	providerSecond := NewProvider().SetName("second")
	managerFirst := NewProviderManager(nil).SetName("first")
	managerSecond := NewProviderManager(nil).SetName("second")

	assert.Same(t, house, house.WithFrameStarterOptions(frameFirst).WithFrameStarterOptions(frameSecond))
	assert.Same(t, house, house.WithCoreStarterOptions(coreFirst))
	assert.Same(t, house, house.WithProviders(providerFirst).WithProviders(providerSecond))
	assert.Same(t, house, house.WithPManagers(managerFirst).WithPManagers(managerSecond))
	assert.Len(t, house.frameStarterOpts, 2)
	assert.Len(t, house.coreStarterOpts, 1)
	assert.Equal(t, []IProvider{providerFirst, providerSecond}, house.providers)
	assert.Equal(t, []IProviderManager{managerFirst, managerSecond}, house.managers)
}

func TestCoordinateServerRun_ReturnsWhenCoreReturnsNil(t *testing.T) {
	starter := &coordinatedServerStarter{}

	runErr, shutdownErr, shutdownRequested := coordinateServerRun(starter, nil, nil, make(chan os.Signal))

	require.NoError(t, runErr)
	require.NoError(t, shutdownErr)
	assert.False(t, shutdownRequested)
	assert.Zero(t, starter.shutdownCalls)
}

func TestCoordinateServerRun_SignalInvokesShutdown(t *testing.T) {
	starter := &coordinatedServerStarter{
		runStarted:  make(chan struct{}),
		runReleased: make(chan struct{}),
	}
	stopCh := make(chan os.Signal, 1)
	stopCh <- syscall.SIGTERM

	runErr, shutdownErr, shutdownRequested := coordinateServerRun(starter, nil, nil, stopCh)

	require.NoError(t, runErr)
	require.NoError(t, shutdownErr)
	assert.True(t, shutdownRequested)
	assert.Equal(t, 1, starter.shutdownCalls)
}
