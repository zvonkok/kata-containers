// Copyright (c) 2019-2022 Alibaba Cloud
// Copyright (c) 2019-2022 Ant Group
//
// SPDX-License-Identifier: Apache-2.0
//

use super::{Rootfs, ROOTFS};
use crate::share_fs::{do_get_guest_path, do_get_host_path};
use agent::Storage;
use anyhow::{anyhow, Context, Result};
use async_trait::async_trait;
use hypervisor::{device::device_manager::DeviceManager, BlockConfig, DeviceConfig};
use kata_types::mount::Mount;
use nix::sys::stat::{self, SFlag};
use std::fs;
use tokio::sync::RwLock;

pub(crate) struct BlockRootfs {
    guest_path: String,
    device_id: String,
    mount: oci::Mount,
    storage: Option<agent::Storage>,
}

impl BlockRootfs {
    pub async fn new(
        d: &RwLock<DeviceManager>,
        sid: &str,
        cid: &str,
        dev_id: u64,
        rootfs: &Mount,
    ) -> Result<Self> {
        let container_path = do_get_guest_path(ROOTFS, cid, false, false);
        let host_path = do_get_host_path(ROOTFS, sid, cid, false, false);
        // Create rootfs dir on host to make sure mount point in guest exists, as readonly dir is
        // shared to guest via virtiofs, and guest is unable to create rootfs dir.
        fs::create_dir_all(&host_path)
            .map_err(|e| anyhow!("failed to create rootfs dir {}: {:?}", host_path, e))?;

        let block_device_config = &mut BlockConfig {
            major: stat::major(dev_id) as i64,
            minor: stat::minor(dev_id) as i64,
            ..Default::default()
        };

        let device_id = d
            .write()
            .await
            .new_device(&DeviceConfig::Block(block_device_config.clone()))
            .await
            .context("failed to create deviec")?;

        d.write()
            .await
            .try_add_device(device_id.as_str())
            .await
            .context("failed to add deivce")?;

        let mut storage = Storage {
            fs_type: rootfs.fs_type.clone(),
            mount_point: container_path.clone(),
            options: rootfs.options.clone(),
            ..Default::default()
        };

        // get complete device information
        let dev_info = d
            .read()
            .await
            .get_device_info(device_id.as_str())
            .await
            .context("failed to get device info")?;

        if let DeviceConfig::Block(config) = dev_info {
            storage.driver = config.driver_option;
            storage.source = config.virt_path;
        }

        Ok(Self {
            guest_path: container_path.clone(),
            device_id,
            mount: oci::Mount {
                ..Default::default()
            },
            storage: Some(storage),
        })
    }
}

#[async_trait]
impl Rootfs for BlockRootfs {
    async fn get_guest_rootfs_path(&self) -> Result<String> {
        Ok(self.guest_path.clone())
    }

    async fn get_rootfs_mount(&self) -> Result<Vec<oci::Mount>> {
        Ok(vec![self.mount.clone()])
    }

    async fn get_storage(&self) -> Option<Storage> {
        self.storage.clone()
    }

    async fn get_device_id(&self) -> Result<Option<String>> {
        Ok(Some(self.device_id.clone()))
    }
    async fn cleanup(&self) -> Result<()> {
        Ok(())
    }
}

pub(crate) fn is_block_rootfs(file: &str) -> Option<u64> {
    if file.is_empty() {
        return None;
    }
    match stat::stat(file) {
        Ok(fstat) => {
            if SFlag::from_bits_truncate(fstat.st_mode) == SFlag::S_IFBLK {
                let dev_id = fstat.st_rdev;
                return Some(dev_id);
            }
        }
        Err(_) => return None,
    };
    None
}
