#!/bin/bash
set -xe

shopt -s nullglob
shopt -s extglob

export DEBIAN_FRONTEND=noninteractive

export uname_r=$1
export run_file_name=$2
export run_fm_file_name=$3
export arch_target=$4
export rootfs_type=$5

export driver_source=""
# For open source drivers driver_type="-open" otherwise driver_type="" 
export driver_version=""
export driver_source_version=""
export driver_type="-open"
export supported_gpu_devids="/supported-gpu.devids"

APT_INSTALL="apt -o Dpkg::Options::='--force-confdef' -o Dpkg::Options::='--force-confold' -yqq --no-install-recommends install"

set_arch() {
	if [[ ${arch_target} == x86_64 ]]; then
        	echo "amd64"
	fi

    	if [[ ${arch_target} == aarch64 ]]; then
	        echo "arm64"
    	fi
}

export ARCH=$(set_arch)

# If we hot-plug we need udev to run the nvidia-ctk CDI files generation
create_udev_rule() 
{	
	set -x
	apt-mark hold udev
	
	mkdir -p /etc/udev/rules.d
	
	cat <<-'CHROOT_EOF' > /bin/hotplug_detected.sh
	#!/bin/bash -x 

	exec &>> /tmp/hotplug_detected.log

	# Path to a file that stores the timestamp of the last GPU plug event
	timestamp_file="/tmp/last_gpu_hotplug_timestamp"

	# Update the timestamp file with the current time
	date +%s > "$timestamp_file"

	# Wait time in seconds before considering the hot-plugging process done
	wait_time=5

	# Schedule the execution of the check script after wait_time seconds
	# It will check if the timestamp has not been updated, indicating no new GPUs were added
	(sleep "$wait_time"; /bin/check_hotplug_activity.sh "$timestamp_file" "$wait_time" &)
	CHROOT_EOF
	
	cat <<-'CHROOT_EOF' > /bin/check_hotplug_activity.sh
	#!/bin/bash -x

	. /nvidia_init_functions

	exec &>> /tmp/check_hotplug_activity.log

	timestamp_file=$1
	wait_time=$2

	# Read the last recorded timestamp
	last_timestamp=$(cat "$timestamp_file")
	current_time=$(date +%s)

	# Calculate the difference
	time_diff=$((current_time - last_timestamp))

	# If the difference is greater than or equal to wait_time, execute the target script
	if [ "$time_diff" -ge "$wait_time" ]; then
	        nvidia_container_toolkit
		nvidia_persistenced
	fi
	CHROOT_EOF

	chmod +x /bin/hotplug_detected.sh
	chmod +x /bin/check_hotplug_activity.sh

	cat <<-'CHROOT_EOF' > /etc/udev/rules.d/99-nvidia.rules
	SUBSYSTEM=="pci", ATTRS{vendor}=="0x10de", DRIVER=="nvidia", RUN+="/bin/hotplug_detected.sh"
	CHROOT_EOF
}

cleanup_rootfs() 
{
	echo "chroot: Cleanup NVIDIA GPU rootfs"

	apt-mark hold libstdc++6 libzstd1 libgnutls30 pciutils

	if [ -n "${driver_version}" ]; then
		apt-mark hold libnvidia-cfg1-"${driver_version}"-server \
			nvidia-compute-utils-"${driver_version}"-server \
			nvidia-utils-"${driver_version}-server"         \
			nvidia-kernel-common-"${driver_version}"-server \
			libnvidia-compute-"${driver_version}"-server   
	fi

	kernel_headers=$(dpkg --get-selections | cut -f1 | grep linux-headers)
	linux_images=$(dpkg --get-selections | cut -f1 | grep linux-image)
	for i in ${kernel_headers} ${linux_images}; do
		apt purge -yqq "${i}"
	done

	apt purge -yqq jq make gcc wget libc6-dev git xz-utils curl gpg \
		python3-pip software-properties-common ca-certificates  \
		linux-libc-dev nuitka python3-minimal
		

	if [ -n "${driver_version}" ]; then
		apt purge -yqq nvidia-headless-no-dkms-"${driver_version}-server${driver_type}" \
			nvidia-kernel-source-"${driver_version}-server${driver_type}" -yqq
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

	dpkg --purge apt

	rm -rf /etc/apt/sources.list* /var/lib/apt /var/log/apt /var/cache/debconf
	rm -f /usr/bin/nvidia-ngx-updater /usr/bin/nvidia-container-runtime
	rm -f /var/log/{nvidia-installer.log,dpkg.log,alternatives.log}


	if [ -e /usr/share/nvidia ]; then 
		mv /usr/share/nvidia /root/usr_share_nvidia
	fi 

	rm -rf /usr/share/*

	if [ -e /root/usr_share_nvidia ]; then 
		mv /root/usr_share_nvidia /usr/share/nvidia
	fi 


	# Clear and regenerate the ld cache
	rm -f /etc/ld.so.cache
	ldconfig

	cp /nvidia_init /init
	mv /lib/modules.save_from_purge /lib/modules

}
install_nvidia_ctk() 
{
	echo "chroot: Installing NVIDIA GPU container runtime"
	# Base  gives a nvidia-ctk and the nvidia-container-runtime 
	#eval "${APT_INSTALL}" nvidia-container-toolkit-base=1.13.2-1
	eval "${APT_INSTALL}" nvidia-container-toolkit-base
}

install_nvidia_fabricmanager() 
{
	# if run_fm_file_name exists run it 
	if [ -f /root/"${run_fm_file_name}" ]; then 
		install_nvidia_fabricmanager_from_run_file
	else
		install_nvidia_fabricmanager_from_distribution
	fi
}

install_nvidia_fabricmanager_from_run_file() 
{
	echo "chroot: Install NVIDIA fabricmanager from run file"
	pushd /root >> /dev/null
	chmod +x "${run_fm_file_name}"
	./"${run_fm_file_name}" --nox11 
	popd >> /dev/null
}

install_nvidia_fabricmanager_from_distribution() 
{
	echo "chroot: Install NVIDIA fabricmanager from distribution"
	eval "${APT_INSTALL}" nvidia-fabricmanager-"${driver_version}"
	apt-mark hold nvidia-fabricmanager-"${driver_version}"
}


OBSOLETE_install_nvidia_verifier_hook() 
{
	if [ "${rootfs_type}" != "confidential" ]; then
		echo "chroot: Skipping NVIDIA verifier hook installation"
		return
	fi

	local hooks_dir=/etc/oci/hooks.d
	mkdir -p ${hooks_dir}/prestart
	
	local hook=${hooks_dir}/prestart/nvidia-verifier-hook.sh
	cat <<-'CHROOT_EOF' > "${hook}"
		#!/bin/bash 

		. /nvidia_init_functions
		script=$(basename "$0" .sh)
		exec &> ${logging_directory}/${script}.log

		nvidia_process_kernel_params "nvidia.attestation.mode"
		nvidia_verifier_hook ${attestation_mode}

	CHROOT_EOF
	chmod +x "${hook}"
}

build_nvidia_drivers() 
{
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

		if [ "${arch_target}" == "x86_64" ]; then
			ln -sf /lib/modules/"${kernel_version}"/build/arch/x86 /lib/modules/"${kernel_version}"/build/arch/amd64
		fi



		make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build > /dev/null
		make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build modules_install
		make -j "$(nproc)" CC=gcc SYSSRC=/lib/modules/"${kernel_version}"/build clean > /dev/null
	done
	# Save the modules for later so that a linux-image purge does not remove it
	mv /lib/modules /lib/modules.save_from_purge

	popd >> /dev/null
}

install_userspace_components() 
{
	if [ ! -f /root/"${run_file_name}" ]; then 
		echo "chroot: Skipping NVIDIA userspace runfile components installation"
		return
	fi 

	pushd /root/NVIDIA-* >> /dev/null
	# if aarch64 we need to remove --no-install-compat32-libs
	if [ "${arch_target}" == "aarch64" ]; then
		./nvidia-installer --no-kernel-modules --no-systemd --no-nvidia-modprobe -s --x-prefix=/root
	else
		./nvidia-installer --no-kernel-modules --no-systemd --no-nvidia-modprobe -s --x-prefix=/root --no-install-compat32-libs 
	fi
	popd >> /dev/null

}

prepare_run_file_drivers() 
{
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

	driver_source_version=$(compgen -G NVIDIA-* | grep -v '.run' | cut -d'-' -f4)

	popd >> /dev/null
}

prepare_distribution_drivers() 
{
	# Latest and greatest
	export driver_version=$(apt-cache search --names-only 'nvidia-headless-no-dkms-.?.?.?-server-open' | awk '{ print $1 }' | tail -n 1 | cut -d'-' -f5)
	# Long term support
	#export driver_version="550"
	
	echo "chroot: Prepare NVIDIA distribution drivers"
	eval "${APT_INSTALL}" nvidia-headless-no-dkms-"${driver_version}-server${driver_type}" nvidia-utils-"${driver_version}"-server
}

prepare_nvidia_drivers() 
{
	local driver_source_dir=""

	if [ -f /root/"${run_file_name}" ]; then 
		prepare_run_file_drivers
		
		for source_dir in /root/NVIDIA-*; do
			if [ -d "${source_dir}" ]; then
				driver_source_files="${source_dir}"/kernel${driver_type}
				driver_source_dir="${source_dir}"
				break
			fi
		done
		get_supported_gpus_from_run_file "${driver_source_dir}"

	else 
		prepare_distribution_drivers
	
		for source_dir in /usr/src/nvidia*; do
			if [ -d "${source_dir}" ]; then
				driver_source_files="${source_dir}"
				driver_source_dir="${source_dir}"
				break
			fi
		done
		get_supported_gpus_from_distro_drivers "${driver_source_dir}"
	fi

}

install_build_dependencies() 
{
	echo "chroot: Install NVIDIA drivers build dependencies"
	eval "${APT_INSTALL}" make gcc kmod libvulkan1 pciutils jq 
}

setup_apt_repositories() 
{
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

install_kernel_dependencies() 
{
	dpkg -i  /root/linux-*deb
	rm -f    /root/linux-*deb
}

get_supported_gpus_from_run_file() 
{
	local source_dir="$1"
	local supported_gpus_json="${source_dir}"/supported-gpus/supported-gpus.json

	jq . < "${supported_gpus_json}"  | grep '"devid"' | awk '{ print $2 }' | tr -d ',"'  > ${supported_gpu_devids}
}

get_supported_gpus_from_distro_drivers() 
{
	local supported_gpus_json=/usr/share/doc/nvidia-kernel-common-"${driver_version}"-server/supported-gpus.json

	jq . < "${supported_gpus_json}"  | grep '"devid"' | awk '{ print $2 }' | tr -d ',"'  > ${supported_gpu_devids}
}

export_driver_version() 
{ 
       for modules_version in /lib/modules.save_from_purge/*; do
               modinfo "${modules_version}"/kernel/drivers/video/nvidia.ko | grep ^version | awk '{ print $2 }' > /nvidia_driver_version
               break
       done
}

dcgm_chisel=(
	"/usr/bin/dcgmi"
	"/usr/bin/dcgmproftester10"
	"/usr/bin/dcgmproftester11"
	"/usr/bin/dcgmproftester12"
	"/usr/share/nvidia-validation-suite"
	"/usr/share/dgcm"
	"/usr/local/dcgm"
	"/usr/include/dcgm_*.h"
	"/usr/bin/DcgmProfTesterKernels.ptx"
	"/usr/lib/systemd/system/dcgm.service"
	"/usr/lib/systemd/system/nvidia-dcgm.service"
	"/usr/lib/x86_64-linux-gnu/libdcgm_cublas_proxy10.so"
	"/usr/lib/x86_64-linux-gnu/libdcgm_cublas_proxy11.so"
	"/usr/lib/x86_64-linux-gnu/libdcgm_cublas_proxy12.so"
	"/usr/lib/x86_64-linux-gnu/cmake"
)

install_nvidia_dcgm() 
{
	curl -O https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2004/x86_64/cuda-keyring_1.0-1_all.deb
	dpkg -i cuda-keyring_1.0-1_all.deb && rm -f cuda-keyring_1.0-1_all.deb

	if [ "${arch_target}" == "aarch64" ]; then
		cat <<-'CHROOT_EOF' > /etc/apt/sources.list.d/cuda.list
			deb [signed-by=/usr/share/keyrings/cuda-archive-keyring.gpg] https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/arm64/ /
		CHROOT_EOF
	else
		cat <<-'CHROOT_EOF' > /etc/apt/sources.list.d/cuda.list
			deb [signed-by=/usr/share/keyrings/cuda-archive-keyring.gpg] https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/ /
		CHROOT_EOF
	fi
	apt update
	eval "${APT_INSTALL}" datacenter-gpu-manager

	for file in "${dcgm_chisel[@]}"; 
	do 
    		rm -rf "$file"
	done

	# chisel the package
}

# Start of script
echo "chroot: Setup NVIDIA GPU rootfs"


setup_apt_repositories
install_kernel_dependencies
install_build_dependencies
prepare_nvidia_drivers
build_nvidia_drivers
install_userspace_components
install_nvidia_fabricmanager
install_nvidia_ctk 
export_driver_version
install_nvidia_dcgm
create_udev_rule
cleanup_rootfs

