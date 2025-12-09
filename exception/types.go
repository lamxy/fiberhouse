// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package exception

import "github.com/lamxy/fiberhouse/response"

type Exception response.RespInfo

type ValidateException response.RespInfo

type ExceptionMap map[string]Exception

type ErrorData map[string]string
