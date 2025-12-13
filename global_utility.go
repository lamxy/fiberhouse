// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package fiberhouse

import (
	"errors"
	"fmt"
	"github.com/lamxy/fiberhouse/exception"
	"github.com/lamxy/fiberhouse/globalmanager"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"strings"
)

// RegisterKeyName 定义和拼接全局对象注册带命名空间的key，并返回注册key的名称
//
// namespace前缀规则:
// 1. 命名空间作为注册实例key前缀的一部分，起于模块子系统名字路径，表明对象属于其所在的模块/子系统，如"common-module."，表示common-module的模块名/子系统名前缀;
// 2. 模块名内部继续按照路径名/包名称用点号进行拼接: 如example-module模块内的模型model, 完整的命名空间为example-module.model.xxx;
// 3. 如ExampleModel类要注册进全局管理器，最终组合的key名称为: example-module.model.ExampleModel;
//
// fiberhouse.RegisterKeyName() 方法会自动帮你组合命名空间前缀和组件名称，生成完整的注册key名称；其中fiberhouse.GetNamespace()方法会帮你组合命名空间前缀部分，接受一个名字空间的切片，内部自动
// 按"."拼接名字空间后作为默认值，但当ns参数存在值时，由ns作为的命名空间前缀.
//
// 4. 组件注册到全局管理器的key名称，必须唯一，否则会报错;
// 5. 组件注册到全局管理器的key名称，必须符合标识符命名规范，只能包含字母、数字、下划线，且只能字母或下划线开头.
func RegisterKeyName(name string, ns ...string) (key string) {
	l := len(ns)
	if l == 0 {
		key = name
	} else {
		var list = make([]string, 0, l+1)
		list = append(list, ns...)
		list = append(list, name)
		key = strings.Join(list, ".")
	}
	return
}

// RegisterKeyInitializerFunc 按指定key注册全局对象到全局管理器，同时返回keyName
//
// key的命名空间前缀规则同 RegisterKeyName
func RegisterKeyInitializerFunc(keyName string, Initializer globalmanager.InitializerFunc) (key string) {
	key = keyName
	if key == "" {
		return
	}
	gm := globalmanager.NewGlobalManagerOnce()
	gm.Register(key, Initializer)
	return
}

// GetNamespace 支持自定義key前綴，儅ns為空切片時替代ns。儅ns有值時使用ns返回
func GetNamespace(overrides []string, ns ...string) []string {
	if len(ns) > 0 {
		return ns
	}
	if len(overrides) > 0 {
		return overrides
	}
	return ns
}

// RegisterKeyFuncType 定义获取注册全局管理器key的名称函数类型，用于便捷自动完成该函数签名
// @param ns string namespace key前缀命名空间
type RegisterKeyFuncType func(ns ...string) string

// RegisterInitializerFuncType 定义注册全局管理器初始化函数类型，用于便捷自动完成该函数签名
// @param ns string namespace key前缀命名空间
type RegisterInitializerFuncType func(ctx IApplicationContext, ns ...string) string

// GetInstance 从全局管理获取单例
//
// e.g.
//
//	 s, err := fiberhouse.GetInstance[*service.TestService](h.KeyNSService)
//		if err != nil {
//		}
func GetInstance[T any](name string) (T, error) {
	var zero T
	gm := globalmanager.NewGlobalManagerOnce()
	origin, err := gm.Get(name)
	if err != nil {
		return zero, err
	}
	if instance, ok := origin.(T); ok {
		return instance, nil
	}
	return zero, fmt.Errorf("assertion failure for type of '%s' instance", name)
}

// GetMustInstance 从全局管理获取单例，若不存在则panic
func GetMustInstance[T any](name string) T {
	gm := globalmanager.NewGlobalManagerOnce()
	origin, err := gm.Get(name)
	if err != nil {
		panic(err)
	}
	if instance, ok := origin.(T); ok {
		return instance
	}
	panic(fmt.Errorf("assertion failure for type of '%s' instance", name))
}

// GetNoDocumentsError 检查错误是否为 mongo.ErrNoDocuments，若是则返回零值和自定义错误
func GetNoDocumentsError[T any](err error) (T, error) {
	var zero T
	if errors.Is(err, mongo.ErrNoDocuments) {
		return zero, exception.GetNotFoundDocument()
	}
	return zero, err
}

// GetErrOrNoDocuments 检查错误是否为 mongo.ErrNoDocuments，若是则返回自定义错误
func GetErrOrNoDocuments(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return exception.GetNotFoundDocument()
	}
	return err
}
