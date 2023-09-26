#!/bin/bash
set -xe

shopt -s nullglob
shopt -s extglob

export DEBIAN_FRONTEND=noninteractive

export uname_r=$1
export run_file_name=$2
export arch_target=$3
export driver_source=""
# For open source drivers driver_type="-open" otherwise driver_type="" 
export driver_version=""
export driver_type="-open"
export confidential_compute=true
export supported_gpu_devids="/supported-gpu.devids"

APT_INSTALL="apt -o Dpkg::Options::='--force-confdef' -o Dpkg::Options::='--force-confold' -yqq --no-install-recommends install"

function install_userspace_components() {
	pushd /root/NVIDIA-* >> /dev/null
	./nvidia-installer --no-kernel-modules --no-install-compat32-libs --no-systemd --no-nvidia-modprobe -s
	popd >> /dev/null

}

function regen_apt_cache_multistrap() {
	local multistrap_log=/multistrap.log
	# if the log file does not exist we need to bail out
	if [ ! -f "${multistrap_log}" ]; then
		echo "chroot: ${multistrap_log} file does not exist"
		exit 1
	fi
	eval "${APT_INSTALL}" "$(cat ${multistrap_log})"
}

function create_udev_rule() {
	cat <<-'CHROOT_EOF' > /etc/udev/rules.d/99-nvidia.rules
		ATTRS{vendor}=="0x10de", DRIVER=="nvidia",  RUN+="/usr/bin/nvidia-ctk cdi generate --output=/var/run/cdi/nvidia.json"
	CHROOT_EOF
}

function cleanup_rootfs() {
	echo "chroot: Cleanup NVIDIA GPU rootfs"

	apt-mark hold libstdc++6 libzstd1 libgnutls30 pciutils

	if [ -n "${driver_version}" ]; then
		apt-mark hold libnvidia-cfg1-"${driver_version}" \
			nvidia-compute-utils-"${driver_version}" \
			nvidia-utils-"${driver_version}"         \
			nvidia-kernel-common-"${driver_version}" \
			libnvidia-compute-"${driver_version}"   
	fi

	kernel_headers=$(dpkg --get-selections | cut -f1 | grep linux-headers)
	linux_images=$(dpkg --get-selections | cut -f1 | grep linux-image)
	for i in ${kernel_headers} ${linux_images}; do
		apt purge -yqq "${i}"
	done

	apt purge -yqq jq make gcc git curl gpg software-properties-common \
		python3-pip ca-certificates linux-libc-dev

	if [ -n "${driver_version}" ]; then
		apt purge -yqq nvidia-headless-no-dkms-"${driver_version}${driver_type}" \
			nvidia-kernel-source-"${driver_version}${driver_type}" -yqq
	fi

	apt autoremove -yqq

	apt clean
	apt autoclean

	for modules_version in /lib/modules/*; do
		ln -sf "${modules_version}" /lib/modules/"$(uname -r)"
		touch  "${modules_version}"/modules.order
		touch  "${modules_version}"/modules.builtin
		depmod -a
	done

	rm -rf /etc/apt/sources.list* /var/lib/apt /var/log/apt /var/cache/debconf
	rm -f /usr/bin/nvidia-ngx-updater /usr/bin/nvidia-container-runtime
	rm -f /var/log/{nvidia-installer.log,dpkg.log,alternatives.log}
	# TODO remove complete /usr/share
	
	dpkg --purge apt git

	# Clear and regenerate the ld cache
	rm -f /etc/ld.so.cache
	ldconfig


}

function install_nvidia_container_runtime() {
	echo "chroot: Installing NVIDIA GPU container runtime"

	# Base  gives a nvidia-ctk and the nvidia-container-runtime 
	eval "${APT_INSTALL}" nvidia-container-toolkit-base=1.13.2-1
	# This gives us the nvidia-container-runtime-hook
	eval "${APT_INSTALL}" nvidia-container-toolkit=1.13.2-1

	sed -i "s/#debug/debug/g"                             		/etc/nvidia-container-runtime/config.toml
	sed -i "s|/var/log|/var/log/nvidia-kata-containers|g" 		/etc/nvidia-container-runtime/config.toml
	sed -i "s/#no-cgroups = false/no-cgroups = true/g"    		/etc/nvidia-container-runtime/config.toml
	sed -i "/\[nvidia-container-cli\]/a no-pivot = true"  		/etc/nvidia-container-runtime/config.toml
	sed -i "s/disable-require = false/disable-require = true/g"	/etc/nvidia-container-runtime/config.toml


	local hooks_dir=/etc/oci/hooks.d
	mkdir -p ${hooks_dir}/prestart
	
	local hook=${hooks_dir}/prestart/nvidia-container-runtime-hook.sh
	cat <<-'CHROOT_EOF' > ${hook}
		#!/bin/bash

		. /init-functions
		script=$(basename "$0" .sh)
		exec &> ${logging_directory}/${script}.log

		/usr/bin/nvidia-container-runtime-hook -debug $@ 

	CHROOT_EOF
	chmod +x ${hook}


	local hook=${hooks_dir}/prestart/nvidia-verifier-hook.sh
	cat <<-'CHROOT_EOF' > ${hook}
		#!/bin/bash 

		. /init_functions
		script=$(basename "$0" .sh)
		exec &> ${logging_directory}/${script}.log

		nvidia_process_kernel_params "nvidia.attestation.mode"
		nvidia_verifier_hook ${attestation_mode}

	CHROOT_EOF
	chmod +x ${hook}



}

function build_nvidia_drivers() {
	echo "chroot: Build NVIDIA drivers"
	pushd "${driver_source_files}" >> /dev/null
	
	local kernel_version
	for version in /lib/modules/*; do
		kernel_version=$(basename "${version}")
	        echo "chroot: Building GPU modules for: ${kernel_version}"
		cp /boot/System.map-"${kernel_version}" /lib/modules/"${kernel_version}"/build/System.map
		
		if [ "${arch_target}" == "aarch64" ]; then
			ln -sf /lib/modules/"${kernel_version}"/build/arch/arm64 /lib/modules/"${kernel_version}"/build/arch/aarch64
		fi
	        
		make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build > /dev/null
	        make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build modules_install
	        make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build clean > /dev/null
	done
	popd >> /dev/null
}

function prepare_run_file_drivers() {
	echo "chroot: Prepare NVIDIA run file drivers"
	pushd /root >> /dev/null
	chmod +x "${run_file_name}"
	./"${run_file_name}" -x 

	mkdir -p /usr/share/nvidia/rim/

	# Sooner or later RIM files will be only available remotely
	RIMFILE=$(ls NVIDIA-*/RIM_GH100PROD.swidtag)
	if [ -e "${RIMFILE}" ]; then
		cp NVIDIA-*/RIM_GH100PROD.swidtag /usr/share/nvidia/rim/.
	fi

	#cp NVIDIA-*/RIM_GH100PROD.swidtag /usr/share/nvidia/rim/.

	popd >> /dev/null
}

function prepare_distribution_drivers() {
	# latest and greatest
	driver_version=$(apt-cache  search --names-only 'nvidia-headless-no-dkms-' | grep open | tail -n 1 | cut -d' ' -f1 | cut -d'-' -f5)
	# Long term support
	#export driver_version="525"
	export driver_version
	echo "chroot: Prepare NVIDIA distribution drivers"
	eval "${APT_INSTALL}" nvidia-headless-no-dkms-"${driver_version}${driver_type}" nvidia-utils-"${driver_version}"
}

function install_build_dependencies() {
	echo "chroot: Install NVIDIA drivers build dependencies"
	eval "${APT_INSTALL}" make gcc kmod libvulkan1 pciutils jq
}

function setup_apt_repositories() {
	echo "chroot: Setup APT repositories"
	mkdir -p /var/cache/apt/archives/partial
	mkdir -p /var/log/apt
        mkdir -p /var/lib/dpkg/info
        mkdir -p /var/lib/dpkg/updates
        mkdir -p /var/lib/dpkg/alternatives
        mkdir -p /var/lib/dpkg/triggers
        mkdir -p /var/lib/dpkg/parts
	touch /var/lib/dpkg/status
	rm -f /etc/apt/sources.list.d/*

	if [ "${arch_target}" == "aarch64" ]; then
		cat <<-'CHROOT_EOF' > /etc/apt/sources.list.d/jammy.list
			deb http://ports.ubuntu.com/ubuntu-ports/ jammy main restricted universe multiverse
			deb http://ports.ubuntu.com/ubuntu-ports/ jammy-updates main restricted universe multiverse
			deb http://ports.ubuntu.com/ubuntu-ports/ jammy-security main restricted universe multiverse
		CHROOT_EOF
	else
		cat <<-'CHROOT_EOF' > /etc/apt/sources.list.d/jammy.list
			deb http://archive.ubuntu.com/ubuntu/ jammy main restricted universe multiverse
			deb http://archive.ubuntu.com/ubuntu/ jammy-updates main restricted universe multiverse
			deb http://archive.ubuntu.com/ubuntu/ jammy-security main restricted universe multiverse
		CHROOT_EOF
	fi

	apt update 
	eval "${APT_INSTALL}" curl gpg ca-certificates
	# shellcheck source=/dev/null
	distribution=$(. /etc/os-release;echo "${ID}${VERSION_ID}")
	curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
		curl -s -L https://nvidia.github.io/libnvidia-container/experimental/"${distribution}"/libnvidia-container.list | \
        	sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
         	tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
	apt update
}
function install_kernel_dependencies() {
	dpkg -i  /root/linux-*deb
	rm -f    /root/linux-*deb
}

function install_nvidia_verifier() {
	if [ "${confidential_compute}" == false ]; then 
		echo "chroot: confidential_compute is false, returning"
		return
	fi 

	echo "chroot: Installing NVIDIA GPU Attestation"

	eval "${APT_INSTALL}" python3-minimal python3-pip python3-venv git
	# We need a python to run the NVIDIA verifier
	apt-mark hold python3-minimal

	python3 -m venv  /gpu-attestation
	# shellcheck source=/dev/null
	source /gpu-attestation/bin/activate

	pushd /gpu-attestation >> /dev/null
	git clone https://github.com/NVIDIA/nvtrust.git
	popd >> /dev/null

	pushd /gpu-attestation/nvtrust/guest_tools/gpu_verifiers/local_gpu_verifier >> /dev/null
	pip3 install .
	pip3 install nvidia-ml-py
	popd >> /dev/null

	pushd /gpu-attestation/nvtrust/guest_tools/attestation_sdk/dist >> /dev/null
	pip3 install --no-input ./nv_attestation_sdk-1.1.0-py3-none-any.whl
	popd >> /dev/null

	pushd /gpu-attestation/bin >> /dev/null 
	cp ../nvtrust/guest_tools/attestation_sdk/tests/{LocalGPUTest.py,RemoteGPUTest.py,NVGPULocalPolicyExample.json,NVGPURemotePolicyExample.json} .
	popd >> /dev/null

	pushd /gpu-attestation >> /dev/null
	rm -rf nvtrust
	popd >> /dev/null	
}

function set_confidential_compute() {
	if [ "${confidential_compute}" == false ]; then 
		echo "chroot: confidential_compute is not set, returning"
		return 
	fi
	sed -i "s/export confidential_compute.*/export confidential_compute=true/g" /init_functions
}

function get_supported_gpus_from_run_file() {
	local source_dir="$1"
	local supported_gpus_json="${source_dir}"/supported-gpus/supported-gpus.json

	jq . < "${supported_gpus_json}"  | grep '"devid"' | awk '{ print $2 }' | tr -d ',"'  > ${supported_gpu_devids}
}

function get_supported_gpus_from_distro_drivers() {
	local source_dir="$1"
}

# Start of script
echo "chroot: Setup NVIDIA GPU rootfs"

time { set_confidential_compute; }
time { setup_apt_repositories; }
time { regen_apt_cache_multistrap; }
time { install_kernel_dependencies; }
time { install_build_dependencies; }

if [ -f /root/"${run_file_name}" ]; then 
	time { prepare_run_file_drivers; }

	driver_source_dir=""
	for source_dir in /root/NVIDIA-*; do
		if [ -d "${source_dir}" ]; then
			driver_source_files="${source_dir}"/kernel${driver_type}
			driver_source_dir="${source_dir}"
			break
		fi
	done
	time { get_supported_gpus_from_run_file "${driver_source_dir}"; }
else 
	time { prepare_distribution_drivers; }

	driver_source_dir=""
	for source_dir in /usr/src/nvidia*; do
		if [ -d "${source_dir}" ]; then
			driver_source_files="${source_dir}"
			driver_source_dir="${source_dir}"
			break
		fi
	done
	time { get_supported_gpus_from_distro_drivers "${driver_source_dir}"; }
fi

time build_nvidia_drivers

if [ -f /root/"${run_file_name}" ]; then 
	time install_userspace_components
fi 

time { install_nvidia_container_runtime; }
time { install_nvidia_verifier; }
time { cleanup_rootfs; }
#time create_udev_rule
