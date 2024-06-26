#!/usr/bin/env bash
#
# Copyright (c) 2024 NVIDIA Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

readonly BUILD_DIR="/kata-containers/tools/packaging/kata-deploy/local-build/build/"
# catch errors and then assign
script_dir="$(dirname "$(readlink -f "$0")")"
readonly SCRIPT_DIR="${script_dir}/nvidia"

# This will control how much output the inird/image will produce
DEBUG=""

setup_nvidia-nvrc()
{
	local TARGET="nvidia-nvrc"
	local PROJECT="nvrc"
	local TARGET_BUILD_DIR="${BUILD_DIR}/${TARGET}/builddir"
	local TARGET_DEST_DIR="${BUILD_DIR}/${TARGET}/destdir"
	local TARBALL="${BUILD_DIR}/kata-static-${TARGET}.tar.zst"

	mkdir -p "${TARGET_BUILD_DIR}"
	mkdir -p "${TARGET_DEST_DIR}/bin"

	pushd "${TARGET_BUILD_DIR}" > /dev/null || exit 1

	rm -rf "${PROJECT}"
	git clone https://github.com/zvonkok/${PROJECT}.git

	pushd "${PROJECT}" > /dev/null || exit 1

	cargo build --release --target=x86_64-unknown-linux-musl
	cp target/x86_64-unknown-linux-musl/release/NVRC ../../destdir/bin/.

	popd > /dev/null || exit 1

	tar cvfa "${TARBALL}" -C ../destdir .
	tar tvf  "${TARBALL}"

	popd > /dev/null || exit 1
}

setup_nvidia-gpu-admin-tools()
{
	local TARGET="nvidia-gpu-admin-tools"
	local TARGET_GIT="https://github.com/NVIDIA/gpu-admin-tools"
	local TARGET_BUILD_DIR="${BUILD_DIR}/${TARGET}/builddir"
	local TARGET_DEST_DIR="${BUILD_DIR}/${TARGET}/destdir"
	local TARBALL="${BUILD_DIR}/kata-static-${TARGET}.tar.zst"

	mkdir -p "${TARGET_BUILD_DIR}"
	mkdir -p "${TARGET_DEST_DIR}/sbin"

	pushd "${TARGET_BUILD_DIR}" > /dev/null || exit 1

	rm -rf "$(basename ${TARGET_GIT})"
	git clone ${TARGET_GIT}

	rm -rf dist
	pyinstaller -s -F gpu-admin-tools/nvidia_gpu_tools.py

	cp dist/nvidia_gpu_tools ../destdir/sbin/.

	tar cvfa "${TARBALL}" -C ../destdir .
	tar tvf  "${TARBALL}"

	popd > /dev/null || exit 1
}

setup_nvidia-dcgm-exporter()
{
	local TARGET="nvidia-dcgm-exporter"
	local TARGET_BUILD_DIR="${BUILD_DIR}/${TARGET}/builddir"
	local TARGET_DEST_DIR="${BUILD_DIR}/${TARGET}/destdir"
	local TARBALL="${BUILD_DIR}/kata-static-${TARGET}.tar.zst"

	mkdir -p "${TARGET_BUILD_DIR}"
	mkdir -p "${TARGET_DEST_DIR}/bin"
	mkdir -p "${TARGET_DEST_DIR}/etc"

	pushd "${TARGET_BUILD_DIR}" > /dev/null || exit 1

	local dex="dcgm-exporter"

	rm -rf "${dex}"
	git clone https://github.com/NVIDIA/${dex}
	make -C ${dex} binary

	mkdir -p ../destdir/bin
	mkdir -p ../destdir/etc/${dex}

	cp ${dex}/cmd/${dex}/${dex} ../destdir/bin/.
	cp ${dex}/etc/*.csv ../destdir/etc/${dex}/.

	tar cvfa "${TARBALL}" -C ../destdir .
	tar tvf  "${TARBALL}"

	popd > /dev/null || exit 1
}

setup_nvidia_gpu_rootfs_stage_one()
{
	if [ -e "${BUILD_DIR}/kata-static-nvidia-gpu-rootfs-stage-one.tar.zst" ]; then
		info "nvidia: GPU rootfs stage one already exists"
		return
	fi

	pushd "${ROOTFS_DIR:?}" >> /dev/null

	local rootfs_type=${1:-""}

	info "nvidia: Setup GPU rootfs type=$rootfs_type"

	for component in "nvidia-gpu-admin-tools" "nvidia-dcgm-exporter" "nvidia-nvrc"; do
		if [ ! -e "${BUILD_DIR}/kata-static-${component}.tar.zst" ]; then
			setup_${component}
		fi
	done

	cp "${SCRIPT_DIR}/nvidia_chroot.sh" ./nvidia_chroot.sh

	chmod +x ./nvidia_chroot.sh

	local appendix=""
	if [ "$rootfs_type" == "confidential" ]; then
		appendix="-${rootfs_type}"
	fi

	# We need the kernel packages for building the drivers cleanly will be
	# deinstalled and removed from the roofs once the build finishes.
	tar -xvf ${BUILD_DIR}/kata-static-kernel-nvidia-gpu"${appendix}"-headers.tar.xz -C .

	# If we find a local downloaded run file build the kernel modules
	# with it, otherwise use the distribution packages. Run files may have
	# more recent drivers available then the distribution packages.
	local run_file_name="nvidia-driver.run"
	if [ -f ${BUILD_DIR}/${run_file_name} ]; then
		cp -L ${BUILD_DIR}/${run_file_name} ./${run_file_name}
	fi

	local run_fm_file_name="nvidia-fabricmanager.run"
	if [ -f ${BUILD_DIR}/${run_fm_file_name} ]; then
		cp -L ${BUILD_DIR}/${run_fm_file_name} ./${run_fm_file_name}
	fi

	mount --rbind /dev ./dev
	mount --make-rslave ./dev
	mount -t proc /proc ./proc

	local driver_version="latest"
	if echo "$NVIDIA_STACK_COMPONENTS" | grep -q '\<latest\>'; then
    		driver_version="latest"
	elif echo "$NVIDIA_STACK_COMPONENTS" | grep -q '\<lts\>'; then
		driver_version="lts"
	fi

        chroot . /bin/bash -c "/nvidia_chroot.sh $(uname -r) ${run_file_name} ${run_fm_file_name} ${ARCH} ${driver_version}"

	umount -R ./dev
	umount ./proc

	rm ./nvidia_chroot.sh
	rm ./*.deb

	tar cfa "${BUILD_DIR}"/kata-static-rootfs-nvidia-gpu-stage-one.tar.zst --remove-files *

	popd  >> /dev/null



	pushd "${BUILD_DIR}" >> /dev/null
	curl -LO https://github.com/upx/upx/releases/download/v4.2.4/upx-4.2.4-amd64_linux.tar.xz
	tar xvf upx-4.2.4-amd64_linux.tar.xz
	popd  >> /dev/null

}

chisseled_iptables()
{
	echo "nvidia: chisseling iptables"
	cp -a "${stage_one}"/usr/sbin/xtables-nft-multi sbin/.

	ln -s ../sbin/xtables-nft-multi sbin/iptables-restore
	ln -s ../sbin/xtables-nft-multi sbin/iptables-save

	libdir="lib/x86_64-linux-gnu"
	cp -a "${stage_one}"/${libdir}/libmnl.so.0*      ${libdir}/.
 	cp -a "${stage_one}"/${libdir}/libnftnl.so.11*   ${libdir}/.
 	cp -a "${stage_one}"/${libdir}/libxtables.so.12* ${libdir}/.
}

chisseled_nvswitch()
{
	echo "nvidia: chisseling NVSwitch"
	echo "nvidia: not implemented yet"
	exit 1
}

chisseled_dcgm()
{
	echo "nvidia: chisseling DCGM"

	mkdir -p etc/dcgm-exporter
	libdir="lib/x86_64-linux-gnu"

	cp -a "${stage_one}"/${libdir}/libdcgm.* ${libdir}/.
	cp -a "${stage_one}"/bin/nv-hostengine   bin/.

	tar xvf "${BUILD_DIR}"/kata-static-nvidia-dcgm-exporter.tar.zst -C .
}

# copute always includes utility per default
chisseled_compute()
{
	echo "nvidia: chisseling GPU"

	cp -a "${stage_one}"/nvidia_driver_version .

	tar xvf "${BUILD_DIR}"/kata-static-nvidia-gpu-admin-tools.tar.zst -C .

	cp -a "${stage_one}"/lib/modules/* lib/modules/.

	libdir="lib/x86_64-linux-gnu"
	cp -a "${stage_one}"/${libdir}/libdl.so.2*      ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libz.so.1*       ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libpthread.so.0* ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libresolv.so.2*  ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libc.so.6*       ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libm.so.6*       ${libdir}/.
	cp -a "${stage_one}"/${libdir}/librt.so.1*      ${libdir}/.

	libdir="lib64"
	cp -a "${stage_one}"/${libdir}/ld-linux-x86-64.so.* ${libdir}/.

	tar xvf "${BUILD_DIR}"/kata-static-nvidia-nvrc.tar.zst -C .

	libdir="lib/x86_64-linux-gnu"
	cp -a "${stage_one}"/${libdir}/libnvidia-ml.so.*    ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libcuda.so.*         ${libdir}/.
	cp -a "${stage_one}"/${libdir}/libnvidia-cfg.so.*   ${libdir}/.
	cp -a "${stage_one}"/${libdir}/ld-linux-x86-64.so.* ${libdir}/.

	# basich GPU admin tools
	cp -a "${stage_one}"/bin/nvidia-persistenced bin/.
	cp -a "${stage_one}"/bin/nvidia-smi          bin/.
	cp -a "${stage_one}"/bin/nvidia-ctk          bin/.
}

chisseled_gpudirect()
{
	echo "nvidia: chisseling GPUDirect"
	echo "nvidia: not implemented yet"
	exit 1
}

chisseled_init()
{
	echo "nvidia: chisseling init"
	tar xvf "${BUILD_DIR}"/kata-static-busybox.tar.xz -C .

	mkdir -p dev etc proc run/cdi sys tmp usr var lib/modules lib/firmware \
		 usr/share/nvidia lib/x86_64-linux-gnu lib64

	ln -sf ../run var/run

	cp -a "${stage_one}"/init .
	cp -a "${stage_one}"/sbin/init sbin/.
	cp -a "${stage_one}"/etc/kata-opa etc/.
	cp -a "${stage_one}"/etc/resolv.conf etc/.
	cp -a "${stage_one}"/supported-gpu.devids .

	cp -a "${stage_one}"/lib/firmware/nvidia lib/firmware/.
	cp -a "${stage_one}"/sbin/ldconfig.real sbin/ldconfig

}

compress_rootfs()
{
	echo "nvidia: compressing rootfs"


	# For some unobvious reason libc has executable bit set
	# clean this up otherwise the find -executable will not work correctly
	find . -type f -name "*.so.*" | while IFS= read -r file; do
		chmod -x "${file}"
    		strip "${file}"
	done

	find . -type f -executable | while IFS= read -r file; do
		strip "${file}"
		${BUILD_DIR}/upx-4.2.4-amd64_linux/upx --best --lzma "${file}"
	done
}

toggle_debug()
{
       if echo "$NVIDIA_GPU_STACK" | grep -q '\<debug\>'; then
               export DEBUG="true"
       fi
}

setup_nvidia_gpu_rootfs_stage_two()
{
	readonly stage_one="${BUILD_DIR:?}/rootfs-${VARIANT}-stage-one"
	readonly stage_two="${ROOTFS_DIR:?}"
	readonly stack="${NVIDIA_GPU_STACK:?}"

	echo "nvidia: chisseling the following stack components: $stack"


	[ -e "${stage_one}" ] && rm -rf "${stage_one}"
	[ ! -e "${stage_one}" ] && mkdir -p "${stage_one}"

	tar -C "${stage_one}" -xf ${BUILD_DIR}/kata-static-rootfs-nvidia-gpu-stage-one.tar.zst


	pushd "${stage_two}" >> /dev/null

	toggle_debug
	chisseled_init
	chisseled_iptables

	IFS=',' read -r -a stack_components <<< "$NVIDIA_GPU_STACK"

	for component in "${stack_components[@]}"; do
    		if [ "$component" = "compute" ]; then
        		echo "nvidia: processing \"compute\" component"
        		chisseled_compute
    		elif [ "$component" = "dcgm" ]; then
        		echo "nvidia: processing DCGM component"
        		chisseled_dcgm
    		elif [ "$component" = "nvswitch" ]; then
        		echo "nvidia: processing NVSwitch component"
			chisseled_nvswitch
    		elif [ "$component" = "gpudirect" ]; then
        		echo "nvidia: processing GPUDirect component"
			chisseled_gpudirect
    		fi
	done

	compress_rootfs


	chroot . ldconfig

	popd  >> /dev/null


}
