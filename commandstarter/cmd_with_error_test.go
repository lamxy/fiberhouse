// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package commandstarter

import (
	"errors"
	"testing"

	"github.com/lamxy/fiberhouse"
	"github.com/urfave/cli/v2"
)

// 本文件锁定 commandstarter 四处装配 panic 的替代入口（PRD C1）。
//
// CoreCmdCli 的三个 Register* 方法要求先 RegisterApplication，未注册时直接
// panic；NewFrameCmdApplication 在未提供任何选项时同样 panic。四处都是装配期
// 的顺序/用法错误，调用方应当能够处理。

// cmdStarterWithoutApplication 是未注册应用的 FrameCmdStarter 替身。
//
// 嵌入 FrameCmdStarter 以取得其余方法（GetApplication 来自 IStarter，
// 回传 IApplication），只覆写本用例关心的那一个。
type cmdStarterWithoutApplication struct {
	fiberhouse.FrameCmdStarter
	application fiberhouse.IApplication
}

func (s *cmdStarterWithoutApplication) GetApplication() fiberhouse.IApplication {
	return s.application
}

// wrongKindApplication 实现 IApplication 但不是 ApplicationCmdRegister。
//
// 嵌入 nil 的 IApplication 取得其 14 个方法的集合：本用例只需要它「是一个
// IApplication 却不是 ApplicationCmdRegister」，不会真的调用那些方法。
type wrongKindApplication struct {
	fiberhouse.IApplication
}

func newTestCoreCmdCli() *CoreCmdCli {
	return &CoreCmdCli{coreApp: cli.NewApp()}
}

// TestRegisterWithError_ReportsMissingApplication 三个 Register* 的姊妹入口在
// 未注册应用时都返回同一个可分类错误。
func TestRegisterWithError_ReportsMissingApplication(t *testing.T) {
	starter := &cmdStarterWithoutApplication{}

	for name, call := range map[string]func(*CoreCmdCli) error{
		"RegisterGlobalErrHandler": func(c *CoreCmdCli) error {
			return c.RegisterGlobalErrHandlerWithError(starter)
		},
		"RegisterCommands": func(c *CoreCmdCli) error {
			return c.RegisterCommandsWithError(starter)
		},
		"RegisterCoreGlobalOptional": func(c *CoreCmdCli) error {
			return c.RegisterCoreGlobalOptionalWithError(starter)
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call(newTestCoreCmdCli())
			if err == nil {
				t.Fatal("an unregistered application must be reported")
			}
			if !errors.Is(err, ErrCmdApplicationNotRegistered) {
				t.Fatalf("error = %v, want ErrCmdApplicationNotRegistered in the chain", err)
			}
		})
	}
}

// TestRegisterWithError_ReportsWrongRegisterKind 注册器类型不符同样返回错误。
//
// 原实现直接做类型断言，类型不符时 panic 信息不含任何上下文；姊妹入口补上了
// 这道检查并在错误里给出实际类型。
func TestRegisterWithError_ReportsWrongRegisterKind(t *testing.T) {
	starter := &cmdStarterWithoutApplication{application: &wrongKindApplication{}}

	err := newTestCoreCmdCli().RegisterCommandsWithError(starter)

	if err == nil {
		t.Fatal("a register of the wrong kind must be reported")
	}
	if !errors.Is(err, ErrCmdApplicationNotRegistered) {
		t.Fatalf("error = %v, want ErrCmdApplicationNotRegistered in the chain", err)
	}
}

// TestRegister_KeepsPanicking 三个原入口保留 panic。
func TestRegister_KeepsPanicking(t *testing.T) {
	starter := &cmdStarterWithoutApplication{}

	for name, call := range map[string]func(*CoreCmdCli){
		"RegisterGlobalErrHandler":   func(c *CoreCmdCli) { c.RegisterGlobalErrHandler(starter) },
		"RegisterCommands":           func(c *CoreCmdCli) { c.RegisterCommands(starter) },
		"RegisterCoreGlobalOptional": func(c *CoreCmdCli) { c.RegisterCoreGlobalOptional(starter) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered == nil {
					t.Fatal("the original entry point must keep panicking")
				}
			}()
			call(newTestCoreCmdCli())
		})
	}
}

// TestNewFrameCmdApplicationWithError_ReportsMissingOptions 无选项时返回错误。
func TestNewFrameCmdApplicationWithError_ReportsMissingOptions(t *testing.T) {
	starter, err := NewFrameCmdApplicationWithError(nil)

	if err == nil {
		t.Fatal("an empty option list must be reported")
	}
	if !errors.Is(err, ErrNoCmdStarterOption) {
		t.Fatalf("error = %v, want ErrNoCmdStarterOption", err)
	}
	if starter != nil {
		t.Fatal("a rejected construction must return nil")
	}
}

// TestNewFrameCmdApplication_KeepsPanicking 原入口保留 panic。
func TestNewFrameCmdApplication_KeepsPanicking(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("NewFrameCmdApplication must keep panicking without options")
		}
	}()
	NewFrameCmdApplication(nil)
}

// TestNewFrameCmdApplicationWithError_BuildsWithOptions 有选项时正常构造。
func TestNewFrameCmdApplicationWithError_BuildsWithOptions(t *testing.T) {
	applied := false
	starter, err := NewFrameCmdApplicationWithError(nil, func(fiberhouse.FrameCmdStarter) {
		applied = true
	})

	if err != nil {
		t.Fatalf("NewFrameCmdApplicationWithError() error = %v", err)
	}
	if starter == nil {
		t.Fatal("a successful construction must return a starter")
	}
	if !applied {
		t.Fatal("the provided option must be applied")
	}
}

// TestCmdSentinels_AreDistinct 两个哨兵必须可区分。
func TestCmdSentinels_AreDistinct(t *testing.T) {
	if errors.Is(ErrNoCmdStarterOption, ErrCmdApplicationNotRegistered) {
		t.Fatal("the two sentinels must not alias each other")
	}
	if ErrNoCmdStarterOption.Error() == "" || ErrCmdApplicationNotRegistered.Error() == "" {
		t.Fatal("both sentinels must carry a message")
	}
}
