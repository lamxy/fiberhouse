// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

package plugin

import (
	"github.com/lamxy/fiberhouse"
)

// TODO

type Plugin interface {
	fiberhouse.IProvider
	Start() error
	Stop() error
	Restart() error
}
