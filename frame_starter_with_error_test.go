// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本文件锁定 frame_starter_impl.go 构造子 panic 的替代入口（PRD C1）。
//
// NewFrameApplication 在未提供任何选项时 panic：没有选项就没有注册器，启动器
// 无事可做。这是装配期的用法错误，调用方应当能够处理。

// TestNewFrameApplicationWithError_ReportsMissingOptions 无选项时返回错误。
func TestNewFrameApplicationWithError_ReportsMissingOptions(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)

	starter, err := NewFrameApplicationWithError(ctx)

	require.Error(t, err, "an empty option list must be reported")
	assert.ErrorIs(t, err, ErrNoFrameStarterOption)
	assert.Nil(t, starter, "a rejected construction must return nil")
}

// TestNewFrameApplication_KeepsPanicking 原入口保留 panic。
func TestNewFrameApplication_KeepsPanicking(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)

	assert.Panics(t, func() { NewFrameApplication(ctx) },
		"the original entry point must keep its fail-fast behaviour")
}

// TestNewFrameApplicationWithError_BuildsWithOptions 有选项时正常构造并套用。
func TestNewFrameApplicationWithError_BuildsWithOptions(t *testing.T) {
	ctx := newTask4InternalAppContext(t, nil)
	applied := false

	starter, err := NewFrameApplicationWithError(ctx, func(FrameStarter) {
		applied = true
	})

	require.NoError(t, err)
	require.NotNil(t, starter)
	assert.True(t, applied, "the provided option must be applied")
	assert.Same(t, ctx, starter.GetContext())
}

// TestErrNoFrameStarterOption_IsClassifiable 哨兵必须可分类。
func TestErrNoFrameStarterOption_IsClassifiable(t *testing.T) {
	assert.NotEmpty(t, ErrNoFrameStarterOption.Error())
}
