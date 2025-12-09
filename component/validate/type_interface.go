// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package validate

// RegisterValidatorTagFunc 是一个函数类型，用于注册自定义的校验标签
type RegisterValidatorTagFunc func(*Wrap) error

// ValidateRegister 是一个接口，要求实现 RegisterToWrap 方法。该方法接收一个 *Wrap 类型的参数，用于注册相关逻辑。此接口用于扩展校验注册功能，便于自定义校验器的集成
type ValidateRegister interface {
	RegisterToWrap(wrap *Wrap)
}

// ValidateInitializer 定义了一个函数类型，该函数返回一个 ValidateRegister 接口，用于初始化校验注册器
type ValidateInitializer func() ValidateRegister

// ValidateChecker 定义了一个接口，包含一个 Check 方法。该方法接受可变数量的参数，并返回一个错误。实现该接口的类型可以用于执行自定义的校验逻辑
type ValidateChecker interface {
	// 接受可变数量的参数，该可变参数可以是验证器包装器对象或其他可用于验证处理的参数
	Check(...interface{}) error
}
