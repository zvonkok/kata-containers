// Copyright (c) 2017 Intel Corporation
// Copyright (c) 2018 HyperHQ Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package containerdshim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/mount"
	cdshim "github.com/containerd/containerd/runtime/v2/shim"
	"github.com/kata-containers/runtime/pkg/katautils"
	vc "github.com/kata-containers/runtime/virtcontainers"
	"github.com/kata-containers/runtime/virtcontainers/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func cReap(s *service, status int, id, execid string, exitat time.Time) {
	s.ec <- exit{
		timestamp: exitat,
		pid:       s.pid,
		status:    status,
		id:        id,
		execid:    execid,
	}
}

func cleanupContainer(ctx context.Context, sid, cid, bundlePath string) error {
	logrus.WithField("Service", "Cleanup").WithField("container", cid).Info("Cleanup container")

	err := vci.CleanupContainer(ctx, sid, cid, true)
	if err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to cleanup container")
		return err
	}

	rootfs := filepath.Join(bundlePath, "rootfs")

	if err := mount.UnmountAll(rootfs, 0); err != nil {
		logrus.WithError(err).WithField("container", cid).Warn("failed to cleanup container rootfs")
		return err
	}

	return nil
}

func validBundle(containerID, bundlePath string) (string, error) {
	// container ID MUST be provided.
	if containerID == "" {
		return "", fmt.Errorf("Missing container ID")
	}

	// bundle path MUST be provided.
	if bundlePath == "" {
		return "", fmt.Errorf("Missing bundle path")
	}

	// bundle path MUST be valid.
	fileInfo, err := os.Stat(bundlePath)
	if err != nil {
		return "", fmt.Errorf("Invalid bundle path '%s': %s", bundlePath, err)
	}
	if !fileInfo.IsDir() {
		return "", fmt.Errorf("Invalid bundle path '%s', it should be a directory", bundlePath)
	}

	resolved, err := katautils.ResolvePath(bundlePath)
	if err != nil {
		return "", err
	}

	return resolved, nil
}

func getAddress(ctx context.Context, bundlePath, id string) (string, error) {
	var err error

	// Checks the MUST and MUST NOT from OCI runtime specification
	if bundlePath, err = validBundle(id, bundlePath); err != nil {
		return "", err
	}

	ociSpec, err := oci.ParseConfigJSON(bundlePath)
	if err != nil {
		return "", err
	}

	containerType, err := ociSpec.ContainerType()
	if err != nil {
		return "", err
	}

	if containerType == vc.PodContainer {
		sandboxID, err := ociSpec.SandboxID()
		if err != nil {
			return "", err
		}
		address, err := cdshim.SocketAddress(ctx, sandboxID)
		if err != nil {
			return "", err
		}
		return address, nil
	}

	return "", nil
}

func noNeedForOutput(detach bool, tty bool) bool {
	if !detach {
		return false
	}

	if !tty {
		return false
	}

	return true
}

func removeNamespace(s *oci.CompatOCISpec, nsType specs.LinuxNamespaceType) {
	for i, n := range s.Linux.Namespaces {
		if n.Type == nsType {
			s.Linux.Namespaces = append(s.Linux.Namespaces[:i], s.Linux.Namespaces[i+1:]...)
			return
		}
	}
}
