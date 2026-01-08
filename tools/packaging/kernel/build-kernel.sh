#!/usr/bin/env bash
#
# Copyright (c) 2018 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

readonly script_name="$(basename "${BASH_SOURCE[0]}")"
readonly script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

#project_name
readonly project_name="kata-containers"
[ -n "${GOPATH:-}" ] || GOPATH="${HOME}/go"
# Fetch the first element from GOPATH as working directory
# as go get only works against the first item in the GOPATH
GOPATH="${GOPATH%%:*}"
# Kernel version to be used
kernel_version=""
# Flag know if need to download the kernel source
download_kernel=false
# Default path to search patches to apply to kernel
readonly default_patches_dir="${script_dir}/patches"
# Default path to search config for kata
readonly default_kernel_config_dir="${script_dir}/configs"
# Default path to search for kernel config fragments
readonly default_config_frags_dir="${script_dir}/configs/fragments"
readonly default_config_whitelist="${script_dir}/configs/fragments/whitelist.conf"
readonly default_initramfs="${script_dir}/initramfs.cpio.gz"
# xPU vendor
readonly VENDOR_INTEL="intel"
readonly VENDOR_NVIDIA="nvidia"
readonly KBUILD_SIGN_PIN=${KBUILD_SIGN_PIN:-""}
readonly KERNEL_DEBUG_ENABLED=${KERNEL_DEBUG_ENABLED:-"no"}

#Path to kernel directory
kernel_path=""
#Experimental kernel support. Pull from virtio-fs GitLab instead of kernel.org
build_type=""
#Force generate config when setup
force_setup_generate_config="false"
#GPU kernel support
gpu_vendor=""
#DPU kernel support
dpu_vendor=""
#Confidential guest type
conf_guest=""
#
patches_path=""
#
hypervisor_target=""
#
arch_target=""
#
kernel_config_path=""
#
skip_config_checks="false"
# destdir
DESTDIR="${DESTDIR:-/}"
#PREFIX=
PREFIX="${PREFIX:-/usr}"
#Kernel URL
kernel_url=""
#Linux headers for GPU guest fs module building
linux_headers=""
# Kernel Reference to download using git
kernel_ref=""
# Enable measurement of the guest rootfs at boot.
measured_rootfs="false"

CROSS_BUILD_ARG=""

packaging_scripts_dir="${script_dir}/../scripts"
# shellcheck source=tools/packaging/scripts/lib.sh
source "${packaging_scripts_dir}/lib.sh"

usage() {
	exit_code="$1"
	cat <<EOF
Overview:

	Build a kernel for Kata Containers

Usage:

	$script_name [options] <command> <argument>

Commands:

- setup

- build

- install

Options:

	-a <arch>   	: Arch target to build the kernel, such as aarch64/ppc64le/riscv64/s390x/x86_64.
	-b <type>    	: Enable optional config type.
	-c <path>   	: Path to config file to build the kernel.
	-D <vendor> 	: DPU/SmartNIC vendor, only nvidia.
	-d          	: Enable bash debug.
	-e          	: Enable experimental kernel.
	-E          	: Enable arch-specific experimental kernel, arch info offered by "-a".
	-f          	: Enable force generate config when setup, old kernel path and config will be removed.
	-g <vendor> 	: GPU vendor, intel or nvidia.
	-h          	: Display this help.
	-H <deb|rpm>	: Linux headers for guest fs module building.
	-m              : Enable measured rootfs.
	-k <path>   	: Path to kernel to build.
	-p <path>   	: Path to a directory with patches to apply to kernel.
	-r <ref>        : Enable git mode to download kernel using ref.
	-s          	: Skip .config checks
	-t <hypervisor>	: Hypervisor_target.
	-u <url>	: Kernel URL to be used to download the kernel tarball.
	-v <version>	: Kernel version to use if kernel path not provided.
	-x       	: All the confidential guest protection type for a specific architecture.
EOF
	exit "$exit_code"
}

# Convert architecture to the name used by the Linux kernel build system
arch_to_kernel() {
	local -r arch="$1"

	case "$arch" in
		aarch64) echo "arm64" ;;
		ppc64le) echo "powerpc" ;;
		riscv64) echo "riscv" ;;
		s390x) echo "s390" ;;
		x86_64) echo "$arch" ;;
		*) die "unsupported architecture: $arch" ;;
	esac
}

# When building for measured rootfs the initramfs image should be previously built.
check_initramfs_or_die() {
	[ -f "${default_initramfs}" ] || \
		die "Initramfs for measured rootfs not found at ${default_initramfs}"
}

get_git_kernel() {
	local kernel_path="${2:-}"

	if [ ! -d "${kernel_path}" ] ; then
		mkdir -p "${kernel_path}"
		pushd "${kernel_path}"
		local kernel_git_url="https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git"
		if [ -n "${kernel_url}" ]; then
			kernel_git_url="${kernel_url}"
		fi
		git init
		git remote add origin "${kernel_git_url}"
		popd
	fi
	pushd "${kernel_path}"
	git fetch --depth 1 origin "${kernel_ref}"
	git checkout "${kernel_ref}"
	popd
}

get_kernel() {
	local version="${1:-}"

	local kernel_path=${2:-}
	[ -n "${kernel_path}" ] || die "kernel_path not provided"
	[ ! -d "${kernel_path}" ] || die "kernel_path already exist"

	#Remove extra 'v'
	version=${version#v}

	local major_version=$(echo "${version}" | cut -d. -f1)
	local rc=$(echo "${version}" | grep -oE "\-rc[0-9]+$")

	local tar_suffix="tar.xz"
	if [ -n "${rc}" ]; then
		tar_suffix="tar.gz"
	fi
	kernel_tarball="linux-${version}.${tar_suffix}"

	if [ -z "${rc}" ]; then
		if [[ -f "${kernel_tarball}.sha256" ]] && (grep -qF "${kernel_tarball}" "${kernel_tarball}.sha256"); then
			info "Restore valid ${kernel_tarball}.sha256 to sha256sums.asc"
			cp -f "${kernel_tarball}.sha256" sha256sums.asc
		else
			shasum_url="https://cdn.kernel.org/pub/linux/kernel/v${major_version}.x/sha256sums.asc"
			info "Download kernel checksum file: sha256sums.asc from ${shasum_url}"
			curl --fail -OL "${shasum_url}"
			if (grep -F "${kernel_tarball}" sha256sums.asc >"${kernel_tarball}.sha256"); then
				info "sha256sums.asc is valid, ${kernel_tarball}.sha256 generated"
			else
				die "sha256sums.asc is invalid"
			fi
		fi
	else
		info "Release candidate kernels are not part of the official sha256sums.asc -- skipping sha256sum validation"
	fi

	if [ -f "${kernel_tarball}" ]; then
	       	if [ -n "${rc}" ] && ! sha256sum -c "${kernel_tarball}.sha256"; then
			info "invalid kernel tarball ${kernel_tarball} removing "
			rm -f "${kernel_tarball}"
		fi
	fi
	if [ ! -f "${kernel_tarball}" ]; then
		kernel_tarball_url="https://www.kernel.org/pub/linux/kernel/v${major_version}.x/${kernel_tarball}"
		if [ -n "${kernel_url}" ]; then
			kernel_tarball_url="${kernel_url}${kernel_tarball}"
		fi
		info "Download kernel version ${version}"
		info "Download kernel from: ${kernel_tarball_url}"
		curl --fail -OL "${kernel_tarball_url}"
	else
		info "kernel tarball already downloaded"
	fi

	if [ -z "${rc}" ]; then
		sha256sum -c "${kernel_tarball}.sha256"
	fi

	tar xf "${kernel_tarball}"

	mv "linux-${version}" "${kernel_path}"
}

get_major_kernel_version() {
	local version="${1}"
	[ -n "${version}" ] || die "kernel version not provided"
	major_version=$(echo "${version}" | cut -d. -f1)
	minor_version=$(echo "${version}" | cut -d. -f2)
	echo "${major_version}.${minor_version}"
}

# Make a kernel config file from generic and arch specific
# fragments
# - arg1 - path to arch specific fragments
# - arg2 - path to kernel sources
#
get_kernel_frag_path() {
	local arch_path="$1"
	local common_path="${arch_path}/../common"
	local gpu_path="${arch_path}/../gpu"
	local dpu_path="${arch_path}/../dpu"

	local kernel_path="$2"
	local arch="$3"
	local cmdpath="${kernel_path}/scripts/kconfig/merge_config.sh"
	local config_path="${arch_path}/.config"

	local arch_configs="$(ls ${arch_path}/*.conf)"
	# By default, exclude configs if they have !$arch tag in the header
	local exclude_tags="-e "\!${arch}""

	# Also, let confidential guest opt-out some insecure configs
	if [[ "${conf_guest}" != "" ]];then
		exclude_tags="${exclude_tags} -e "\!${conf_guest}""
	fi

	local common_configs="$(grep ${exclude_tags} ${common_path}/*.conf -L)"

	local extra_configs=""
	if [ "${build_type}" != "" ];then
		local build_type_dir=$(readlink -m "${arch_path}/../build-type/${build_type}")
		if [ ! -d "$build_type_dir" ]; then
			die "No config fragments dir for ${build_type}: ${build_type_dir}"
		fi
		extra_configs=$(find "$build_type_dir" -name '*.conf')
		if [ "${extra_configs}" == "" ];then
			die "No extra configs found in ${build_type_dir}"
		fi
	fi

	# These are the strings that the kernel merge_config.sh script kicks out
	# when it reports an error or warning condition. We search for them in the
	# output to try and fail when we think something has been misconfigured.
	local not_in_string="not in final"
	local redefined_string="redefined"
	local redundant_string="redundant"

	# Later, if we need to add kernel version specific subdirs in order to
	# handle specific cases, then add the path definition and search/list/cat
	# here.
	local all_configs="${common_configs} ${arch_configs}"
	if [[ ${build_type} != "" ]]; then
		all_configs="${all_configs} ${extra_configs}"
	fi

	if [[ "${gpu_vendor}" != "" ]];then
		info "Add kernel config for GPU due to '-g ${gpu_vendor}'"
		# If conf_guest is set we need to update the CONFIG_LOCALVERSION
		# to match the suffix created in install_kata
		# -nvidia-gpu-confidential, the linux headers will be named the very
		# same if build with make deb-pkg for TDX or SNP.
		local gpu_configs=$(mktemp).conf
		local gpu_subst_configs="${gpu_path}/${gpu_vendor}.${arch_target}.conf.in"
		if [[ "${conf_guest}" != "" ]];then
			export CONF_GUEST_SUFFIX="-${conf_guest}"
		else
			export CONF_GUEST_SUFFIX=""
		fi
		envsubst <${gpu_subst_configs} >${gpu_configs}
		unset CONF_GUEST_SUFFIX

		all_configs="${all_configs} ${gpu_configs}"
	fi

	if [[ "${dpu_vendor}" != "" ]]; then
		info "Add kernel config for DPU/SmartNIC due to '-n ${dpu_vendor}'"
		local dpu_configs="${dpu_path}/${dpu_vendor}.conf"
		all_configs="${all_configs} ${dpu_configs}"
	fi

	if [ "${measured_rootfs}" == "true" ]; then
		info "Enabling config for confidential guest trust storage protection"
		local cryptsetup_configs="$(ls ${common_path}/confidential_containers/cryptsetup.conf)"
		all_configs="${all_configs} ${cryptsetup_configs}"

		check_initramfs_or_die
		info "Enabling config for confidential guest measured boot"
		local initramfs_configs="$(ls ${common_path}/confidential_containers/initramfs.conf)"
		all_configs="${all_configs} ${initramfs_configs}"
	fi

	if [[ "${conf_guest}" != "" ]];then
		info "Enabling config for '${conf_guest}' confidential guest protection"
		local conf_configs="$(ls ${arch_path}/${conf_guest}/*.conf)"
		all_configs="${all_configs} ${conf_configs}"

		local tmpfs_configs="$(ls ${common_path}/confidential_containers/tmpfs.conf)"
		all_configs="${all_configs} ${tmpfs_configs}"
	fi

	if [[ "${KBUILD_SIGN_PIN}" != "" ]]; then
		info "Enabling config for module signing"
		local sign_configs
		sign_configs="$(ls ${common_path}/signing/module_signing.conf)"
		all_configs="${all_configs} ${sign_configs}"
	fi

	if [[ ${KERNEL_DEBUG_ENABLED} == "yes" ]]; then
		info "Enable kernel debug"
		local debug_configs="$(ls ${common_path}/common/debug.conf)"
		all_configs="${all_configs} ${debug_configs}"
	fi

	if [[ "$force_setup_generate_config" == "true" ]]; then
		info "Remove existing config ${config_path} due to '-f'"
		[ -f "$config_path" ] && rm -f "${config_path}"
		[ -f "$config_path".old ] && rm -f "${config_path}".old
	fi

	info "Constructing config from fragments: ${config_path}"


	export KCONFIG_CONFIG=${config_path}
	export ARCH=${arch_target}
	cd ${kernel_path}

	local results
	results=$( ${cmdpath} -r -n ${all_configs} )
	# Only consider results highlighting "not in final"
	results=$(grep "${not_in_string}" <<< "$results")
	# Do not care about options that are in whitelist
	results=$(grep -v -f ${default_config_whitelist} <<< "$results")
	local version_config_whitelist="${default_config_whitelist%.*}-${kernel_version}.conf"
	if [ -f ${version_config_whitelist} ]; then
		results=$(grep -v -f ${version_config_whitelist} <<< "$results")
	fi

	[[ "${skip_config_checks}" == "true" ]] && echo "${config_path}" && return

	# Did we request any entries that did not make it?
	local missing=$(echo $results | grep -v -q "${not_in_string}"; echo $?)
	if [ ${missing} -ne 0 ]; then
		info "Some CONFIG elements failed to make the final .config:"
		info "${results}"
		info "Generated config file can be found in ${config_path}"
		die "Failed to construct requested .config file"
	fi

	# Did we define something as two different values?
	local redefined=$(echo ${results} | grep -v -q "${redefined_string}"; echo $?)
	if [ ${redefined} -ne 0 ]; then
		info "Some CONFIG elements are redefined in fragments:"
		info "${results}"
		info "Generated config file can be found in ${config_path}"
		die "Failed to construct requested .config file"
	fi

	# Did we define something twice? Nominally this may not be an error, and it
	# might be convenient to allow it, but for now, let's pick up on them.
	local redundant=$(echo ${results} | grep -v -q "${redundant_string}"; echo $?)
	if [ ${redundant} -ne 0 ]; then
		info "Some CONFIG elements are redundant in fragments:"
		info "${results}"
		info "Generated config file can be found in ${config_path}"
		die "Failed to construct requested .config file"
	fi

	echo "${config_path}"
}

# Locate and return the path to the relevant kernel config file
# - arg1: kernel version
# - arg2: hypervisor target
# - arg3: arch target
# - arg4: kernel source path
get_default_kernel_config() {
	local version="${1}"

	local hypervisor="$2"
	local kernel_arch="$3"
	local kernel_path="$4"

	[ -n "${version}" ] || die "kernel version not provided"
	[ -n "${hypervisor}" ] || die "hypervisor not provided"
	[ -n "${kernel_arch}" ] || die "kernel arch not provided"

	archfragdir="${default_config_frags_dir}/${kernel_arch}"
	if [ -d "${archfragdir}" ]; then
		config="$(get_kernel_frag_path ${archfragdir} ${kernel_path} ${kernel_arch})"
	else
		[ "${hypervisor}" == "firecracker" ] && hypervisor="kvm"
		config="${default_kernel_config_dir}/${kernel_arch}_kata_${hypervisor}_${major_kernel}.x"
	fi

	[ -f "${config}" ] || die "failed to find default config ${config}"
	echo "${config}"
}

get_config_and_patches() {
	if [ -z "${patches_path}" ]; then
		patches_path="${default_patches_dir}"
	fi
}

get_config_version() {
	get_config_and_patches
	config_version_file="${default_patches_dir}/../kata_config_version"
	if [ -f "${config_version_file}" ]; then
		cat "${config_version_file}"
	else
		die "failed to find ${config_version_file}"
	fi
}

setup_kernel() {
	local kernel_path=${1:-}
	local gpu_vendor=${2:-}

	[ -n "${kernel_path}" ] || die "kernel_path not provided"

	if [[ "$force_setup_generate_config" == "true" ]] && [[ -d "$kernel_path" ]];then
		info "Remove existing directory ${kernel_path} due to '-f'"
		rm -rf "${kernel_path}"
	fi

	if [ -d "$kernel_path" ]; then
		info "${kernel_path} already exist"
		if [[ "${force_setup_generate_config}" != "true" ]];then
			return
		else
			info "Force generate config due to '-f'"
		fi
	else
		info "kernel path does not exist, will download kernel"
		download_kernel="true"
		[ -n "$kernel_version" ] || die "failed to get kernel version: Kernel version is emtpy"

		if [[ ${download_kernel} == "true" ]]; then
			if [ -z "${kernel_ref}" ]; then
				get_kernel "${kernel_version}" "${kernel_path}"
			else
				get_git_kernel "${kernel_version}" "${kernel_path}"
			fi
		fi

		[ -n "$kernel_path" ] || die "failed to find kernel source path"
	fi

	# GPU vendor == NVIDIA: Clone the open-source NVIDIA GPU kernel modules into the
	# kernel tree at drivers/gpu/nvidia/. These modules will be built as built-in (obj-y)
	# rather than loadable modules (obj-m) since the Kata guest kernel is minimal and
	# does not support module loading. The driver uses PCI device matching, so it
	# gracefully does nothing when no NVIDIA GPU is present in the VM.
	if [[ "${gpu_vendor}" == "${VENDOR_NVIDIA}" ]]; then
		kernel_nvidia_path="${kernel_path}/drivers/gpu/nvidia"
		mkdir -p ${kernel_nvidia_path}
		pushd ${kernel_nvidia_path} >> /dev/null
		info "Checking out open-gpu-kernel-modules repo"
		GIT_TERMINAL_PROMPT=0 git clone -b 590.48.01.Z --single-branch --depth 1 \
			https://github.com/zvonkok/open-gpu-kernel-modules.git .

		# Apply in-tree kernel build fixes for kernel 6.x
		info "Applying in-tree build patches for NVIDIA driver"

		# 1. Create kernel 6.x compatibility header to OVERRIDE conftest's wrong detections
		# This is included AFTER conftest.h so we use #undef then #define
		cat > kernel-open/common/inc/nv-kernel-6x-compat.h << 'NVCOMPAT'
/*
 * Kernel 6.x In-Tree Build Compatibility Header
 *
 * This header is included AFTER conftest.h to override incorrect detections.
 * Conftest runs compile tests that may fail in the in-tree build environment,
 * so we provide the correct values for kernel 6.x here.
 */
#ifndef _NV_KERNEL_6X_COMPAT_H_
#define _NV_KERNEL_6X_COMPAT_H_

#include <linux/version.h>

#if LINUX_VERSION_CODE >= KERNEL_VERSION(6, 0, 0)

/* Override conftest detections with correct kernel 6.x values */

/* of_dma_configure has 3 args since 4.18 */
#undef NV_OF_DMA_CONFIGURE_ARGUMENT_COUNT
#define NV_OF_DMA_CONFIGURE_ARGUMENT_COUNT 3

/* DMA APIs removed/privatized in kernel 5.17+ */
#undef NV_DMA_IS_DIRECT_PRESENT
#undef NV_PHYS_TO_DMA_PRESENT
#undef NV_IS_EXPORT_SYMBOL_PRESENT_swiotlb_map_sg_attrs
#define NV_IS_EXPORT_SYMBOL_PRESENT_swiotlb_map_sg_attrs 0
#undef NV_IS_EXPORT_SYMBOL_PRESENT_swiotlb_dma_ops
#define NV_IS_EXPORT_SYMBOL_PRESENT_swiotlb_dma_ops 0

/* Memory encryption - these ARE available in kernel 6.x */
#undef NV_IS_EXPORT_SYMBOL_GPL_set_memory_encrypted
#define NV_IS_EXPORT_SYMBOL_GPL_set_memory_encrypted 0
#undef NV_IS_EXPORT_SYMBOL_GPL_set_memory_decrypted
#define NV_IS_EXPORT_SYMBOL_GPL_set_memory_decrypted 0

/* Refcount APIs - available in kernel 6.x */
#undef NV_IS_EXPORT_SYMBOL_GPL_refcount_inc
#define NV_IS_EXPORT_SYMBOL_GPL_refcount_inc 1
#undef NV_IS_EXPORT_SYMBOL_GPL_refcount_dec_and_test
#define NV_IS_EXPORT_SYMBOL_GPL_refcount_dec_and_test 1

/* timer_delete_sync available in kernel 6.x */
#undef NV_IS_EXPORT_SYMBOL_PRESENT_timer_delete_sync
#define NV_IS_EXPORT_SYMBOL_PRESENT_timer_delete_sync 1

/* These should already be correct from compile tests, but ensure they are */
#ifndef NV_MM_HAS_MMAP_LOCK
#define NV_MM_HAS_MMAP_LOCK
#endif
#ifndef NV_VM_FAULT_T_IS_PRESENT
#define NV_VM_FAULT_T_IS_PRESENT
#endif
#ifndef NV_VM_FLAGS_SET_PRESENT
#define NV_VM_FLAGS_SET_PRESENT
#endif
#ifndef NV_PROC_OPS_PRESENT
#define NV_PROC_OPS_PRESENT
#endif
#ifndef NV_KTIME_GET_RAW_TS64_PRESENT
#define NV_KTIME_GET_RAW_TS64_PRESENT
#endif

/* set_memory_array_uc/wb removed in kernel 6.x - force fallback path */
#undef NV_SET_MEMORY_ARRAY_UC_PRESENT

#endif /* LINUX_VERSION_CODE >= 6.0.0 */
#endif /* _NV_KERNEL_6X_COMPAT_H_ */
NVCOMPAT

		# 2. Fix nv_stdarg.h - use linux/stdarg.h for kernel builds
		cat > kernel-open/common/inc/nv_stdarg.h << 'NVSTDARG'
/*
 * SPDX-FileCopyrightText: Copyright (c) 2021 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: MIT
 */
#ifndef _NV_STDARG_H_
#define _NV_STDARG_H_

#if defined(NV_KERNEL_INTERFACE_LAYER) && defined(NV_LINUX)
  #include <linux/stdarg.h>
#else
  #include <stdarg.h>
#endif

#endif
NVSTDARG

		# 3. Include compat header at the END of conftest.h so ALL files get correct overrides
		# This is cleaner than patching individual files (nv-linux.h, nv-time.h, nv-mm.h, nvidia-drm-conftest.h)
		sed -i '/#endif/i\
\
/* Include kernel 6.x compatibility overrides */\
#include "nv-kernel-6x-compat.h"' kernel-open/common/inc/conftest.h

		# 4. Fix Kbuild for in-tree builds
		sed -i '/^NV_BUILD_TYPE ?= release/a\
NV_INTREE_BUILD := 1' kernel-open/Kbuild
		sed -i 's/BUILD_SANITY_CHECKS += module_symvers_sanity_check/# Skipped: module_symvers_sanity_check/' kernel-open/Kbuild

		# 5. conftest.sh - no changes needed
		# Compile tests work correctly; symbol checks default to "not present"
		# when Module.symvers is missing, which is correct for kernel 6.x

		# 6. Fix autoconf.h paths - modern kernels use generated/autoconf.h
		sed -i 's|#include <linux/autoconf.h>|#include <generated/autoconf.h>|' kernel-open/nvidia/internal_crypt_lib.h
		sed -i 's|#include <linux/autoconf.h>|#include <generated/autoconf.h>|' kernel-open/common/inc/nv-linux.h

		# === DIRECT CODE FIXES for code WITHOUT proper #ifdef guards ===

		# 7. nv-dma.c: get_dma_ops() was privatized in kernel 5.17+
		python3 -c "
with open('kernel-open/nvidia/nv-dma.c', 'r') as f:
    c = f.read()
old = 'const struct dma_map_ops *ops = get_dma_ops(dma_dev->dev);'
new = '#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 17, 0)\n    return NV_FALSE;\n#else\n    const struct dma_map_ops *ops = get_dma_ops(dma_dev->dev);'
c = c.replace(old, new)
c = c.replace('return (ops->map_resource != NULL);', 'return (ops->map_resource != NULL);\n#endif')
with open('kernel-open/nvidia/nv-dma.c', 'w') as f:
    f.write(c)
"
		# 8. nv-linux.h: nv_dma_maps_swiotlb - comment out get_dma_ops for kernel 5.17+
		# For kernel 5.17+, get_dma_ops was privatized, so we disable swiotlb detection
		sed -i 's/const struct dma_map_ops \*ops __attribute__.*= get_dma_ops(dev);/const struct dma_map_ops *ops __attribute__ ((unused)) = NULL; \/\* get_dma_ops removed 5.17+ *\//' kernel-open/common/inc/nv-linux.h

		# 10. os-interface.c: struct timespec removed in kernel 6.x - use timespec64
		sed -i 's/struct timespec ts;/struct timespec64 ts;/' kernel-open/nvidia/os-interface.c
		sed -i 's/getnstimeofday(&ts);/ktime_get_real_ts64(\&ts);/' kernel-open/nvidia/os-interface.c


		# 9. nv-backlight.c: get_backlight_device_by_name renamed to backlight_device_get_by_name
		sed -i 's/get_backlight_device_by_name/backlight_device_get_by_name/g' kernel-open/nvidia/nv-backlight.c


		# 12. nv-caps-imex.c: class_create signature changed in kernel 6.4 (removed THIS_MODULE)
		sed -i 's/class_create(THIS_MODULE,/class_create(/' kernel-open/nvidia/nv-caps-imex.c
		# Fix devnode callback signature (const struct device* in 6.x)
		python3 -c "
with open('kernel-open/nvidia/nv-caps-imex.c', 'r') as f:
    c = f.read()
import re
# Match various whitespace patterns
c = re.sub(r'(char\s*\*\s*nv_caps_imex_devnode\s*\()struct device', r'\1const struct device', c)
with open('kernel-open/nvidia/nv-caps-imex.c', 'w') as f:
    f.write(c)
"

		# 13. uvm_linux.h: sg_dma_page_iter now defined in kernel - wrap in #if 0
		python3 << 'UVMPATCH'
with open('kernel-open/nvidia-uvm/uvm_linux.h', 'r') as f:
    c = f.read()
old = """    struct sg_dma_page_iter {
        struct sg_page_iter base;
    };"""
new = """#if 0 /* sg_dma_page_iter defined in kernel 6.x */
    struct sg_dma_page_iter {
        struct sg_page_iter base;
    };
#endif"""
c = c.replace(old, new)
with open('kernel-open/nvidia-uvm/uvm_linux.h', 'w') as f:
    f.write(c)
UVMPATCH

		# 14. nv-pci.c: iommu_dev_enable/disable_feature removed - replace with 0
		sed -i 's|iommu_dev_enable_feature(nvl->dev, IOMMU_DEV_FEAT_SVA)|0|' kernel-open/nvidia/nv-pci.c
		sed -i 's|iommu_dev_disable_feature(nvl->dev, IOMMU_DEV_FEAT_SVA)|0|' kernel-open/nvidia/nv-pci.c

		# 15. os-interface.c: timespec APIs - use 64-bit versions
		sed -i 's/jiffies_to_timespec(/jiffies_to_timespec64(/g' kernel-open/nvidia/os-interface.c
		sed -i 's/timespec_to_ns(/timespec64_to_ns(/g' kernel-open/nvidia/os-interface.c

		# 16. os-interface.c: add_memory_driver_managed needs MHP_NONE flag
		sed -i 's/add_memory_driver_managed(node, segment_base, segment_size, "System RAM (NVIDIA)")/add_memory_driver_managed(node, segment_base, segment_size, "System RAM (NVIDIA)", MHP_NONE)/' kernel-open/nvidia/os-interface.c

		# 17. nv-vm.c: add set_memory.h include
		python3 -c "
with open('kernel-open/nvidia/nv-vm.c', 'r') as f:
    c = f.read()
if '#include <asm/set_memory.h>' not in c:
    c = c.replace('#include \"nv-linux.h\"', '#include \"nv-linux.h\"\n#include <asm/set_memory.h>', 1)
with open('kernel-open/nvidia/nv-vm.c', 'w') as f:
    f.write(c)
"

		# 18. os-mlock.c: follow_pfn removed - return error
		sed -i 's|return follow_pfn(vma, address, pfn);|return -EINVAL;|' kernel-open/nvidia/os-mlock.c

		# 19. uvm_gpu.c: dma_to_phys not exported - use direct cast
		sed -i 's/dma_to_phys(&gpu->parent->pci_dev->dev, (dma_addr_t)address.address)/(phys_addr_t)address.address/' kernel-open/nvidia-uvm/uvm_gpu.c


		# 20. uvm_populate_pageable.c: handle_mm_fault has 4 args in kernel 6.x (added regs arg)
		python3 -c "
with open('kernel-open/nvidia-uvm/uvm_populate_pageable.c', 'r') as f:
    c = f.read()
# Change handle_mm_fault(vma, addr, flags) to handle_mm_fault(vma, addr, flags, NULL)
c = c.replace('handle_mm_fault(vma, addr, flags)', 'handle_mm_fault(vma, addr, flags, NULL)')
with open('kernel-open/nvidia-uvm/uvm_populate_pageable.c', 'w') as f:
    f.write(c)
"

		# 21. uvm_linux.h: fix uvm_sg_page_iter_dma_address macro for kernel 6.x
		python3 -c "
with open('kernel-open/nvidia-uvm/uvm_linux.h', 'r') as f:
    c = f.read()
# The macro passes &((dma_iter)->base) but should pass dma_iter directly
c = c.replace('sg_page_iter_dma_address(&((dma_iter)->base))', 'sg_page_iter_dma_address(dma_iter)')
with open('kernel-open/nvidia-uvm/uvm_linux.h', 'w') as f:
    f.write(c)
"

		# 22-25. Remove duplicate source files for built-in kernel build
		# nv-kthread-q.c and nv-pci-table.c are symlinked into multiple module dirs
		# For built-in (obj-y), these cause multiple definition errors
		# Keep them only in nvidia/ (main module), remove from others
		info "Removing duplicate source files from sub-module Kbuilds for built-in build"

		# 22. nvidia-modeset: remove nv-kthread-q.c
		sed -i '/NVIDIA_MODESET_SOURCES.*nv-kthread-q.c/d' kernel-open/nvidia-modeset/nvidia-modeset.Kbuild

		# 23. nvidia-drm: remove nv-kthread-q.c and nv-pci-table.c
		sed -i '/NVIDIA_DRM_SOURCES.*nv-kthread-q.c/d' kernel-open/nvidia-drm/nvidia-drm-sources.mk
		sed -i '/NVIDIA_DRM_SOURCES.*nv-pci-table.c/d' kernel-open/nvidia-drm/nvidia-drm-sources.mk

		# 24. nvidia-uvm: remove nv-kthread-q.c (and selftest)
		sed -i '/NVIDIA_UVM_SOURCES.*nv-kthread-q.c/d' kernel-open/nvidia-uvm/nvidia-uvm-sources.Kbuild
		sed -i '/NVIDIA_UVM_SOURCES.*nv-kthread-q-selftest.c/d' kernel-open/nvidia-uvm/nvidia-uvm-sources.Kbuild

		# 25. nvidia-uvm: remove nvstatus.c (conflicts with nv-kernel.o_binary blob)
		sed -i '/NVIDIA_UVM_SOURCES.*nvstatus.c/d' kernel-open/nvidia-uvm/nvidia-uvm-sources.Kbuild

		# 26. nv-backlight.c: backlight_device_get_by_name requires CONFIG_BACKLIGHT_CLASS_DEVICE
		# Undefine NV_GET_BACKLIGHT_DEVICE_BY_NAME_PRESENT so the #else path (NV_ERR_NOT_SUPPORTED) is used
		# Add the undef at the top of the file, after the includes
		sed -i 's|#include "nv-linux.h"|#include "nv-linux.h"\n\n/* Disable backlight - requires CONFIG_BACKLIGHT_CLASS_DEVICE which is not enabled in Kata */\n#undef NV_GET_BACKLIGHT_DEVICE_BY_NAME_PRESENT|' kernel-open/nvidia/nv-backlight.c

		# 27. nv-platform.c: Tegra-specific functions don't exist on x86_64
		# Use weak symbols for the external Tegra functions
		python3 << 'TEGRAPATCH'
with open('kernel-open/nvidia/nv-platform.c', 'r') as f:
    c = f.read()

# Add weak stub implementations at the end of file, before the last #endif if any
# These provide fallback implementations when the real Tegra functions aren't linked

stub_code = '''
/* Weak stubs for Tegra functions - used on non-Tegra platforms (x86_64) */
#ifndef CONFIG_ARCH_TEGRA
int __attribute__((weak)) tegra_fuse_control_read(unsigned long addr, unsigned int *data)
{
    return -ENODEV;
}

int __attribute__((weak)) tsec_comms_send_cmd(void *cmd, unsigned int queue_id,
    void (*cb_func)(void *, void *), void *cb_context)
{
    return -ENODEV;
}

int __attribute__((weak)) tsec_comms_set_init_cb(void (*cb_func)(void *, void *), void *cb_context)
{
    return -ENODEV;
}

void __attribute__((weak)) tsec_comms_clear_init_cb(void)
{
}

void * __attribute__((weak)) tsec_comms_alloc_mem_from_gscco(unsigned int size_in_bytes, unsigned int *gscco_offset)
{
    return NULL;
}

void __attribute__((weak)) tsec_comms_free_gscco_mem(void *mem)
{
}
#endif /* !CONFIG_ARCH_TEGRA */
'''

# Append the stubs at the end of the file
c = c.rstrip() + '\n' + stub_code + '\n'

with open('kernel-open/nvidia/nv-platform.c', 'w') as f:
    f.write(c)
TEGRAPATCH

		# 28. uvm_test.c: nv_kthread_q_run_self_test called but we removed the source
		# Stub it out or remove the test
		python3 << 'UVMTESTPATCH'
with open('kernel-open/nvidia-uvm/uvm_test.c', 'r') as f:
    c = f.read()

# Replace the call to nv_kthread_q_run_self_test with NV_OK
c = c.replace('nv_kthread_q_run_self_test()', 'NV_OK')

with open('kernel-open/nvidia-uvm/uvm_test.c', 'w') as f:
    f.write(c)
UVMTESTPATCH

		# 29. Remove -ffunction-sections and -fdata-sections from NVIDIA Makefiles
		# These create per-function/data sections that cause "orphan section" linker warnings
		# when linked into the kernel. For in-tree builds, kernel's DCE handles this.
		sed -i '/^CFLAGS += -ffunction-sections/d' src/nvidia/Makefile
		sed -i '/^CFLAGS += -fdata-sections/d' src/nvidia/Makefile
		sed -i '/^CFLAGS += -ffunction-sections/d' src/nvidia-modeset/Makefile
		sed -i '/^CFLAGS += -fdata-sections/d' src/nvidia-modeset/Makefile

		popd >> /dev/null

		# Hook NVIDIA driver into kernel build (built-in)
		info "Adding NVIDIA to drivers/gpu/Makefile"
		echo "obj-y += nvidia/" >> "${kernel_path}/drivers/gpu/Makefile"
	fi

	get_config_and_patches

	[ -d "${patches_path}" ] || die " patches path '${patches_path}' does not exist"

	local major_kernel
	major_kernel=$(get_major_kernel_version "${kernel_version}")
	local patches_dir_for_version="${patches_path}/${major_kernel}.x"
	local build_type_patches_dir="${patches_path}/${major_kernel}.x/${build_type}"

	[ -n "${arch_target}" ] || arch_target="$(uname -m)"
	arch_target=$(arch_to_kernel "${arch_target}")
	(
	cd "${kernel_path}" || exit 1

	# Apply version specific patches
	${packaging_scripts_dir}/apply_patches.sh "${patches_dir_for_version}"

	# Apply version specific patches for build_type build
	if [ "${build_type}" != "" ] ;then
		info "Apply build_type patches from ${build_type_patches_dir}"
		${packaging_scripts_dir}/apply_patches.sh "${build_type_patches_dir}"
	fi

	[ -n "${hypervisor_target}" ] || hypervisor_target="kvm"
	[ -n "${kernel_config_path}" ] || kernel_config_path=$(get_default_kernel_config "${kernel_version}" "${hypervisor_target}" "${arch_target}" "${kernel_path}")

	if [ "${measured_rootfs}" == "true" ]; then
		check_initramfs_or_die
		info "Copying initramfs from: ${default_initramfs}"
		cp "${default_initramfs}" ./
	fi

	info "Copying config file from: ${kernel_config_path}"
	cp "${kernel_config_path}" ./.config
	ARCH=${arch_target}  make oldconfig ${CROSS_BUILD_ARG}
	)
}

build_kernel() {
	local kernel_path=${1:-}
	[ -n "${kernel_path}" ] || die "kernel_path not provided"
	[ -d "${kernel_path}" ] || die "path to kernel does not exist, use ${script_name} setup"
	[ -n "${arch_target}" ] || arch_target="$(uname -m)"
	arch_target=$(arch_to_kernel "${arch_target}")

	# Build NVIDIA kernel objects before kernel build (if NVIDIA GPU support enabled)
	#
	# The NVIDIA driver has two parts:
	# 1. src/nvidia/ - OS-agnostic Resource Manager, built separately → nv-kernel.o
	# 2. kernel-open/ - Linux kernel interface layer, built by Kbuild
	#
	# The Kbuild expects kernel-open/nvidia/nv-kernel.o_binary to exist.
	# In standalone builds, the top-level Makefile creates this as a symlink.
	# For in-tree kernel builds, we must create the symlink manually since
	# the standalone Makefile rules are skipped (KERNELRELEASE is set).
	#
	if [[ "${gpu_vendor}" == "${VENDOR_NVIDIA}" ]] && [[ -d "${kernel_path}/drivers/gpu/nvidia/src" ]]; then
		local nvidia_path="${kernel_path}/drivers/gpu/nvidia"

		info "Building NVIDIA kernel objects (nv-kernel.o, nv-modeset-kernel.o)"
		make -C "${nvidia_path}/src/nvidia" -j$(nproc)
		make -C "${nvidia_path}/src/nvidia-modeset" -j$(nproc)

		# Localize conflicting symbols in precompiled blobs for built-in kernel
		# nv-modeset-kernel.o contains xz decompression code that conflicts with kernel lib/xz
		# and nvstatusToString that conflicts with nv-kernel.o
		info "Localizing conflicting symbols in NVIDIA blobs for built-in build"
		objcopy \
			--localize-symbol=xz_dec_reset \
			--localize-symbol=xz_dec_init \
			--localize-symbol=xz_dec_run \
			--localize-symbol=xz_dec_end \
			--localize-symbol=xz_dec_lzma2_reset \
			--localize-symbol=xz_dec_lzma2_run \
			--localize-symbol=xz_dec_lzma2_create \
			--localize-symbol=xz_dec_lzma2_end \
			--localize-symbol=nvstatusToString \
			"${nvidia_path}/src/nvidia-modeset/_out/Linux_x86_64/nv-modeset-kernel.o"

		# Create .o_binary symlinks that kernel-open/ Kbuild expects
		# These point to the objects we just built in src/
		info "Creating NVIDIA .o_binary symlinks for Kbuild"
		ln -sf "../../src/nvidia/_out/Linux_x86_64/nv-kernel.o" \
			"${nvidia_path}/kernel-open/nvidia/nv-kernel.o_binary"
		ln -sf "../../src/nvidia-modeset/_out/Linux_x86_64/nv-modeset-kernel.o" \
			"${nvidia_path}/kernel-open/nvidia-modeset/nv-modeset-kernel.o_binary"
	fi

	pushd "${kernel_path}" >>/dev/null
	make -j $(nproc) ARCH="${arch_target}" ${CROSS_BUILD_ARG}
	if [ "${conf_guest}" == "confidential" ]; then
		make -j $(nproc) INSTALL_MOD_STRIP=1 INSTALL_MOD_PATH=${kernel_path} modules_install
	fi
	[ "$arch_target" != "powerpc" ] && ([ -e "arch/${arch_target}/boot/bzImage" ] || [ -e "arch/${arch_target}/boot/Image.gz" ])
	[ -e "vmlinux" ]
	([ "${hypervisor_target}" == "firecracker" ] || [ "${hypervisor_target}" == "cloud-hypervisor" ]) && [ "${arch_target}" == "arm64" ] && [ -e "arch/${arch_target}/boot/Image" ]
	popd >>/dev/null
}

build_kernel_headers() {
	local kernel_path=${1:-}
	[ -n "${kernel_path}" ] || die "kernel_path not provided"
	[ -d "${kernel_path}" ] || die "path to kernel does not exist, use ${script_name} setup"
	[ -n "${arch_target}" ] || arch_target="$(uname -m)"
	arch_target=$(arch_to_kernel "${arch_target}")
	pushd "${kernel_path}" >>/dev/null

	if [ "$linux_headers" == "deb" ]; then
		export KBUILD_BUILD_USER="${USER}"
		make -j $(nproc) bindeb-pkg ARCH="${arch_target}"
	fi
	if [ "$linux_headers" == "rpm" ]; then
		make -j $(nproc) rpm-pkg ARCH="${arch_target}"
	fi
	# If we encrypt the key earlier it will break the kernel_headers build.
	# At this stage the kernel has created the certs/signing_key.pem
	# encrypt it for later usage in another job or out-of-tree build
	# only encrypt if we have KBUILD_SIGN_PIN set
	local key="certs/signing_key.pem"
	if [ -n "${KBUILD_SIGN_PIN}" ]; then
		[ -e "${key}" ] || die "${key} missing but KBUILD_SIGN_PIN is set"
		openssl rsa -aes256 -in ${key} -out ${key} -passout env:KBUILD_SIGN_PIN
	fi

	popd >>/dev/null
}

install_kata() {
	local kernel_path=${1:-}
	[ -n "${kernel_path}" ] || die "kernel_path not provided"
	[ -d "${kernel_path}" ] || die "path to kernel does not exist, use ${script_name} setup"
	[ -n "${arch_target}" ] || arch_target="$(uname -m)"
	arch_target=$(arch_to_kernel "${arch_target}")
	pushd "${kernel_path}" >>/dev/null
	config_version=$(get_config_version)
	[ -n "${config_version}" ] || die "failed to get config version"
	install_path=$(readlink -m "${DESTDIR}/${PREFIX}/share/${project_name}")

	suffix=""
	if [[ ${build_type} != "" ]]; then
		suffix="-${build_type}"
	fi

	if [[ ${conf_guest} != "" ]];then
		suffix="-${conf_guest}${suffix}"
	fi

	if [[ ${gpu_vendor} != "" ]];then
		suffix="-${gpu_vendor}-gpu${suffix}"
	fi

	vmlinuz="vmlinuz-${kernel_version}-${config_version}${suffix}"
	vmlinux="vmlinux-${kernel_version}-${config_version}${suffix}"

	if [ -e "arch/${arch_target}/boot/bzImage" ]; then
		bzImage="arch/${arch_target}/boot/bzImage"
	elif [ -e "arch/${arch_target}/boot/Image.gz" ]; then
		bzImage="arch/${arch_target}/boot/Image.gz"
	elif [ "${arch_target}" != "powerpc" ]; then
		die "failed to find image"
	fi

	# Install compressed kernel
	if [ "${arch_target}" = "powerpc" ]; then
		install --mode 0644 -D "vmlinux" "${install_path}/${vmlinuz}"
	else
		install --mode 0644 -D "${bzImage}" "${install_path}/${vmlinuz}"
	fi

	# Install uncompressed kernel
	if [ "${arch_target}" = "arm64" ] || [ "${arch_target}" = "riscv" ]; then
		install --mode 0644 -D "arch/${arch_target}/boot/Image" "${install_path}/${vmlinux}"
	elif [ "${arch_target}" = "s390" ]; then
		install --mode 0644 -D "arch/${arch_target}/boot/vmlinux" "${install_path}/${vmlinux}"
	else
		install --mode 0644 -D "vmlinux" "${install_path}/${vmlinux}"
	fi

	install --mode 0644 -D ./.config "${install_path}/config-${kernel_version}-${config_version}${suffix}"

	ln -sf "${vmlinuz}" "${install_path}/vmlinuz${suffix}.container"
	ln -sf "${vmlinux}" "${install_path}/vmlinux${suffix}.container"
	ls -la "${install_path}/vmlinux${suffix}.container"
	ls -la "${install_path}/vmlinuz${suffix}.container"
	popd >>/dev/null
}

main() {
	while getopts "a:b:c:dD:eEfg:hH:k:mp:r:st:u:v:x" opt; do
		case "$opt" in
			a)
				arch_target="${OPTARG}"
				;;
			b)
				build_type="${OPTARG}"
				;;
			c)
				kernel_config_path="${OPTARG}"
				;;
			d)
				PS4=' Line ${LINENO}: '
				set -x
				;;
			D)
				dpu_vendor="${OPTARG}"
				[[ "${dpu_vendor}" == "${VENDOR_NVIDIA}" ]] || die "DPU vendor only support nvidia"
				;;
			e)
				build_type="experimental"
				;;
			E)
				build_type="arch-experimental"
				;;
			f)
				force_setup_generate_config="true"
				;;
			g)
				gpu_vendor="${OPTARG}"
				[[ "${gpu_vendor}" == "${VENDOR_INTEL}" || "${gpu_vendor}" == "${VENDOR_NVIDIA}" ]] || die "GPU vendor only support intel and nvidia"
				;;
			h)
				usage 0
				;;
			H)
				linux_headers="${OPTARG}"
				;;
			m)
				measured_rootfs="true"
				;;
			k)
				kernel_path="$(realpath ${OPTARG})"
				;;
			p)
				patches_path="${OPTARG}"
				;;
			r)
				kernel_ref="${OPTARG}"
				;;
			s)
				skip_config_checks="true"
				;;
			t)
				hypervisor_target="${OPTARG}"
				;;
			u)
				kernel_url="${OPTARG}"
				;;
			v)
				kernel_version="${OPTARG}"
				;;
			x)
				conf_guest="confidential"
				;;
			*)
				echo "ERROR: invalid argument '$opt'"
				exit 1
				;;
		esac
	done

	shift $((OPTIND - 1))

	subcmd="${1:-}"

	[ -z "${subcmd}" ] && usage 1

	if [[ ${build_type} == "experimental" ]] && [[ ${hypervisor_target} == "dragonball" ]]; then
		build_type="dragonball-experimental"
		if [ -n "$kernel_version" ];  then
			kernel_major_version=$(get_major_kernel_version "${kernel_version}")
			if [[ ${kernel_major_version} != "5.10" ]]; then
				info "dragonball-experimental kernel patches are only tested on 5.10.x kernel now, other kernel version may cause confliction"
			fi
		fi
	fi

	# If not kernel version take it from versions.yaml
	if [ -z "$kernel_version" ]; then
		if [[ ${build_type} == "experimental" ]]; then
			kernel_version=$(get_from_kata_deps ".assets.kernel-experimental.tag")
		elif [[ ${build_type} == "arch-experimental" ]]; then
			case "${arch_target}" in
			"aarch64")
				build_type="arm-experimental"
				kernel_version=$(get_from_kata_deps ".assets.kernel-arm-experimental.version")
			;;
			*)
				info "No arch-specific experimental kernel supported, using experimental one instead"
				kernel_version=$(get_from_kata_deps ".assets.kernel-experimental.tag")
			;;
			esac
		elif [[ ${build_type} == "dragonball-experimental" ]]; then
			kernel_version=$(get_from_kata_deps ".assets.kernel-dragonball-experimental.version")
		elif [[ "${conf_guest}" != "" ]]; then
			#If specifying a tag for kernel_version, must be formatted version-like to avoid unintended parsing issues
			kernel_version=$(get_from_kata_deps ".assets.kernel.${conf_guest}.version" 2>/dev/null || true)
			[ -n "${kernel_version}" ] || kernel_version=$(get_from_kata_deps ".assets.kernel.${conf_guest}.tag")
		else
			kernel_version=$(get_from_kata_deps ".assets.kernel.version")
		fi
	fi
	#Remove extra 'v'
	kernel_version="${kernel_version#v}"

	if [ -z "${kernel_path}" ]; then
		config_version=$(get_config_version)
		if [[ ${build_type} != "" ]]; then
			kernel_path="${PWD}/kata-linux-${build_type}-${kernel_version}-${config_version}"
		else
			kernel_path="${PWD}/kata-linux-${kernel_version}-${config_version}"
		fi
		info "Config version: ${config_version}"
	fi

	info "Kernel version: ${kernel_version}"

	[ "${arch_target}" != "" -a "${arch_target}" != $(uname -m) ] && CROSS_BUILD_ARG="CROSS_COMPILE=${arch_target}-linux-gnu-"

	case "${subcmd}" in
		build)
			build_kernel "${kernel_path}"
			;;
		build-headers)
			build_kernel_headers "${kernel_path}"
			;;
		install)
			install_kata "${kernel_path}"
			;;
		setup)
			setup_kernel "${kernel_path}" "${gpu_vendor}"
			[ -d "${kernel_path}" ] || die "${kernel_path} does not exist"
			echo "Kernel source ready: ${kernel_path} "
			;;
		*)
			usage 1
			;;

	esac
}

main "$@"
