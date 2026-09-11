// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package option

import (
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse"
)

// 本文件锁定 option 包四处装配 panic 的替代入口（PRD C1）。
//
// 四个 With*Register 在注册器未实现目标接口时 panic，且 panic 发生在**选项
// 执行期**——即启动链深处，panic 信息不含调用现场。姊妹入口把校验提前到
// 选项构造期，调用方因此能在装配开始前就拿到错误。

// bareRegister 只实现 IRegister，不实现任何目标接口。
type bareRegister struct{ name string }

func (r *bareRegister) GetName() string     { return r.name }
func (r *bareRegister) SetName(name string) { r.name = name }

// TestWithAppRegisterWithError_ReportsTypeMismatch 类型不符时返回错误。
func TestWithAppRegisterWithError_ReportsTypeMismatch(t *testing.T) {
	opt, err := WithAppRegisterWithError(&bareRegister{name: "bare"})

	if err == nil {
		t.Fatal("WithAppRegisterWithError() error = nil, want a type mismatch error")
	}
	if !errors.Is(err, ErrRegisterTypeMismatch) {
		t.Fatalf("error = %v, want ErrRegisterTypeMismatch in the chain", err)
	}
	if opt != nil {
		t.Fatal("a rejected option must be nil")
	}
}

// TestWithModuleRegisterWithError_ReportsTypeMismatch 同上。
func TestWithModuleRegisterWithError_ReportsTypeMismatch(t *testing.T) {
	_, err := WithModuleRegisterWithError(&bareRegister{name: "bare"})

	if !errors.Is(err, ErrRegisterTypeMismatch) {
		t.Fatalf("error = %v, want ErrRegisterTypeMismatch", err)
	}
}

// TestWithTaskRegisterWithError_ReportsTypeMismatch 同上。
func TestWithTaskRegisterWithError_ReportsTypeMismatch(t *testing.T) {
	_, err := WithTaskRegisterWithError(&bareRegister{name: "bare"})

	if !errors.Is(err, ErrRegisterTypeMismatch) {
		t.Fatalf("error = %v, want ErrRegisterTypeMismatch", err)
	}
}

// TestWithAppRegister_KeepsPanicking 原入口保留 panic。
//
// 条件 4 要求的是「成对出现」：panic 入口可以存在，但必须同时有返回 error 的
// 替代入口。依赖 fail-fast 的调用方不应被静默改变行为。
func TestWithAppRegister_KeepsPanicking(t *testing.T) {
	for name, call := range map[string]func(){
		"app":    func() { WithAppRegister(&bareRegister{name: "bare"}) },
		"module": func() { WithModuleRegister(&bareRegister{name: "bare"}) },
		"task":   func() { WithTaskRegister(&bareRegister{name: "bare"}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("the original entry point must keep panicking on a type mismatch")
				}
			}()
			call()
		})
	}
}

// TestWithCmdRegisterWithError_RejectsNil nil 注册器返回错误。
func TestWithCmdRegisterWithError_RejectsNil(t *testing.T) {
	var nilRegister fiberhouse.ApplicationCmdRegister

	_, err := WithCmdRegisterWithError(nilRegister)

	if !errors.Is(err, ErrRegisterTypeMismatch) {
		t.Fatalf("error = %v, want ErrRegisterTypeMismatch", err)
	}
}

// TestWithCmdRegister_KeepsPanickingOnNil 原入口对 nil 保留 panic。
func TestWithCmdRegister_KeepsPanickingOnNil(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("WithCmdRegister must keep panicking on a nil register")
		}
	}()
	var nilRegister fiberhouse.ApplicationCmdRegister
	WithCmdRegister(nilRegister)
}

// TestErrRegisterTypeMismatch_IsClassifiable 哨兵必须非空且可分类。
func TestErrRegisterTypeMismatch_IsClassifiable(t *testing.T) {
	if ErrRegisterTypeMismatch.Error() == "" {
		t.Fatal("the sentinel must carry a message")
	}
}
