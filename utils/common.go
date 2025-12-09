// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package utils

import (
	"github.com/tidwall/gjson"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"
	"unsafe"
)

// GetExecPath 获取当前可执行文件执行时目录
func GetExecPath() string {
	dir, err := os.Executable()
	if err != nil {
		panic("GetExecPath Error: " + err.Error())
	}
	return filepath.Dir(dir)
}

// GetWD 获取当前工作目录
func GetWD() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("GetWD Error: " + err.Error())
	}
	return dir
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
	if len(isZero) > 0 {
		if isZero[0] {
			if v.IsZero() || v.IsNil() {
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
