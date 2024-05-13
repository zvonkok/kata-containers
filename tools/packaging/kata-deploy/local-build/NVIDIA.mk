
# Copyright (c) 2024 NVIDIA Corporation
#
# SPDX-License-Identifier: Apache-2.0
#

NVIDIA_BASE_TARBALLS = nvidia-serial-targets \
	ovmf-tarball \ 
	nydus-tarball \
	qemu-tarball \
	qemu-snp-experimental-tarball \
	shim-v2-tarball \
	virtiofsd-tarball \ 
	rootfs-nvidia-gpu-initrd-tarball \
	rootfs-nvidia-gpu-image-tarball \
	rootfs-nvidia-gpu-initrd-confidential-tarball \
	rootfs-nvidia-gpu-image-confidential-tarball 
NVIDIA_BASE_SERIAL_TARBALLS = kernel-nvidia-gpu-tarball \
	kernel-nvidia-gpu-confidential-tarball 

nvidia-gpu: ${NVIDIA_BASE_TARBALLS}

nvidia-serial-targets:
	${MAKE} -f $(MK_PATH) -j 1 V= \
	${NVIDIA_BASE_SERIAL_TARBALLS}