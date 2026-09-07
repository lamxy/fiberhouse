// Copyright (c) 2025 lamxy and Contributors
// SPDX-License-Identifier: MIT
//
// Author: lamxy <pytho5170@hotmail.com>
// GitHub: https://github.com/lamxy

// 本文件使用外部测试包 globalmanager_test，只能访问导出标识符。
//
// 这正是它存在的理由：包内测试无法证明「调用方可以分类某个错误」，因为包内测试
// 对私有哨兵同样可见。维护冲突是可恢复条件，调用方必须能够 errors.Is 它才能实现
// 退避重试；只有从包外断言，才真正锁定这条公开契约。
package globalmanager_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/lamxy/fiberhouse/globalmanager"
)

// blockingClosable 在 Close 中阻塞，用于把一个维护操作稳定地停在临界区内。
type blockingClosable struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingClosable) Close() error {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return nil
}

// TestErrMaintenanceInProgress_IsClassifiableByExternalCaller 证明包外调用方可以把
// 维护冲突识别为一个可重试条件，而不必匹配错误字符串。
func TestErrMaintenanceInProgress_IsClassifiableByExternalCaller(t *testing.T) {
	manager := globalmanager.NewGlobalManager()
	resource := &blockingClosable{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	if !manager.Register("resource", func() (interface{}, error) { return resource, nil }) {
		t.Fatal("Register() returned false for a new key")
	}
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// 第一个 Release 进入 Close 并停在那里，占住维护闸门。
	firstDone := make(chan error, 1)
	go func() { firstDone <- manager.Release("resource") }()
	<-resource.entered

	// 第二个 Release 必须 fail-fast，且可被包外 errors.Is 分类。
	err := manager.Release("resource")
	if err == nil {
		t.Fatal("concurrent Release() returned nil, want a busy error")
	}
	if !errors.Is(err, globalmanager.ErrMaintenanceInProgress) {
		t.Fatalf("concurrent Release() error = %v, want errors.Is(err, ErrMaintenanceInProgress)", err)
	}
	// 错误文本必须带上 key，否则日志无法定位是哪个对象在冲突。
	if !containsKey(err.Error(), "resource") {
		t.Fatalf("busy error %q does not name the conflicting key", err.Error())
	}

	close(resource.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Release() error = %v", err)
	}

	// 闸门释放后重试必须能够成功——这正是调用方分类该错误后要做的事。
	if _, err := manager.Get("resource"); err != nil {
		t.Fatalf("Get() after release error = %v", err)
	}
	if err := manager.Release("resource"); err != nil {
		t.Fatalf("retried Release() error = %v", err)
	}
}

func containsKey(message, key string) bool {
	for i := 0; i+len(key) <= len(message); i++ {
		if message[i:i+len(key)] == key {
			return true
		}
	}
	return false
}
