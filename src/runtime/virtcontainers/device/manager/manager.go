// Copyright (c) 2017-2018 Intel Corporation
// Copyright (c) 2018 Huawei Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package manager

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/kata-containers/runtime/virtcontainers/device/api"
	"github.com/kata-containers/runtime/virtcontainers/device/config"
	"github.com/kata-containers/runtime/virtcontainers/device/drivers"
	persistapi "github.com/kata-containers/runtime/virtcontainers/persist/api"
	"github.com/kata-containers/runtime/virtcontainers/utils"
)

const (
	// VirtioMmio indicates block driver is virtio-mmio based
	VirtioMmio string = "virtio-mmio"
	// VirtioBlock indicates block driver is virtio-blk based
	VirtioBlock string = "virtio-blk"
	// VirtioBlockCCW indicates block driver is virtio-blk-ccw based
	VirtioBlockCCW string = "virtio-blk-ccw"
	// VirtioSCSI indicates block driver is virtio-scsi based
	VirtioSCSI string = "virtio-scsi"
	// Nvdimm indicates block driver is nvdimm based
	Nvdimm string = "nvdimm"
)

var (
	// ErrIDExhausted represents that devices are too many
	// and no more IDs can be generated
	ErrIDExhausted = errors.New("IDs are exhausted")
	// ErrDeviceNotExist represents device hasn't been created before
	ErrDeviceNotExist = errors.New("device with specified ID hasn't been created")
	// ErrDeviceNotAttached represents the device isn't attached
	ErrDeviceNotAttached = errors.New("device isn't attached")
	// ErrRemoveAttachedDevice represents the device isn't detached
	// so not allow to remove from list
	ErrRemoveAttachedDevice = errors.New("can't remove attached device")
)

type deviceManager struct {
	blockDriver           string
	vhostUserStoreEnabled bool
	vhostUserStorePath    string

	devices map[string]api.Device
	sync.RWMutex
}

func deviceLogger() *logrus.Entry {
	return api.DeviceLogger().WithField("subsystem", "device")
}

// NewDeviceManager creates a deviceManager object behaved as api.DeviceManager
func NewDeviceManager(blockDriver string, vhostUserStoreEnabled bool, vhostUserStorePath string, devices []api.Device) api.DeviceManager {
	dm := &deviceManager{
		vhostUserStoreEnabled: vhostUserStoreEnabled,
		vhostUserStorePath:    vhostUserStorePath,
		devices:               make(map[string]api.Device),
	}
	if blockDriver == VirtioMmio {
		dm.blockDriver = VirtioMmio
	} else if blockDriver == VirtioBlock {
		dm.blockDriver = VirtioBlock
	} else if blockDriver == Nvdimm {
		dm.blockDriver = Nvdimm
	} else if blockDriver == VirtioBlockCCW {
		dm.blockDriver = VirtioBlockCCW
	} else {
		dm.blockDriver = VirtioSCSI
	}

	drivers.AllPCIeDevs = make(map[string]bool)

	for _, dev := range devices {
		dm.devices[dev.DeviceID()] = dev
	}
	return dm
}

func (dm *deviceManager) findDeviceByMajorMinor(major, minor int64) api.Device {
	for _, dev := range dm.devices {
		dma, dmi := dev.GetMajorMinor()
		if dma == major && dmi == minor {
			return dev
		}
	}
	return nil
}

// createDevice creates one device based on DeviceInfo
func (dm *deviceManager) createDevice(devInfo config.DeviceInfo) (dev api.Device, err error) {
	// pmem device may points to block devices or raw files,
	// do not change its HostPath.
	if !devInfo.Pmem {
		path, err := config.GetHostPathFunc(devInfo, dm.vhostUserStoreEnabled, dm.vhostUserStorePath)
		if err != nil {
			return nil, err
		}
		devInfo.HostPath = path
	}

	defer func() {
		if err == nil {
			dev.Reference()
		}
	}()

	if existingDev := dm.findDeviceByMajorMinor(devInfo.Major, devInfo.Minor); existingDev != nil {
		return existingDev, nil
	}

	// device ID must be generated by manager instead of device itself
	// in case of ID collision
	if devInfo.ID, err = dm.newDeviceID(); err != nil {
		return nil, err
	}
	if isVFIO(devInfo.HostPath) {
		return drivers.NewVFIODevice(&devInfo), nil
	} else if isVhostUserBlk(devInfo) {
		if devInfo.DriverOptions == nil {
			devInfo.DriverOptions = make(map[string]string)
		}
		devInfo.DriverOptions["block-driver"] = dm.blockDriver
		return drivers.NewVhostUserBlkDevice(&devInfo), nil
	} else if isBlock(devInfo) {
		if devInfo.DriverOptions == nil {
			devInfo.DriverOptions = make(map[string]string)
		}
		devInfo.DriverOptions["block-driver"] = dm.blockDriver
		return drivers.NewBlockDevice(&devInfo), nil
	} else {
		deviceLogger().WithField("device", devInfo.HostPath).Info("Device has not been passed to the container")
		return drivers.NewGenericDevice(&devInfo), nil
	}
}

// NewDevice creates a device based on specified DeviceInfo
func (dm *deviceManager) NewDevice(devInfo config.DeviceInfo) (api.Device, error) {
	dm.Lock()
	defer dm.Unlock()
	dev, err := dm.createDevice(devInfo)
	if err == nil {
		dm.devices[dev.DeviceID()] = dev
	}
	return dev, err
}

// RemoveDevice deletes the device from list based on specified device id
func (dm *deviceManager) RemoveDevice(id string) error {
	dm.Lock()
	defer dm.Unlock()
	dev, ok := dm.devices[id]
	if !ok {
		return ErrDeviceNotExist
	}

	if dev.Dereference() == 0 {
		if dev.GetAttachCount() > 0 {
			return ErrRemoveAttachedDevice
		}
		delete(dm.devices, id)
	}
	return nil
}

func (dm *deviceManager) newDeviceID() (string, error) {
	for i := 0; i < 5; i++ {
		// generate an random ID
		randBytes, err := utils.GenerateRandomBytes(8)
		if err != nil {
			return "", err
		}
		id := hex.EncodeToString(randBytes)

		// check ID collision, choose another one if ID is in use
		if _, ok := dm.devices[id]; !ok {
			return id, nil
		}
	}
	return "", ErrIDExhausted
}

func (dm *deviceManager) AttachDevice(id string, dr api.DeviceReceiver) error {
	dm.Lock()
	defer dm.Unlock()

	d, ok := dm.devices[id]
	if !ok {
		return ErrDeviceNotExist
	}

	if err := d.Attach(dr); err != nil {
		return err
	}
	return nil
}

func (dm *deviceManager) DetachDevice(id string, dr api.DeviceReceiver) error {
	dm.Lock()
	defer dm.Unlock()

	d, ok := dm.devices[id]
	if !ok {
		return ErrDeviceNotExist
	}
	if d.GetAttachCount() == 0 {
		return ErrDeviceNotAttached
	}

	if err := d.Detach(dr); err != nil {
		return err
	}
	return nil
}

func (dm *deviceManager) GetDeviceByID(id string) api.Device {
	dm.RLock()
	defer dm.RUnlock()
	if d, ok := dm.devices[id]; ok {
		return d
	}
	return nil
}

func (dm *deviceManager) GetAllDevices() []api.Device {
	dm.RLock()
	defer dm.RUnlock()
	devices := []api.Device{}
	for _, v := range dm.devices {
		devices = append(devices, v)
	}
	return devices
}

func (dm *deviceManager) IsDeviceAttached(id string) bool {
	dm.RLock()
	defer dm.RUnlock()
	d, ok := dm.devices[id]
	if !ok {
		return false
	}
	return d.GetAttachCount() > 0
}

// NewDevice creates a device based on specified DeviceInfo
func (dm *deviceManager) LoadDevices(devStates []persistapi.DeviceState) {
	dm.Lock()
	defer dm.Unlock()

	for _, ds := range devStates {
		var dev api.Device

		switch config.DeviceType(ds.Type) {
		case config.DeviceGeneric:
			dev = &drivers.GenericDevice{}
		case config.DeviceBlock:
			dev = &drivers.BlockDevice{}
		case config.DeviceVFIO:
			dev = &drivers.VFIODevice{}
		case config.VhostUserSCSI:
			dev = &drivers.VhostUserSCSIDevice{}
		case config.VhostUserBlk:
			dev = &drivers.VhostUserBlkDevice{}
		case config.VhostUserNet:
			dev = &drivers.VhostUserNetDevice{}
		default:
			deviceLogger().WithField("device-type", ds.Type).Warning("unrecognized device type is detected")
			// continue the for loop
			continue
		}

		dev.Load(ds)
		dm.devices[dev.DeviceID()] = dev
	}
}
