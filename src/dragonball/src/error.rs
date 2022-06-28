// Copyright (C) 2022 Alibaba Cloud. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Portions Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the THIRD-PARTY file

//! Error codes for the virtual machine monitor subsystem.

#[cfg(feature = "dbs-virtio-devices")]
use dbs_virtio_devices::Error as VirtIoError;

use crate::device_manager;

/// Shorthand result type for internal VMM commands.
pub type Result<T> = std::result::Result<T, Error>;

/// Errors associated with the VMM internal logic.
///
/// These errors cannot be generated by direct user input, but can result from bad configuration
/// of the host (for example if Dragonball doesn't have permissions to open the KVM fd).
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// Failure occurs in issuing KVM ioctls and errors will be returned from kvm_ioctls lib.
    #[error("failure in issuing KVM ioctl command")]
    Kvm(#[source] kvm_ioctls::Error),

    /// The host kernel reports an unsupported KVM API version.
    #[error("unsupported KVM version {0}")]
    KvmApiVersion(i32),

    /// Cannot initialize the KVM context due to missing capabilities.
    #[error("missing KVM capability")]
    KvmCap(kvm_ioctls::Cap),

    #[cfg(target_arch = "x86_64")]
    #[error("failed to configure MSRs")]
    /// Cannot configure MSRs
    GuestMSRs(dbs_arch::msr::Error),

    /// MSR inner error
    #[error("MSR inner error")]
    Msr(vmm_sys_util::fam::Error),
}

/// Errors associated with starting the instance.
#[derive(Debug, thiserror::Error)]
pub enum StartMicrovmError {
    /// Cannot read from an Event file descriptor.
    #[error("failure while reading from EventFd file descriptor")]
    EventFd,

    /// The device manager was not configured.
    #[error("the device manager failed to manage devices: {0}")]
    DeviceManager(#[source] device_manager::DeviceMgrError),

    /// Cannot add devices to the Legacy I/O Bus.
    #[error("failure in managing legacy device: {0}")]
    LegacyDevice(#[source] device_manager::LegacyDeviceError),

    #[cfg(feature = "virtio-vsock")]
    /// Failed to create the vsock device.
    #[error("cannot create virtio-vsock device: {0}")]
    CreateVsockDevice(#[source] VirtIoError),

    #[cfg(feature = "virtio-vsock")]
    /// Cannot initialize a MMIO Vsock Device or add a device to the MMIO Bus.
    #[error("failure while registering virtio-vsock device: {0}")]
    RegisterVsockDevice(#[source] device_manager::DeviceMgrError),
}
