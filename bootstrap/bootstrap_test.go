// go:build test
package bootstrap

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"os"
)

// resetBootstrap 重置单例（仅测试使用）
func resetBootstrap() {
	AppConfigured = nil
	Logger = nil
	cfgOnce = sync.Once{}
	logOnce = sync.Once{}
}

const baseYml = `
application:
  env: dev
  appType: web
  appLog:
    enableConsole: false
    enableFile: true
    filename: test.log
    consoleJSON: false
    asyncConf:
      enable: false
      type: diode
    level: debug
`

func writeConfig(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func readFile(t *testing.T, file string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(b)
}

func Test_Config_EnvOverrideAndSingleton(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)

	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	t.Setenv("APP_CONF_application_appLog_level", "error")

	cfg1 := NewConfigOnce(tmp)
	if lvl := cfg1.String("application.appLog.level", ""); lvl != "error" {
		t.Fatalf("expect env override=error got=%s", lvl)
	}
	// 修改环境，再次调用不应改变
	t.Setenv("APP_CONF_application_appLog_level", "panic")
	cfg2 := NewConfigOnce(tmp)
	if cfg1 != cfg2 {
		t.Fatal("config singleton broken")
	}
	if lvl := cfg2.String("application.appLog.level", ""); lvl != "error" {
		t.Fatalf("singleton not kept, got=%s", lvl)
	}
}

func Test_Config_DefaultEnv(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	cfg := NewConfigOnce(tmp)
	if env := cfg.String("application.env", ""); env != "dev" {
		t.Fatalf("expect default env=dev got=%s", env)
	}
}

func Test_Config_AppTypeCmdSelection(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	// appType=cmd => 文件名 application_cmd_dev.yml
	cmdYml := strings.Replace(baseYml, "appType: web", "appType: cmd", 1)
	writeConfig(t, tmp, "application_cmd_dev.yml", cmdYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "cmd")
	cfg := NewConfigOnce(tmp)
	if at := cfg.String("application.appType", ""); at != "cmd" {
		t.Fatalf("expect appType=cmd got=%s", at)
	}
}

func Test_Config_EnvArraySplit(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	t.Setenv("APP_CONF_custom_values", "a b c")
	cfg := NewConfigOnce(tmp)
	arr := cfg.Strings("custom.values", nil)
	if len(arr) != 3 || arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Fatalf("expect [a b c] got=%v", arr)
	}
}

func Test_Logger_FileSync_LevelFilter(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	t.Setenv("APP_CONF_application_appLog_level", "error")

	cfg := NewConfigOnce(tmp)
	logDir := filepath.Join(tmp, "logs")
	logger := NewLoggerOnce(cfg, logDir)
	logger.Debug().Msg("debug suppressed")
	logger.Error().Msg("error occurred")
	_ = logger.Close()

	logFile := filepath.Join(logDir, cfg.String("application.appLog.filename", "app.log"))
	content := readFile(t, logFile)
	if strings.Contains(content, "debug suppressed") {
		t.Fatal("debug should be filtered out")
	}
	if !strings.Contains(content, "error occurred") {
		t.Fatal("error log missing")
	}
	if logger.GetLevel() != zerolog.ErrorLevel {
		t.Fatalf("expect level=error got=%s", logger.GetLevel())
	}
}

func Test_Logger_InvalidLevelFallback(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	t.Setenv("APP_CONF_application_appLog_level", "NOT_A_LEVEL")

	cfg := NewConfigOnce(tmp)
	logger := NewLoggerOnce(cfg, tmp)
	defer logger.Close()
	// 应回退到 info
	if logger.GetLevel() != zerolog.TraceLevel {
		t.Fatalf("invalid level fallback failed, got=%s", logger.GetLevel())
	}
}

func Test_Logger_Singleton(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	cfg := NewConfigOnce(tmp)

	l1 := NewLoggerOnce(cfg, tmp)
	l2 := NewLoggerOnce(cfg, tmp)

	if l1 != l2 {
		t.Fatal("logger singleton broken")
	}

	if l1.GetZeroLogger() != l2.GetZeroLogger() {
		t.Fatal("logger singleton broken")
	}
	_ = l1.Close()
}

func Test_Logger_AsyncChan(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	asyncYml := strings.ReplaceAll(baseYml, "enable: false", "enable: true")
	asyncYml = strings.ReplaceAll(asyncYml, "type: diode", "type: chan")
	writeConfig(t, tmp, "application_web_dev.yml", asyncYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")

	cfg := NewConfigOnce(tmp)
	logger := NewLoggerOnce(cfg, tmp)
	for i := 0; i < 5; i++ {
		logger.Info().Int("i", i).Msg("async-chan")
	}
	time.Sleep(150 * time.Millisecond)
	_ = logger.Close()

	logFile := filepath.Join(tmp, cfg.String("application.appLog.filename", "app.log"))
	content := readFile(t, logFile)
	if !strings.Contains(content, "async-chan") {
		t.Fatal("async chan writer not flushed")
	}
}

func Test_Logger_AsyncDiode(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	// 保持 type: diode
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	t.Setenv("APP_CONF_application_appLog_asyncConf_enable", "true")
	// diode 依赖必要配置，否则报错
	t.Setenv("APP_CONF_application_appLog_asyncConf_diodeConf_bufferSize", strconv.Itoa(4096))
	t.Setenv("APP_CONF_application_appLog_asyncConf_diodeConf_size", strconv.Itoa(33554432))

	cfg := NewConfigOnce(tmp)

	logger := NewLoggerOnce(cfg, tmp)
	for i := 0; i < 5; i++ {
		logger.Info().Int("i", i).Msg("async-diode")
	}
	time.Sleep(1000 * time.Millisecond)
	_ = logger.Close()

	logFile := filepath.Join(tmp, cfg.String("application.appLog.filename", "test.log"))
	content := readFile(t, logFile)
	if !strings.Contains(content, "async-diode") {
		t.Fatal("async diode writer not flushed")
	}
}

func Test_Logger_ConcurrentSafety(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")

	t.Setenv("APP_CONF_application_appLog_level", strconv.Itoa(2))
	cfg := NewConfigOnce(tmp)

	const n = 30
	ch := make(chan zerolog.Level, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := NewLoggerOnce(cfg, tmp)
			ch <- l.GetZeroLogger().GetLevel()
		}()
	}
	wg.Wait()
	close(ch)

	counts := make(map[zerolog.Level]int)
	for lvl := range ch {
		counts[lvl]++
	}

	if len(counts) != 1 {
		t.Fatalf("logger not singleton concurrently, seen levels: %v", counts)
	}

	if Logger != nil {
		_ = Logger.Close()
	}
}

func Test_Logger_CloseIdempotent(t *testing.T) {
	resetBootstrap()
	tmp := t.TempDir()
	writeConfig(t, tmp, "application_web_dev.yml", baseYml)
	t.Setenv("APP_ENV_application_env", "dev")
	t.Setenv("APP_ENV_application_appType", "web")
	cfg := NewConfigOnce(tmp)
	l := NewLoggerOnce(cfg, tmp)
	if err := l.Close(); err != nil {
		t.Fatalf("first close err: %v", err)
	}
	if err := l.Close(); err != nil { // 不应 panic
		t.Fatalf("second close err: %v", err)
	}
}
