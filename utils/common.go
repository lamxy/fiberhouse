// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode"
	"unsafe"

	"github.com/tidwall/gjson"
)

// GetExecPath 获取当前可执行文件执行时目录
//
// 解析失败时 panic。需要处理该错误的调用方请改用 GetExecPathWithError。
func GetExecPath() string {
	dir, err := GetExecPathWithError()
	if err != nil {
		panic(err)
	}
	return dir
}

// GetExecPathWithError 与 GetExecPath 相同，但解析失败时返回错误而非 panic。
//
// os.Executable 在少数平台与容器环境下会失败（例如可执行文件被删除后重启）。
// 调用方通常有可用的回退路径——退回工作目录或配置指定的路径——因此这类失败
// 应当可被处理，而不是终止进程。
func GetExecPathWithError() (string, error) {
	dir, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Dir(dir), nil
}

// GetWD 获取当前工作目录
//
// 解析失败时 panic。需要处理该错误的调用方请改用 GetWDWithError。
func GetWD() string {
	dir, err := GetWDWithError()
	if err != nil {
		panic(err)
	}
	return dir
}

// GetWDWithError 与 GetWD 相同，但解析失败时返回错误而非 panic。
//
// os.Getwd 会在工作目录已被删除时失败，这在长期运行的进程里确实会发生。
func GetWDWithError() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return dir, nil
}

// JsonValidString 检查字符串是否时有效json
func JsonValidString(j string) bool {
	return gjson.Valid(j)
}

// JsonValidBytes 检查字节切片是否有效json
func JsonValidBytes(j []byte) bool {
	return gjson.ValidBytes(j)
}

// ValidConstant 检查常量是否有效，支持可选参数isZero，true表示检查是否为零值或nil，false表示只检查是否为nil
func ValidConstant(constName interface{}, isZero ...bool) bool {
	v := reflect.ValueOf(constName)
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		if v.IsNil() {
			return false
		}
	}
	if len(isZero) > 0 {
		if isZero[0] {
			if v.IsZero() {
				return false
			}
		}
	}
	return true
}

// NormalizeWhitespace 规范化字符串中的空白字符，将连续的空白字符替换为单个空格
func NormalizeWhitespace(s string) string {
	var builder strings.Builder
	inWhitespace := false

	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inWhitespace {
				// Write a single space for the first whitespace character encountered
				builder.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			builder.WriteRune(r)
			inWhitespace = false
		}
	}

	return builder.String()
}

// FileExists 检查文件是否存在
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

// UnsafeString returns a string pointer without allocation
//
// copy from goFiber/fiber utils
func UnsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// UnsafeBytes returns a byte pointer without allocation.
//
// copy from goFiber/fiber utils
func UnsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// StackMsg 获取当前 goroutine 的完整调用栈信息，需将字节切片转为字符串
func StackMsg() string {
	return UnsafeString(debug.Stack())
}

// CaptureStack 捕获当前 goroutine 的调用栈信息，跳过前3层调用栈
// 固定 64 个 uintptr 数组，栈上分配
func CaptureStack() string {
	const size = 64
	var pcs [size]uintptr
	n := runtime.Callers(3, pcs[:]) // skip跳过前3层
	frames := runtime.CallersFrames(pcs[:n])

	var strBuilder strings.Builder
	strBuilder.Grow(64 * 64) // 預估分配，減少擴容
	strBuilder.WriteString("stack trace:\n")

	for {
		frm, more := frames.Next()
		strBuilder.WriteString(frm.Function)
		strBuilder.WriteString("\n\t")
		strBuilder.WriteString(frm.File)
		strBuilder.WriteByte(':')
		strBuilder.WriteString(strconv.Itoa(frm.Line))
		strBuilder.WriteByte('\n')

		if !more {
			break
		}
	}
	return strBuilder.String()
}

// debugStackLines 获取当前 goroutine 的调用栈信息行切片
func DebugStackLines(stacks string) []string {
	if len(stacks) == 0 || !strings.Contains(stacks, "\n") {
		return nil
	}
	return strings.Split(strings.ReplaceAll(stacks, "\t", "    "), "\n")
}
