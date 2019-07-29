#!/bin/bash
#
# Copyright (c) 2018 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
#
set -e

cidir=$(dirname "$0")
source "${cidir}/lib.sh"

clone_tests_repo

pushd "${tests_repo_dir}"
.ci/setup.sh
popd

bash "${cidir}/static-checks.sh"
# yq needed to correctly parse runtime/versions.yaml
make -C ${tests_repo_dir} install-yq

