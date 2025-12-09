package jsonconvert

import (
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"sync"
)

var (
	dataWrapOptPool = sync.Pool{
		New: func() interface{} {
			return &DataWrapOpt{
				data:         nil,
				serializable: 0,
			}
		},
	}
)

// DataWrapOpt 数据包装优化器
// 减少 reflect.Value 的使用，覆盖常见类型为 type-switch。
type DataWrapOpt struct {
	data         interface{}
	serializable int32 // 0=false, 1=true
}

// DataWrapOptPoolGet 从池中获取DataWrapOpt实例
func DataWrapOptPoolGet() *DataWrapOpt {
	return dataWrapOptPool.Get().(*DataWrapOpt)
}

// DataWrapOptPoolPut 归还DataWrapOpt实例到池中
func DataWrapOptPoolPut(dw *DataWrapOpt) {
	if dw != nil {
		dw.data = nil
		dw.serializable = 0
		dataWrapOptPool.Put(dw)
	}
}

func NewDataWrapOpt(d interface{}) *DataWrapOpt {
	dw := DataWrapOptPoolGet()
	dw.SetData(d)
	//dw := &DataWrapOpt{
	//	data: d,
	//}

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

	if isJSONSerializableOpt(d) {
		dw.serializable = 1
	} else {
		dw.serializable = 0
	}
	return dw
}

func (dw *DataWrapOpt) SetData(d interface{}) {
	dw.data = d
}

func (dw *DataWrapOpt) Release() {
	dw.Reset()
	DataWrapOptPoolPut(dw)
}

func (dw *DataWrapOpt) Reset() {
	dw.data = nil
	dw.serializable = 0
}

func (dw *DataWrapOpt) CanJSONSerializable() bool {
	return dw.serializable == 1
}

func (dw *DataWrapOpt) GetJson(jsonEncoder func(interface{}) ([]byte, error)) ([]byte, error) {
	if !dw.CanJSONSerializable() {
		return nil, errors.New("DataWrapOpt GetJson: origin data is not serializable")
	}
	return jsonEncoder(dw.data)
}

// GetString 优先 type switch，必要时最少一次 reflect.ValueOf
func (dw *DataWrapOpt) GetString() string {
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

	// 零分配 type switch for common primitive types
	switch v := dw.data.(type) {
	case []byte:
		return string(v)
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}

	// 回退，仅做一次 reflect.ValueOf
	rv := reflect.ValueOf(dw.data)
	return getStringFromValueOpt(rv)
}

func getStringFromValueOpt(rv reflect.Value) string {
	switch rv.Kind() {
	case reflect.String:
		return rv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool())
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return string(rv.Bytes())
		}
		return "[Unpeeled slice]"
	case reflect.Ptr:
		if rv.IsNil() {
			return ""
		}
		return getStringFromValueOpt(rv.Elem())
	case reflect.Interface:
		if rv.IsNil() {
			return ""
		}
		return getStringFromValueOpt(rv.Elem())
	default:
		return ""
	}
}

// isJSONSerializableOpt 优先覆盖常见基本类型，避免reflect
func isJSONSerializableOpt(d interface{}) bool {
	if d == nil {
		return true
	}
	switch d.(type) {
	case string, []byte,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return false
	}

	v := reflect.ValueOf(d)
	t := v.Type()
	switch t.Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return true
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return isJSONSerializableOpt(v.Elem().Interface())
	default:
		return false
	}
}
