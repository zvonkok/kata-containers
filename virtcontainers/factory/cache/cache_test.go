// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/factory/direct"
	"github.com/kata-containers/runtime/virtcontainers/persist/fs"
)

func TestTemplateFactory(t *testing.T) {
	assert := assert.New(t)

	testDir := fs.MockStorageRootPath()
	defer fs.MockStorageDestroy()

	hyperConfig := vc.HypervisorConfig{
		KernelPath: testDir,
		ImagePath:  testDir,
	}
	vmConfig := vc.VMConfig{
		HypervisorType:   vc.MockHypervisor,
		AgentType:        vc.NoopAgentType,
		ProxyType:        vc.NoopProxyType,
		HypervisorConfig: hyperConfig,
	}

	ctx := context.Background()

	// New
	f := New(ctx, 2, direct.New(ctx, vmConfig))

	// Config
	assert.Equal(f.Config(), vmConfig)

	// GetBaseVM
	vm, err := f.GetBaseVM(ctx, vmConfig)
	assert.Nil(err)

	err = vm.Stop()
	assert.Nil(err)

	// CloseFactory
	f.CloseFactory(ctx)
}
