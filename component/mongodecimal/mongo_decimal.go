// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package mongodecimal

import (
	"fmt"
	"github.com/govalues/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"
	"reflect"
)

// MongoDecimal 是一个值编解码器，允许十进制编码，将 Decimal128 与 decimal.Decimal 相互转换
type MongoDecimal struct{}

// 检查接口是否实现。如果没有，将在编译阶段报告错误。
var _ bson.ValueEncoder = &MongoDecimal{}
var _ bson.ValueDecoder = &MongoDecimal{}

// EncodeValue 从 decimal.Decimal 编码到 BSON
func (dc *MongoDecimal) EncodeValue(_ bson.EncodeContext, w bson.ValueWriter, value reflect.Value) error {
	dec, ok := value.Interface().(decimal.Decimal)
	if !ok {
		return fmt.Errorf("value %v to encode is not of type decimal.Decimal", value)
	}
	// Convert decimal.Decimal to primitive.Decimal128.
	primDec, err := bson.ParseDecimal128(dec.String())
	if err != nil {
		return fmt.Errorf("error converting decimal.Decimal %v to primitive.Decimal128: %v", dec, err)
	}
	return w.WriteDecimal128(primDec)
}

// DecodeValue 从 BSON 解码到 decimal.Decimal
func (dc *MongoDecimal) DecodeValue(_ bson.DecodeContext, r bson.ValueReader, value reflect.Value) error {
	primDec, err := r.ReadDecimal128()
	if err != nil {
		return fmt.Errorf("error reading primitive.Decimal128 from ValueReader: %v", err)
	}
	// Convert primitive.Decimal128 to Golang's decimal.Decimal.
	dec, err := decimal.Parse(primDec.String())
	if err != nil {
		return fmt.Errorf("error converting primitive.Decimal128 %v to decimal.Decimal: %v", primDec, err)
	}
	// Set the value to decimal.Decimal type data
	value.Set(reflect.ValueOf(dec))
	return nil
}
