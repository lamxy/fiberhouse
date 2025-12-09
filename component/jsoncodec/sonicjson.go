// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package jsoncodec

import (
	"encoding/json"
	"github.com/bytedance/sonic"
)

/* JSON编解码接口化，按需选择不同的编解码库：encoding/json、go-json、bytedance.sonic、etc... */

// SonicJSON Wrap sonic
// 默认sonic不支持html逃逸转义和SortKeys，有需要的场景自定义设置
type SonicJSON struct {
	Config        sonic.Config
	ConfigDefault sonic.API
}

// Marshal use Sonic Marshal
func (s *SonicJSON) Marshal(v interface{}) ([]byte, error) {
	return s.ConfigDefault.Marshal(v)
}

// Unmarshal use Sonic Unmarshal
func (s *SonicJSON) Unmarshal(data []byte, v interface{}) error {
	err := s.ConfigDefault.Unmarshal(data, v)
	if err != nil {
		// TODO metrics.Count("sonic_decode_error", 1)  // 指标监控(针对恶意请求)
		err = json.Unmarshal(data, v) // fallback，标准库json容错
	}
	return err
}

// SonicJsonEscape just start HTML escape
func SonicJsonEscape() *SonicJSON {
	snc := SonicJSON{
		Config: sonic.Config{
			EscapeHTML: true,
		},
	}
	return snc.SetCfg()
}

// SonicJsonSortEscape just start Keys Sort & HTML escape
func SonicJsonSortEscape() *SonicJSON {
	snc := SonicJSON{
		Config: sonic.Config{
			EscapeHTML:  true,
			SortMapKeys: true,
		},
	}
	return snc.SetCfg()
}

// SonicJsonDefault sonic ConfigDefault
func SonicJsonDefault() *SonicJSON {
	snc := SonicJSON{
		ConfigDefault: sonic.ConfigDefault,
	}
	return &snc
}

// SonicJsonStd sonic ConfigStd
func SonicJsonStd() *SonicJSON {
	snc := SonicJSON{
		ConfigDefault: sonic.ConfigStd,
	}
	return &snc
}

// SonicJsonFastest sonic ConfigFastest
func SonicJsonFastest() *SonicJSON {
	snc := SonicJSON{
		ConfigDefault: sonic.ConfigFastest,
	}
	return &snc
}

// SetCfg set sonic config
func (s *SonicJSON) SetCfg(cfg ...sonic.Config) *SonicJSON {
	if len(cfg) > 0 {
		s.Config = cfg[0]
	}
	s.ConfigDefault = s.Config.Froze()
	return s
}
