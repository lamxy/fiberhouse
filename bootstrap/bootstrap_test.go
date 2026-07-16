package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

const devYAML = `
application:
  env: yaml-env
  appName: yaml-dev
  appType: web
  fileMarker: application-dev
  appLog:
    enableConsole: false
    enableFile: true
    filename: test.log
    consoleJSON: false
    level: debug
    rollConf:
      maxSize: 10
      maxBackups: 3
      maxAge: 7
      compress: false
    asyncConf:
      enable: false
      type: diode
      chanConf:
        bufferSize: 256
        chanSize: 128
      diodeConf:
        size: 128
        bufferSize: 256
        flushInterval: 60000
`

type bootstrapTestState struct {
	loggerClosed bool
}

// isolateBootstrapGlobals keeps tests serial and resets package singletons only
// after the test has closed any logger it created.
func isolateBootstrapGlobals(t *testing.T) *bootstrapTestState {
	t.Helper()

	state := &bootstrapTestState{}
	t.Cleanup(func() {
		if Logger != nil && !state.loggerClosed {
			if err := Logger.Close(); err != nil {
				t.Errorf("close logger during cleanup: %v", err)
			}
		}

		cfgOnce = sync.Once{}
		AppConfigured = nil
		logOnce = sync.Once{}
		Logger = nil
	})
	return state
}

func (s *bootstrapTestState) closeLogger(t *testing.T) {
	t.Helper()
	if Logger == nil {
		t.Fatal("logger was not initialized")
	}
	err := Logger.Close()
	s.loggerClosed = true
	if err != nil {
		t.Fatalf("close logger: %v", err)
	}
}

func writeConfig(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write config %s: %v", name, err)
	}
}

func readFile(t *testing.T, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	return string(content)
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	value, present := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if present {
			if err := os.Setenv(key, value); err != nil {
				t.Errorf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Errorf("clear %s: %v", key, err)
		}
	})
}

func TestConfig_DefaultEnvironmentLoadsApplicationDev(t *testing.T) {
	isolateBootstrapGlobals(t)
	unsetenv(t, "APP_ENV_application_env")
	unsetenv(t, "APP_CONF_application_appName")
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)

	cfg := NewConfigOnce(dir)

	if got := cfg.String("application.fileMarker"); got != "application-dev" {
		t.Fatalf("default environment loaded wrong file: got %q", got)
	}
	if got := cfg.String("application.appName"); got != "yaml-dev" {
		t.Fatalf("YAML value not loaded: got %q", got)
	}
	if got := cfg.String("application.env"); got != "dev" {
		t.Fatalf("selected environment was not written back: got %q", got)
	}
}

func TestConfig_EnvironmentSelectionAndOverrideOrder(t *testing.T) {
	isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	prodYAML := strings.ReplaceAll(devYAML, "yaml-dev", "yaml-prod")
	prodYAML = strings.ReplaceAll(prodYAML, "application-dev", "application-prod")
	writeConfig(t, dir, "application_prod.yml", prodYAML)
	t.Setenv("APP_ENV_application_env", "prod")
	t.Setenv("APP_CONF_application_appName", "conf-prod")

	cfg := NewConfigOnce(dir)

	if got := cfg.String("application.fileMarker"); got != "application-prod" {
		t.Fatalf("prod YAML value not loaded: got %q", got)
	}
	if got := cfg.String("application.env"); got != "prod" {
		t.Fatalf("selected environment was not written back: got %q", got)
	}
	if got := cfg.String("application.appName"); got != "conf-prod" {
		t.Fatalf("APP_CONF did not override YAML: got %q", got)
	}

	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_CONF_application_appName", "changed-after-initialize")
	if same := NewConfigOnce(dir); same != cfg {
		t.Fatal("NewConfigOnce did not return the initialized singleton")
	}
	if got := cfg.String("application.appName"); got != "conf-prod" {
		t.Fatalf("singleton changed after environment mutation: got %q", got)
	}
}

func TestConfig_AppTypeDoesNotChangeFilename(t *testing.T) {
	isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	writeConfig(t, dir, "application_cmd_dev.yml", strings.ReplaceAll(devYAML, "application-dev", "obsolete-app-type-file"))
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "cmd")

	cfg := NewConfigOnce(dir)

	if got := cfg.String("application.fileMarker"); got != "application-dev" {
		t.Fatalf("appType changed the selected filename: got marker %q", got)
	}
}

func TestConfig_EnvironmentValueSplitsOnSpaces(t *testing.T) {
	isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_CONF_custom_values", "a b c")

	got := NewConfigOnce(dir).Strings("custom.values")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("split values: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("split values: got %v, want %v", got, want)
		}
	}
}

func TestLogger_FileSyncHonorsLevel(t *testing.T) {
	state := isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_CONF_application_appLog_level", "error")

	cfg := NewConfigOnce(dir)
	logger := NewLoggerOnce(cfg, dir)
	logger.Debug().Msg("debug suppressed")
	logger.Error().Msg("error emitted")
	state.closeLogger(t)

	content := readFile(t, filepath.Join(dir, "test.log"))
	if strings.Contains(content, "debug suppressed") {
		t.Fatal("debug message was not filtered")
	}
	if !strings.Contains(content, "error emitted") {
		t.Fatal("error message was not written")
	}
	if got := logger.GetLevel(); got != zerolog.ErrorLevel {
		t.Fatalf("logger level: got %s, want %s", got, zerolog.ErrorLevel)
	}
}

func TestLogger_InvalidLevelFallsBackToTrace(t *testing.T) {
	isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_CONF_application_appLog_level", "not-a-level")

	logger := NewLoggerOnce(NewConfigOnce(dir), dir)
	if got := logger.GetLevel(); got != zerolog.TraceLevel {
		t.Fatalf("fallback level: got %s, want %s", got, zerolog.TraceLevel)
	}
}

func TestLogger_NewLoggerOnceIsConcurrentSingleton(t *testing.T) {
	isolateBootstrapGlobals(t)
	dir := t.TempDir()
	writeConfig(t, dir, "application_dev.yml", devYAML)
	t.Setenv("APP_ENV_application_env", "dev")
	cfg := NewConfigOnce(dir)

	const callers = 30
	results := make(chan LoggerWrapper, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- NewLoggerOnce(cfg, dir)
		}()
	}
	wg.Wait()
	close(results)

	var first LoggerWrapper
	for logger := range results {
		if first == nil {
			first = logger
			continue
		}
		if logger != first {
			t.Fatal("NewLoggerOnce returned different logger instances")
		}
	}
}
