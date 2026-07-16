// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT

//go:generate protoc -I ../.. --go_out=../.. --go_opt=paths=source_relative ../../response/pb/resp_info.proto

// Package responsepb contains the generated Protobuf representation of unified responses.
package responsepb
