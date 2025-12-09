// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package jsonconvert

import (
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"sync"
)

var (
	dataWrapPool = sync.Pool{
		New: func() interface{} {
			return &DataWrap{
				data:         nil,
				serializable: 0,
			}
		},
	}
)

// DataWrap 数据包装器
type DataWrap struct {
	data         interface{} // 非并发场景使用，无需原子操作 atomic.Value
	serializable int32       // 0=false, 1=true   // 非并发场景使用，无需原子操作 atomic.Int32
}

// DataWrapPoolGet 从池中获取DataWrap实例
func DataWrapPoolGet() *DataWrap {
	return dataWrapPool.Get().(*DataWrap)
}

func DataWrapPoolPut(dw *DataWrap) {
	if dw != nil {
		dw.data = nil
		dw.serializable = 0
		dataWrapPool.Put(dw)
	}
}

func NewDataWrap(d interface{}) *DataWrap {
	dw := DataWrapPoolGet()
	dw.SetData(d)

	// 处理nil值
	if d == nil {
		dw.data = ""
		dw.serializable = 0
		return dw
	}

	// 处理运行时错误
	switch dType := d.(type) {
	case runtime.Error:
		dw.data = "RuntimeError: " + dType.Error()
		dw.serializable = 0
		return dw
	}

	// 根据类型判断JSON序列化能力
	if dw.isJSONSerializable(d) {
		dw.serializable = 1
	} else {
		dw.serializable = 0
	}

	return dw
}

// SetData 设置数据
func (dw *DataWrap) SetData(d interface{}) {
	dw.data = d
}

// Release 释放DataWrap实例到池中
func (dw *DataWrap) Release() {
	dw.Reset()
	DataWrapPoolPut(dw)
}

// Reset 重置DataWrap实例属性
func (dw *DataWrap) Reset() {
	dw.data = nil
	dw.serializable = 0
}

// isJSONSerializable 判断是否可以JSON序列化
func (dw *DataWrap) isJSONSerializable(d interface{}) bool {
	dt := reflect.TypeOf(d)
	dv := reflect.ValueOf(d)

	switch dt.Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return true // 复杂类型，假设可序列化，后续尝试json编码，处理错误

	case reflect.Ptr:
		if dv.IsNil() {
			return true
		}
		return dw.isJSONSerializable(dv.Elem().Interface())

	case reflect.Interface:
		if dv.IsNil() {
			return true
		}
		return dw.isJSONSerializable(dv.Elem().Interface())

	default:
		return false // 基本类型不使用JSON序列化
	}
}

// CanJSONSerializable 是否可以JSON序列化
func (dw *DataWrap) CanJSONSerializable() bool {
	return dw.serializable == 1
}

// GetJson 复杂类型JSON序列化
func (dw *DataWrap) GetJson(jsonEncoder func(interface{}) ([]byte, error)) ([]byte, error) {
	if !dw.CanJSONSerializable() {
		return nil, errors.New("DataWrap GetJson: origin data is not serializable")
	}

	data, err := jsonEncoder(dw.data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetString 非复杂类型直接转换成字符串
func (dw *DataWrap) GetString() string {
	if dw.data == nil {
		return ""
	}

	if dw.CanJSONSerializable() {
		return ""
	}

	// 优先处理字符串类型（避免反射开销）
	if str, ok := dw.data.(string); ok {
		return str
	}

	dt := reflect.TypeOf(dw.data)
	dv := reflect.ValueOf(dw.data)

	switch dt.Kind() {
	case reflect.String:
		return dv.String()

	// 整数类型
	case reflect.Int:
		return strconv.FormatInt(int64(dw.data.(int)), 10)
	case reflect.Int8:
		return strconv.FormatInt(int64(dw.data.(int8)), 10)
	case reflect.Int16:
		return strconv.FormatInt(int64(dw.data.(int16)), 10)
	case reflect.Int32:
		return strconv.FormatInt(int64(dw.data.(int32)), 10)
	case reflect.Int64:
		return strconv.FormatInt(dw.data.(int64), 10)

	// 无符号整数类型
	case reflect.Uint:
		return strconv.FormatUint(uint64(dw.data.(uint)), 10)
	case reflect.Uint8:
		return strconv.FormatUint(uint64(dw.data.(uint8)), 10)
	case reflect.Uint16:
		return strconv.FormatUint(uint64(dw.data.(uint16)), 10)
	case reflect.Uint32:
		return strconv.FormatUint(uint64(dw.data.(uint32)), 10)
	case reflect.Uint64:
		return strconv.FormatUint(dw.data.(uint64), 10)

	// 浮点数类型
	case reflect.Float32:
		return strconv.FormatFloat(float64(dw.data.(float32)), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(dw.data.(float64), 'g', -1, 64)

	// 布尔类型
	case reflect.Bool:
		return strconv.FormatBool(dw.data.(bool))

	// 字节切片
	case reflect.Slice:
		if dt.Elem().Kind() == reflect.Uint8 {
			return string(dv.Bytes())
		}
		return "[Unpeeled slice]"

	// 指针类型
	case reflect.Ptr:
		if dv.IsNil() {
			return ""
		}
		return (&DataWrap{data: dv.Elem().Interface()}).GetString()

	// 接口类型
	case reflect.Interface:
		if dv.IsNil() {
			return ""
		}
		return (&DataWrap{data: dv.Elem().Interface()}).GetString()

	default:
		return ""
	}
}

// GetData 获取原始数据
func (dw *DataWrap) GetData() interface{} {
	return dw.data
}
