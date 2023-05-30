#!/bin/bash
set -xe

export DEBIAN_FRONTEND=noninteractive

export uname_r=$1
export run_file_name=$2
export arch_target=$3
export driver_source=""
export driver_version=""

APT_INSTALL="apt -o Dpkg::Options::='--force-confdef' -o Dpkg::Options::='--force-confold' -yqq --no-install-recommends install"


function install_userspace_components() {
	pushd /root/NVIDIA-* >> /dev/null
	./nvidia-installer --no-kernel-modules -s
	popd >> /dev/null

}

function create_udev_rule() {
	cat <<-'CHROOT_EOF' > /etc/udev/rules.d/99-nvidia.rules
		ATTRS{vendor}=="0x10de", DRIVER=="nvidia",  RUN+="/usr/bin/nvidia-ctk cdi generate --output=/var/run/cdi/nvidia.json"
	CHROOT_EOF
}

function cleanup_rootfs() {
	echo "chroot: Cleanup NVIDIA GPU rootfs"
	driver_version=$(apt-cache  search --names-only 'nvidia-headless-no-dkms-' | grep open | tail -n 1 | cut -d' ' -f1 | cut -d'-' -f5)
	apt-mark hold libnvidia-cfg1-${driver_version} libstdc++6 libzstd1 libgnutls30  \
		nvidia-compute-utils-${driver_version} nvidia-utils-${driver_version}        \
		nvidia-kernel-common-${driver_version} libnvidia-compute-${driver_version}   \
		nvidia-modprobe

	linux_headers=$(dpkg --get-selections | cut -f1 | grep linux-headers)
	apt remove make gcc curl gpg software-properties-common \
		linux-libc-dev ${linux_headers}                  \
		nvidia-headless-no-dkms-${driver_version}-open nvidia-kernel-source-${driver_version}-open -yqq

	apt autoremove -yqq

	apt clean
	apt autoclean
	rm -rf /etc/apt/sources.list* /var/lib/apt /usr/share

	# Clear the cache and regenerate the ld cache
	> /etc/ld.so.cache
	ldconfig
}

function install_nvidia_container_runtime() {
	echo "chroot: Installing NVIDIA GPU container runtime"

	eval ${APT_INSTALL} nvidia-container-toolkit

	sed -i "s/#debug/debug/g" /etc/nvidia-container-runtime/config.toml
	sed -i "s/#no-cgroups = false/no-cgroups = true/g" /etc/nvidia-container-runtime/config.toml
	sed -i "/\[nvidia-container-cli\]/a no-pivot = true"  /etc/nvidia-container-runtime/config.toml

	hooks_dir=/etc/oci/hooks.d
	mkdir -p ${hooks_dir}/prestart
	
	cat <<-'CHROOT_EOF' > ${hooks_dir}/prestart/nvidia-container-toolkit.sh
		#!/bin/bash -x
		/usr/bin/nvidia-container-runtime-hook -debug $@
	CHROOT_EOF
	chmod +x ${hooks_dir}/prestart/nvidia-container-toolkit.sh
}

function build_nvidia_drivers() {
	echo "chroot: Build NVIDIA drivers"
	pushd ${driver_source_files} >> /dev/null
	for linux_headers in $(ls /lib/modules/); do
	        echo "chroot: Building GPU modules for: ${linux_headers}"
		cp /boot/System.map-${linux_headers} /lib/modules/${linux_headers}/build/System.map
		
		if [ "${arch_target}" == "aarch64" ]; then
			ln -sf /lib/modules/${linux_headers}/build/arch/arm64 /lib/modules/${linux_headers}/build/arch/aarch64
		fi
	        
		make CC=gcc SYSSRC=/lib/modules/${linux_headers}/build > /dev/null
	        make CC=gcc SYSSRC=/lib/modules/${linux_headers}/build modules_install
	        make CC=gcc SYSSRC=/lib/modules/${linux_headers}/build clean > /dev/null
	done
	popd >> /dev/null
}

function prepare_run_file_drivers() {
	echo "chroot: Prepare NVIDIA run file drivers"
	pushd /root >> /dev/null
	chmod +x ${run_file_name}
	./${run_file_name} -x 
	popd >> /dev/null
}

function prepare_distribution_drivers() {
	driver_version=$(apt-cache  search --names-only 'nvidia-headless-no-dkms-' | grep open | tail -n 1 | cut -d' ' -f1 | cut -d'-' -f5)
	echo "chroot: Prepare NVIDIA distribution drivers"
	eval ${APT_INSTALL} nvidia-headless-no-dkms-${driver_version}-open nvidia-utils-${driver_version}
}

function install_build_dependencies() {
	echo "chroot: Install NVIDIA drivers build dependencies"
	eval ${APT_INSTALL} make gcc kmod libvulkan1
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
	eval ${APT_INSTALL} curl

	distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
	curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
		curl -s -L https://nvidia.github.io/libnvidia-container/experimental/$distribution/libnvidia-container.list | \
        	sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
         	tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
	apt update
}
function install_kernel_dependencies() {
	dpkg -i  /root/linux-*deb
	rm -f    /root/linux-*deb
}

echo "chroot: Setup NVIDIA GPU rootfs"

setup_apt_repositories
install_kernel_dependencies
install_build_dependencies

if [ -f /root/${run_file_name} ]; then 
	prepare_run_file_drivers
	driver_source_files="/root/$(ls /root/ | grep NVIDIA)/kernel-open"
else 
	prepare_distribution_drivers
	driver_source_files="/usr/src/$(ls /usr/src/ | grep ^nvidia)"
fi

build_nvidia_drivers

if [ -f /root/${run_file_name} ]; then 
	install_userspace_components
fi 

install_nvidia_container_runtime

cleanup_rootfs
create_udev_rule 