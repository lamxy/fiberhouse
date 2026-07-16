// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package jsoncodec

import (
	"encoding/json"
	ginJson "github.com/gin-gonic/gin/codec/json"
	"io"
)

// StdJSON 使用标准库 encoding/json 的编解码实现
type StdJSON struct{}

// Marshal 使用标准库 json.Marshal
func (s *StdJSON) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal 使用标准库 json.Unmarshal
func (s *StdJSON) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// MarshalIndent 使用标准库 json.MarshalIndent
func (s *StdJSON) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// NewEncoder 使用标准库 json.NewEncoder
func (s *StdJSON) NewEncoder(writer io.Writer) ginJson.Encoder {
	return json.NewEncoder(writer)
}

// NewDecoder 使用标准库 json.NewDecoder
func (s *StdJSON) NewDecoder(reader io.Reader) ginJson.Decoder {
	return json.NewDecoder(reader)
}

// StdJsonDefault 返回标准库 JSON 编解码器实例
func StdJsonDefault() *StdJSON {
	return &StdJSON{}
}
