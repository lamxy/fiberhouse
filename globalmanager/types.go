// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package globalmanager

type KeyName = string

type InitializerFunc func() (interface{}, error)

type InitializerMap map[string]InitializerFunc

type RegisterKeyFunc func(...string) string

type RegisterKeySlice []func(...string) string
