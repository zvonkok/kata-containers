// Copyright (c) 2017-2018 Intel Corporation
// Copyright (c) 2018 Huawei Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package drivers

import (
	"encoding/hex"

	"github.com/kata-containers/runtime/virtcontainers/device/api"
	"github.com/kata-containers/runtime/virtcontainers/device/config"
	"github.com/kata-containers/runtime/virtcontainers/utils"
)

// VhostUserBlkDevice is a block vhost-user based device
type VhostUserBlkDevice struct {
	*GenericDevice
	config.VhostUserDeviceAttrs
}

//
// VhostUserBlkDevice's implementation of the device interface:
//

// Attach is standard interface of api.Device, it's used to add device to some
// DeviceReceiver
func (device *VhostUserBlkDevice) Attach(devReceiver api.DeviceReceiver) (err error) {
	if device.DeviceInfo.Hotplugged {
		return nil
	}

	// generate a unique ID to be used for hypervisor commandline fields
	randBytes, err := utils.GenerateRandomBytes(8)
	if err != nil {
		return err
	}
	id := hex.EncodeToString(randBytes)

	device.DevID = id
	device.Type = device.DeviceType()

	defer func() {
		if err == nil {
			device.DeviceInfo.Hotplugged = true
		}
	}()
	return devReceiver.AppendDevice(device)
}

// Detach is standard interface of api.Device, it's used to remove device from some
// DeviceReceiver
func (device *VhostUserBlkDevice) Detach(devReceiver api.DeviceReceiver) error {
	if !device.DeviceInfo.Hotplugged {
		return nil
	}

	device.DeviceInfo.Hotplugged = false
	return nil
}

// DeviceType is standard interface of api.Device, it returns device type
func (device *VhostUserBlkDevice) DeviceType() config.DeviceType {
	return config.VhostUserBlk
}

// GetDeviceInfo returns device information used for creating
func (device *VhostUserBlkDevice) GetDeviceInfo() interface{} {
	device.Type = device.DeviceType()
	return &device.VhostUserDeviceAttrs
}

// It should implement IsAttached() and DeviceID() as api.Device implementation
// here it shares function from *GenericDevice so we don't need duplicate codes
