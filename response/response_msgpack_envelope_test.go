// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package response

import (
	"math"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

// parseMsgPackEnvelope 按 msgpack 解码出的具体整数类型分支处理 code 字段。
// msgpack 会按值大小选择最紧凑的编码，因此同一个逻辑整数在往返后可能是
// int8/uint8/int64/uint64 中的任意一种；这些分支此前几乎没有覆盖，
// 而它们正是"响应 code 被静默读错"的唯一防线。
//
// 这里直接构造带有确定 Go 类型的 envelope，绕过 msgpack 的紧凑编码选择，
// 从而逐个钉住 switch 的每条分支。
func TestParseMsgPackEnvelope_AcceptsEveryIntegerWidth(t *testing.T) {
	tests := []struct {
		name     string
		code     interface{}
		wantCode int
	}{
		{name: "int", code: int(700), wantCode: 700},
		{name: "int8", code: int8(-8), wantCode: -8},
		{name: "int16", code: int16(-1600), wantCode: -1600},
		{name: "int32", code: int32(-320000), wantCode: -320000},
		{name: "int64", code: int64(-6400000000), wantCode: -6400000000},
		{name: "uint", code: uint(701), wantCode: 701},
		{name: "uint8", code: uint8(8), wantCode: 8},
		{name: "uint16", code: uint16(1600), wantCode: 1600},
		{name: "uint32", code: uint32(320000), wantCode: 320000},
		{name: "uint64", code: uint64(6400000000), wantCode: 6400000000},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := msgpack.Marshal(map[string]interface{}{
				"code": testCase.code,
				"msg":  "ok",
				"data": map[string]interface{}{"k": "v"},
			})
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			resp, err := ParseMsgPackResponse(payload)
			if err != nil {
				t.Fatalf("ParseMsgPackResponse() error = %v", err)
			}
			if resp.Code != testCase.wantCode {
				t.Fatalf("Code = %d, want %d", resp.Code, testCase.wantCode)
			}
			if resp.Msg != "ok" {
				t.Fatalf("Msg = %q, want %q", resp.Msg, "ok")
			}
			if resp.Data == nil {
				t.Fatal("Data = nil, want the decoded payload")
			}
		})
	}
}

// TestParseMsgPackEnvelope_RejectsOutOfRangeAndWrongTypedCode 钉住溢出与类型守卫：
// 超出 int 范围的 code 必须报错而不是静默截断，非整数 code 必须被拒绝。
func TestParseMsgPackEnvelope_RejectsOutOfRangeAndWrongTypedCode(t *testing.T) {
	// 仅在 int 为 64 位的平台上，uint64 溢出分支才可达；32 位平台下 uint32
	// 也会溢出。用 math.MaxUint64 保证两种平台都触发溢出守卫。
	tests := []struct {
		name    string
		code    interface{}
		wantErr string
	}{
		{name: "uint64 overflows int", code: uint64(math.MaxUint64), wantErr: "overflows int"},
		{name: "string code rejected", code: "700", wantErr: "want integer"},
		{name: "float code rejected", code: 7.5, wantErr: "want integer"},
		{name: "bool code rejected", code: true, wantErr: "want integer"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := msgpack.Marshal(map[string]interface{}{
				"code": testCase.code,
				"msg":  "ok",
			})
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if _, err := ParseMsgPackResponse(payload); err == nil {
				t.Fatal("ParseMsgPackResponse() error = nil, want an error")
			} else if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), testCase.wantErr)
			}
		})
	}
}

// TestParseMsgPackResponseWithType_DecodesIntoTargetAndFallsBack 覆盖
// ParseMsgPackResponseWithType 的三条分支：解码进目标类型、dataType 为 nil 时
// 保留原始值、以及目标类型不匹配时返回错误。
func TestParseMsgPackResponseWithType_DecodesIntoTargetAndFallsBack(t *testing.T) {
	type target struct {
		Name string `msgpack:"name"`
		Age  int    `msgpack:"age"`
	}

	payload, err := msgpack.Marshal(map[string]interface{}{
		"code": 700,
		"msg":  "ok",
		"data": map[string]interface{}{"name": "fiberhouse", "age": 2},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	t.Run("decodes into the requested type", func(t *testing.T) {
		into := &target{}
		resp, err := ParseMsgPackResponseWithType(payload, into)
		if err != nil {
			t.Fatalf("ParseMsgPackResponseWithType() error = %v", err)
		}
		if into.Name != "fiberhouse" || into.Age != 2 {
			t.Fatalf("decoded = %#v, want {fiberhouse 2}", *into)
		}
		if resp.Data != into {
			t.Fatal("Data must point at the caller-supplied target")
		}
	})

	t.Run("nil dataType keeps the raw decoded value", func(t *testing.T) {
		resp, err := ParseMsgPackResponseWithType(payload, nil)
		if err != nil {
			t.Fatalf("ParseMsgPackResponseWithType() error = %v", err)
		}
		if _, ok := resp.Data.(map[string]interface{}); !ok {
			t.Fatalf("Data = %T, want the raw map when dataType is nil", resp.Data)
		}
	})

	t.Run("incompatible target reports the decode error", func(t *testing.T) {
		var into int
		if _, err := ParseMsgPackResponseWithType(payload, &into); err == nil {
			t.Fatal("ParseMsgPackResponseWithType() error = nil, want a decode error")
		}
	})

	t.Run("envelope without data leaves Data unset", func(t *testing.T) {
		noData, err := msgpack.Marshal(map[string]interface{}{"code": 700, "msg": "ok"})
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		into := &target{}
		resp, err := ParseMsgPackResponseWithType(noData, into)
		if err != nil {
			t.Fatalf("ParseMsgPackResponseWithType() error = %v", err)
		}
		if resp.Data != nil {
			t.Fatalf("Data = %#v, want nil when the envelope carries no data", resp.Data)
		}
	})
}
