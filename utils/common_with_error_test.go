// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package utils

import (
	"path/filepath"
	"testing"
)

// 本文件锁定 utils/common.go 两处 panic 的替代入口（PRD C1）。
//
// GetExecPath / GetWD 此前唯一入口即 panic。两者的失败都是**可恢复**的：
// os.Executable 在可执行文件被删除后重启时会失败，os.Getwd 在工作目录被删除
// 时会失败，调用方通常有回退路径（退回配置指定的路径）。

// TestGetExecPathWithError_ReturnsAbsolutePath 正常路径返回绝对路径且无错误。
func TestGetExecPathWithError_ReturnsAbsolutePath(t *testing.T) {
	path, err := GetExecPathWithError()
	if err != nil {
		t.Fatalf("GetExecPathWithError() error = %v", err)
	}
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("GetExecPathWithError() = %q, want an absolute path", path)
	}
}

// TestGetExecPathWithError_AgreesWithGetExecPath 两个入口在成功路径上结论一致。
func TestGetExecPathWithError_AgreesWithGetExecPath(t *testing.T) {
	withError, err := GetExecPathWithError()
	if err != nil {
		t.Fatalf("GetExecPathWithError() error = %v", err)
	}
	if got := GetExecPath(); got != withError {
		t.Fatalf("GetExecPath() = %q, GetExecPathWithError() = %q; the pair must agree",
			got, withError)
	}
}

// TestGetWDWithError_ReturnsWorkingDirectory 正常路径返回工作目录。
func TestGetWDWithError_ReturnsWorkingDirectory(t *testing.T) {
	dir, err := GetWDWithError()
	if err != nil {
		t.Fatalf("GetWDWithError() error = %v", err)
	}
	if dir == "" {
		t.Fatal("GetWDWithError() returned an empty directory")
	}
}

// TestGetWDWithError_AgreesWithGetWD 两个入口在成功路径上结论一致。
func TestGetWDWithError_AgreesWithGetWD(t *testing.T) {
	withError, err := GetWDWithError()
	if err != nil {
		t.Fatalf("GetWDWithError() error = %v", err)
	}
	if got := GetWD(); got != withError {
		t.Fatalf("GetWD() = %q, GetWDWithError() = %q; the pair must agree", got, withError)
	}
}
