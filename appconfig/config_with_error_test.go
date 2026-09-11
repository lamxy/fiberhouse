// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package appconfig

import (
	"errors"
	"testing"
)

// 本文件锁定 appconfig 两处配置加载 panic 的替代入口（PRD C1）。
//
// LoadDefault / LoadYaml 此前唯一入口即 panic。配置加载失败是**可恢复**的：
// 文件不存在、权限不足、YAML 语法错误都可能发生，而调用方常有回退路径
// （改用默认配置、换一个配置目录）。

// TestLoadYamlWithError_ReportsMissingFile 文件不存在时返回错误。
func TestLoadYamlWithError_ReportsMissingFile(t *testing.T) {
	ac := NewAppConfig().SetConfPath(t.TempDir())

	cfg, err := ac.LoadYamlWithError("definitely-not-there.yml")

	if err == nil {
		t.Fatal("a missing config file must be reported")
	}
	if !errors.Is(err, ErrConfigLoadFailed) {
		t.Fatalf("error = %v, want ErrConfigLoadFailed in the chain", err)
	}
	if cfg != nil {
		t.Fatal("a failed load must return a nil config")
	}
}

// TestLoadYamlWithError_ErrorNamesTheFile 错误必须点名文件。
//
// 这是旧 panic 文本里最有用的一项信息，不能在改造中丢失。
func TestLoadYamlWithError_ErrorNamesTheFile(t *testing.T) {
	ac := NewAppConfig().SetConfPath(t.TempDir())

	_, err := ac.LoadYamlWithError("named-target.yml")

	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !contains(got, "named-target.yml") {
		t.Fatalf("error = %q, want it to name the file", got)
	}
}

// TestLoadYaml_KeepsPanicking 原入口保留 panic。
func TestLoadYaml_KeepsPanicking(t *testing.T) {
	ac := NewAppConfig().SetConfPath(t.TempDir())

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("LoadYaml must keep panicking on a missing file")
		}
	}()
	ac.LoadYaml("definitely-not-there.yml")
}

// TestLoadDefaultWithError_SucceedsAndChains 正常路径返回配置本身以支持链式调用。
func TestLoadDefaultWithError_SucceedsAndChains(t *testing.T) {
	ac := NewAppConfig()

	cfg, err := ac.LoadDefaultWithError(map[string]interface{}{"probe.key": "value"})

	if err != nil {
		t.Fatalf("LoadDefaultWithError() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("a successful load must return the config for chaining")
	}
	if got := cfg.String("probe.key"); got != "value" {
		t.Fatalf("probe.key = %q, want \"value\"", got)
	}
}

// TestLoadDefaultWithError_AgreesWithLoadDefault 两个入口在成功路径上结论一致。
func TestLoadDefaultWithError_AgreesWithLoadDefault(t *testing.T) {
	values := map[string]interface{}{"agree.key": "same"}

	viaPanic := NewAppConfig().LoadDefault(values)
	viaError, err := NewAppConfig().LoadDefaultWithError(values)
	if err != nil {
		t.Fatalf("LoadDefaultWithError() error = %v", err)
	}

	if viaPanic.String("agree.key") != viaError.String("agree.key") {
		t.Fatal("the pair must produce the same configuration")
	}
}

// TestErrConfigLoadFailed_IsClassifiable 哨兵必须可分类。
func TestErrConfigLoadFailed_IsClassifiable(t *testing.T) {
	if ErrConfigLoadFailed.Error() == "" {
		t.Fatal("the sentinel must carry a message")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
