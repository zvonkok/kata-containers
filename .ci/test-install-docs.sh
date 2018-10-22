#!/bin/bash
#
# Copyright (c) 2018 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -e

# The go binary isn't installed, but we checkout the repos to the standard
# golang locations.
export GOPATH=${GOPATH:-${HOME}/go}

typeset -r docker_image="busybox"
typeset -r kata_project_url="github.com/kata-containers"
typeset -r test_repo="${kata_project_url}/tests"
typeset -r test_repo_url="https://${test_repo}"
typeset -r test_repo_dir="${GOPATH}/src/${test_repo}"
typeset -r kata_project_dir="${GOPATH}/src/${kata_project_url}"

typeset -r mgr="${test_repo_dir}/cmd/kata-manager/kata-manager.sh"

die()
{
	local msg="$*"
	echo >&2 "ERROR: $msg"
	exit 1
}

info()
{
	local msg="$*"
	echo "INFO: $msg"
}

# Grab a copy of the tests repository
get_tests_repo()
{
	[ -d "${test_repo_dir}" ] && return

	mkdir -p "${kata_project_dir}"

	git clone "${test_repo_url}" "${test_repo_dir}"
}

setup()
{
	source /etc/os-release || source /usr/lib/os-release

	mkdir -p "${GOPATH}"

	get_tests_repo

	[ -e "$mgr" ] || die "cannot find $mgr"
}

# Perform a simple test to create a container
create_kata_container()
{
	local -r test_name="$1"

	local -r msg=$(info "Successfully tested ${test_name} on distro ${ID} ${VERSION}")

	# Perform a basic test
	sudo -E docker run --rm -i --runtime "kata-runtime" "${docker_image}" echo "$msg"
}

# Run the kata manager to "execute" the install guide to ensure the commands
# it specified result in a working system.
test_distro_install_guide()
{
	info "Installing system from the $ID install guide"

	$mgr install-docker-system

	$mgr configure-image
	$mgr enable-debug

	local mgr_name="${mgr##*/}"

	local test_name="${mgr_name} to test install guide"

	info "Install using ${test_name}"

	create_kata_container "${test_name}"
}

run_tests()
{
	test_distro_install_guide
}

# Detect if any installation documents changed. If so, execute all the
# documents to test they result in a working system.
check_install_docs()
{
	if [ -n "$TRAVIS" ]
	then
		info "Not testing install guide as Travis lacks modern distro support and VT-x"
		return
	fi

	# List of filters used to restrict the types of file changes.
	# See git-diff-tree(1) for further info.
	local filters=""

	# Added file
	filters+="A"

	# Copied file
	filters+="C"

	# Modified file
	filters+="M"

	# Renamed file
	filters+="R"

	# Unmerged (U) and Unknown (X) files. These particular filters
	# shouldn't be necessary but just in case...
	filters+="UX"

	# List of changed files
	local files=$(git diff-tree \
		--name-only \
		--no-commit-id \
		--diff-filter="${filters}" \
		-r \
		origin/master HEAD || true)

	# No files were changed
	[ -z "$files" ] && return

	changed=$(echo "${files}" | grep "^install/.*\.md$" || true)

	[ -z "$changed" ] && info "No install documents modified" && return

	info "Found modified install documents: ${changed}"

	# Regardless of which installation documents were changed, we test
	# them all where possible.
	run_tests
}

setup
check_install_docs
