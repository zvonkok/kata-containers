// Copyright (c) 2016 Intel Corporation
// Copyright (c) 2020 Adobe Inc.
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/cgroups"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/opencontainers/runc/libcontainer/configs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/kata-containers/kata-containers/src/runtime/pkg/katautils/katatrace"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/device/api"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/device/config"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/device/drivers"
	deviceManager "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/device/manager"
	exp "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/experimental"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/persist"
	persistapi "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/persist/api"
	pbTypes "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/agent/protocols/grpc"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/annotations"
	vccgroups "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/cgroups"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/compatoci"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/cpuset"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/rootless"
	vcTypes "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/types"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/types"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/utils"
)

// sandboxTracingTags defines tags for the trace span
var sandboxTracingTags = map[string]string{
	"source":    "runtime",
	"package":   "virtcontainers",
	"subsystem": "sandbox",
}

const (
	// vmStartTimeout represents the time in seconds a sandbox can wait before
	// to consider the VM starting operation failed.
	vmStartTimeout = 10

	// DirMode is the permission bits used for creating a directory
	DirMode = os.FileMode(0750) | os.ModeDir

	mkswapPath = "/sbin/mkswap"
	rwm        = "rwm"
)

var (
	errSandboxNotRunning = errors.New("Sandbox not running")
)

// SandboxStatus describes a sandbox status.
type SandboxStatus struct {
	ContainersStatus []ContainerStatus

	// Annotations allow clients to store arbitrary values,
	// for example to add additional status values required
	// to support particular specifications.
	Annotations map[string]string

	ID               string
	Hypervisor       HypervisorType
	State            types.SandboxState
	HypervisorConfig HypervisorConfig
}

// SandboxStats describes a sandbox's stats
type SandboxStats struct {
	CgroupStats CgroupStats
	Cpus        int
}

// SandboxConfig is a Sandbox configuration.
type SandboxConfig struct {
	// Volumes is a list of shared volumes between the host and the Sandbox.
	Volumes []types.Volume

	// Containers describe the list of containers within a Sandbox.
	// This list can be empty and populated by adding containers
	// to the Sandbox a posteriori.
	//TODO: this should be a map to avoid duplicated containers
	Containers []ContainerConfig

	// SandboxBindMounts - list of paths to mount into guest
	SandboxBindMounts []string

	// Experimental features enabled
	Experimental []exp.Feature

	// Cgroups specifies specific cgroup settings for the various subsystems that the container is
	// placed into to limit the resources the container has available
	Cgroups *configs.Cgroup

	// Annotations keys must be unique strings and must be name-spaced
	// with e.g. reverse domain notation (org.clearlinux.key).
	Annotations map[string]string

	ID string

	Hostname string

	HypervisorType HypervisorType

	AgentConfig KataAgentConfig

	NetworkConfig NetworkConfig

	HypervisorConfig HypervisorConfig

	ShmSize uint64

	// SharePidNs sets all containers to share the same sandbox level pid namespace.
	SharePidNs bool

	// SystemdCgroup enables systemd cgroup support
	SystemdCgroup bool

	// SandboxCgroupOnly enables cgroup only at podlevel in the host
	SandboxCgroupOnly bool

	DisableGuestSeccomp bool
}

// valid checks that the sandbox configuration is valid.
func (sandboxConfig *SandboxConfig) valid() bool {
	if sandboxConfig.ID == "" {
		return false
	}

	if _, err := newHypervisor(sandboxConfig.HypervisorType); err != nil {
		sandboxConfig.HypervisorType = QemuHypervisor
	}

	// validate experimental features
	for _, f := range sandboxConfig.Experimental {
		if exp.Get(f.Name) == nil {
			return false
		}
	}
	return true
}

// Sandbox is composed of a set of containers and a runtime environment.
// A Sandbox can be created, deleted, started, paused, stopped, listed, entered, and restored.
type Sandbox struct {
	ctx        context.Context
	devManager api.DeviceManager
	factory    Factory
	hypervisor hypervisor
	agent      agent
	store      persistapi.PersistDriver

	swapDevices []*config.BlockDrive
	volumes     []types.Volume

	monitor         *monitor
	config          *SandboxConfig
	annotationsLock *sync.RWMutex
	wg              *sync.WaitGroup
	cgroupMgr       *vccgroups.Manager
	cw              *consoleWatcher

	containers map[string]*Container

	id string

	network Network

	state types.SandboxState

	networkNS NetworkNamespace

	sync.Mutex

	swapSizeBytes int64
	shmSize       uint64
	swapDeviceNum uint

	sharePidNs        bool
	seccompSupported  bool
	disableVMShutdown bool
}

// ID returns the sandbox identifier string.
func (s *Sandbox) ID() string {
	return s.id
}

// Logger returns a logrus logger appropriate for logging Sandbox messages
func (s *Sandbox) Logger() *logrus.Entry {
	return virtLog.WithFields(logrus.Fields{
		"subsystem": "sandbox",
		"sandbox":   s.id,
	})
}

// Annotations returns any annotation that a user could have stored through the sandbox.
func (s *Sandbox) Annotations(key string) (string, error) {
	s.annotationsLock.RLock()
	defer s.annotationsLock.RUnlock()

	value, exist := s.config.Annotations[key]
	if !exist {
		return "", fmt.Errorf("Annotations key %s does not exist", key)
	}

	return value, nil
}

// SetAnnotations sets or adds an annotations
func (s *Sandbox) SetAnnotations(annotations map[string]string) error {
	s.annotationsLock.Lock()
	defer s.annotationsLock.Unlock()

	for k, v := range annotations {
		s.config.Annotations[k] = v
	}
	return nil
}

// GetAnnotations returns sandbox's annotations
func (s *Sandbox) GetAnnotations() map[string]string {
	s.annotationsLock.RLock()
	defer s.annotationsLock.RUnlock()

	return s.config.Annotations
}

// GetNetNs returns the network namespace of the current sandbox.
func (s *Sandbox) GetNetNs() string {
	return s.networkNS.NetNsPath
}

// GetHypervisorPid returns the hypervisor's pid.
func (s *Sandbox) GetHypervisorPid() (int, error) {
	pids := s.hypervisor.getPids()
	if len(pids) == 0 || pids[0] == 0 {
		return -1, fmt.Errorf("Invalid hypervisor PID: %+v", pids)
	}

	return pids[0], nil
}

// GetAllContainers returns all containers.
func (s *Sandbox) GetAllContainers() []VCContainer {
	ifa := make([]VCContainer, len(s.containers))

	i := 0
	for _, v := range s.containers {
		ifa[i] = v
		i++
	}

	return ifa
}

// GetContainer returns the container named by the containerID.
func (s *Sandbox) GetContainer(containerID string) VCContainer {
	if c, ok := s.containers[containerID]; ok {
		return c
	}
	return nil
}

// Release closes the agent connection.
func (s *Sandbox) Release(ctx context.Context) error {
	s.Logger().Info("release sandbox")
	if s.monitor != nil {
		s.monitor.stop()
	}
	s.hypervisor.disconnect(ctx)
	return s.agent.disconnect(ctx)
}

// Status gets the status of the sandbox
func (s *Sandbox) Status() SandboxStatus {
	var contStatusList []ContainerStatus
	for _, c := range s.containers {
		rootfs := c.config.RootFs.Source
		if c.config.RootFs.Mounted {
			rootfs = c.config.RootFs.Target
		}

		contStatusList = append(contStatusList, ContainerStatus{
			ID:          c.id,
			State:       c.state,
			PID:         c.process.Pid,
			StartTime:   c.process.StartTime,
			RootFs:      rootfs,
			Annotations: c.config.Annotations,
		})
	}

	return SandboxStatus{
		ID:               s.id,
		State:            s.state,
		Hypervisor:       s.config.HypervisorType,
		HypervisorConfig: s.config.HypervisorConfig,
		ContainersStatus: contStatusList,
		Annotations:      s.config.Annotations,
	}
}

// Monitor returns a error channel for watcher to watch at
func (s *Sandbox) Monitor(ctx context.Context) (chan error, error) {
	if s.state.State != types.StateRunning {
		return nil, errSandboxNotRunning
	}

	s.Lock()
	if s.monitor == nil {
		s.monitor = newMonitor(s)
	}
	s.Unlock()

	return s.monitor.newWatcher(ctx)
}

// WaitProcess waits on a container process and return its exit code
func (s *Sandbox) WaitProcess(ctx context.Context, containerID, processID string) (int32, error) {
	if s.state.State != types.StateRunning {
		return 0, errSandboxNotRunning
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return 0, err
	}

	return c.wait(ctx, processID)
}

// SignalProcess sends a signal to a process of a container when all is false.
// When all is true, it sends the signal to all processes of a container.
func (s *Sandbox) SignalProcess(ctx context.Context, containerID, processID string, signal syscall.Signal, all bool) error {
	if s.state.State != types.StateRunning {
		return errSandboxNotRunning
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	return c.signalProcess(ctx, processID, signal, all)
}

// WinsizeProcess resizes the tty window of a process
func (s *Sandbox) WinsizeProcess(ctx context.Context, containerID, processID string, height, width uint32) error {
	if s.state.State != types.StateRunning {
		return errSandboxNotRunning
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	return c.winsizeProcess(ctx, processID, height, width)
}

// IOStream returns stdin writer, stdout reader and stderr reader of a process
func (s *Sandbox) IOStream(containerID, processID string) (io.WriteCloser, io.Reader, io.Reader, error) {
	if s.state.State != types.StateRunning {
		return nil, nil, nil, errSandboxNotRunning
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, nil, nil, err
	}

	return c.ioStream(processID)
}

func createAssets(ctx context.Context, sandboxConfig *SandboxConfig) error {
	span, _ := katatrace.Trace(ctx, nil, "createAssets", sandboxTracingTags, map[string]string{"sandbox_id": sandboxConfig.ID})
	defer span.End()

	for _, name := range types.AssetTypes() {
		a, err := types.NewAsset(sandboxConfig.Annotations, name)
		if err != nil {
			return err
		}

		if err := sandboxConfig.HypervisorConfig.addCustomAsset(a); err != nil {
			return err
		}
	}

	_, imageErr := sandboxConfig.HypervisorConfig.assetPath(types.ImageAsset)
	_, initrdErr := sandboxConfig.HypervisorConfig.assetPath(types.InitrdAsset)

	if imageErr != nil && initrdErr != nil {
		return fmt.Errorf("%s and %s cannot be both set", types.ImageAsset, types.InitrdAsset)
	}

	return nil
}

func (s *Sandbox) getAndStoreGuestDetails(ctx context.Context) error {
	guestDetailRes, err := s.agent.getGuestDetails(ctx, &grpc.GuestDetailsRequest{
		MemBlockSize:    true,
		MemHotplugProbe: true,
	})
	if err != nil {
		return err
	}

	if guestDetailRes != nil {
		s.state.GuestMemoryBlockSizeMB = uint32(guestDetailRes.MemBlockSizeBytes >> 20)
		if guestDetailRes.AgentDetails != nil {
			s.seccompSupported = guestDetailRes.AgentDetails.SupportsSeccomp
		}
		s.state.GuestMemoryHotplugProbe = guestDetailRes.SupportMemHotplugProbe
	}

	return nil
}

// createSandbox creates a sandbox from a sandbox description, the containers list, the hypervisor
// and the agent passed through the Config structure.
// It will create and store the sandbox structure, and then ask the hypervisor
// to physically create that sandbox i.e. starts a VM for that sandbox to eventually
// be started.
func createSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (*Sandbox, error) {
	span, ctx := katatrace.Trace(ctx, nil, "createSandbox", sandboxTracingTags, map[string]string{"sandbox_id": sandboxConfig.ID})
	defer span.End()

	if err := createAssets(ctx, &sandboxConfig); err != nil {
		return nil, err
	}

	s, err := newSandbox(ctx, sandboxConfig, factory)
	if err != nil {
		return nil, err
	}

	if len(s.config.Experimental) != 0 {
		s.Logger().WithField("features", s.config.Experimental).Infof("Enable experimental features")
	}

	// Sandbox state has been loaded from storage.
	// If the Stae is not empty, this is a re-creation, i.e.
	// we don't need to talk to the guest's agent, but only
	// want to create the sandbox and its containers in memory.
	if s.state.State != "" {
		return s, nil
	}

	// Below code path is called only during create, because of earlier check.
	if err := s.agent.createSandbox(ctx, s); err != nil {
		return nil, err
	}

	// Set sandbox state
	if err := s.setSandboxState(types.StateReady); err != nil {
		return nil, err
	}

	return s, nil
}

func newSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (sb *Sandbox, retErr error) {
	span, ctx := katatrace.Trace(ctx, nil, "newSandbox", sandboxTracingTags, map[string]string{"sandbox_id": sandboxConfig.ID})
	defer span.End()

	if !sandboxConfig.valid() {
		return nil, fmt.Errorf("Invalid sandbox configuration")
	}

	// create agent instance
	agent := getNewAgentFunc(ctx)()

	hypervisor, err := newHypervisor(sandboxConfig.HypervisorType)
	if err != nil {
		return nil, err
	}

	s := &Sandbox{
		id:              sandboxConfig.ID,
		factory:         factory,
		hypervisor:      hypervisor,
		agent:           agent,
		config:          &sandboxConfig,
		volumes:         sandboxConfig.Volumes,
		containers:      map[string]*Container{},
		state:           types.SandboxState{BlockIndexMap: make(map[int]struct{})},
		annotationsLock: &sync.RWMutex{},
		wg:              &sync.WaitGroup{},
		shmSize:         sandboxConfig.ShmSize,
		sharePidNs:      sandboxConfig.SharePidNs,
		networkNS:       NetworkNamespace{NetNsPath: sandboxConfig.NetworkConfig.NetNSPath},
		ctx:             ctx,
		swapDeviceNum:   0,
		swapSizeBytes:   0,
		swapDevices:     []*config.BlockDrive{},
	}

	hypervisor.setSandbox(s)

	if s.store, err = persist.GetDriver(); err != nil || s.store == nil {
		return nil, fmt.Errorf("failed to get fs persist driver: %v", err)
	}

	defer func() {
		if retErr != nil {
			s.Logger().WithError(retErr).Error("Create new sandbox failed")
			s.store.Destroy(s.id)
		}
	}()

	spec := s.GetPatchedOCISpec()
	if spec != nil && spec.Process.SelinuxLabel != "" {
		sandboxConfig.HypervisorConfig.SELinuxProcessLabel = spec.Process.SelinuxLabel
	}

	s.devManager = deviceManager.NewDeviceManager(sandboxConfig.HypervisorConfig.BlockDeviceDriver,
		sandboxConfig.HypervisorConfig.EnableVhostUserStore,
		sandboxConfig.HypervisorConfig.VhostUserStorePath, nil)

	// Ignore the error. Restore can fail for a new sandbox
	if err := s.Restore(); err != nil {
		s.Logger().WithError(err).Debug("restore sandbox failed")
	}

	// store doesn't require hypervisor to be stored immediately
	if err = s.hypervisor.createSandbox(ctx, s.id, s.networkNS, &sandboxConfig.HypervisorConfig); err != nil {
		return nil, err
	}

	if s.disableVMShutdown, err = s.agent.init(ctx, s, sandboxConfig.AgentConfig); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Sandbox) createCgroupManager() error {
	var err error
	cgroupPath := ""

	// Do not change current cgroup configuration.
	// Create a spec without constraints
	resources := specs.LinuxResources{}

	if s.config == nil {
		return fmt.Errorf("Could not create cgroup manager: empty sandbox configuration")
	}

	spec := s.GetPatchedOCISpec()
	if spec != nil && spec.Linux != nil {
		cgroupPath = spec.Linux.CgroupsPath

		// Kata relies on the cgroup parent created and configured by the container
		// engine by default. The exception is for devices whitelist as well as sandbox-level
		// CPUSet.
		if spec.Linux.Resources != nil {
			resources.Devices = spec.Linux.Resources.Devices

			intptr := func(i int64) *int64 { return &i }
			// Determine if device /dev/null and /dev/urandom exist, and add if they don't
			nullDeviceExist := false
			urandomDeviceExist := false
			for _, device := range resources.Devices {
				if device.Type == "c" && device.Major == intptr(1) && device.Minor == intptr(3) {
					nullDeviceExist = true
				}

				if device.Type == "c" && device.Major == intptr(1) && device.Minor == intptr(9) {
					urandomDeviceExist = true
				}
			}

			if !nullDeviceExist {
				// "/dev/null"
				resources.Devices = append(resources.Devices, []specs.LinuxDeviceCgroup{
					{Type: "c", Major: intptr(1), Minor: intptr(3), Access: rwm, Allow: true},
				}...)
			}
			if !urandomDeviceExist {
				// "/dev/urandom"
				resources.Devices = append(resources.Devices, []specs.LinuxDeviceCgroup{
					{Type: "c", Major: intptr(1), Minor: intptr(9), Access: rwm, Allow: true},
				}...)
			}

			if spec.Linux.Resources.CPU != nil {
				resources.CPU = &specs.LinuxCPU{
					Cpus: spec.Linux.Resources.CPU.Cpus,
				}
			}
		}

		//TODO: in Docker or Podman use case, it is reasonable to set a constraint. Need to add a flag
		// to allow users to configure Kata to constrain CPUs and Memory in this alternative
		// scenario. See https://github.com/kata-containers/runtime/issues/2811
	}

	if s.devManager != nil {
		for _, d := range s.devManager.GetAllDevices() {
			dev, err := vccgroups.DeviceToLinuxDevice(d.GetHostPath())
			if err != nil {
				s.Logger().WithError(err).WithField("device", d.GetHostPath()).Warn("Could not add device to sandbox resources")
				continue
			}
			resources.Devices = append(resources.Devices, dev)
		}
	}

	// Create the cgroup manager, this way it can be used later
	// to create or detroy cgroups
	if s.cgroupMgr, err = vccgroups.New(
		&vccgroups.Config{
			Cgroups:     s.config.Cgroups,
			CgroupPaths: s.state.CgroupPaths,
			Resources:   resources,
			CgroupPath:  cgroupPath,
		},
	); err != nil {
		return err
	}

	return nil
}

// storeSandbox stores a sandbox config.
func (s *Sandbox) storeSandbox(ctx context.Context) error {
	span, _ := katatrace.Trace(ctx, s.Logger(), "storeSandbox", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	// flush data to storage
	if err := s.Save(); err != nil {
		return err
	}
	return nil
}

func rwLockSandbox(sandboxID string) (func() error, error) {
	store, err := persist.GetDriver()
	if err != nil {
		return nil, fmt.Errorf("failed to get fs persist driver: %v", err)
	}

	return store.Lock(sandboxID, true)
}

// findContainer returns a container from the containers list held by the
// sandbox structure, based on a container ID.
func (s *Sandbox) findContainer(containerID string) (*Container, error) {
	if s == nil {
		return nil, vcTypes.ErrNeedSandbox
	}

	if containerID == "" {
		return nil, vcTypes.ErrNeedContainerID
	}

	if c, ok := s.containers[containerID]; ok {
		return c, nil
	}

	return nil, errors.Wrapf(vcTypes.ErrNoSuchContainer, "Could not find the container %q from the sandbox %q containers list",
		containerID, s.id)
}

// removeContainer removes a container from the containers list held by the
// sandbox structure, based on a container ID.
func (s *Sandbox) removeContainer(containerID string) error {
	if s == nil {
		return vcTypes.ErrNeedSandbox
	}

	if containerID == "" {
		return vcTypes.ErrNeedContainerID
	}

	if _, ok := s.containers[containerID]; !ok {
		return errors.Wrapf(vcTypes.ErrNoSuchContainer, "Could not remove the container %q from the sandbox %q containers list",
			containerID, s.id)
	}

	delete(s.containers, containerID)

	return nil
}

// Delete deletes an already created sandbox.
// The VM in which the sandbox is running will be shut down.
func (s *Sandbox) Delete(ctx context.Context) error {
	if s.state.State != types.StateReady &&
		s.state.State != types.StatePaused &&
		s.state.State != types.StateStopped {
		return fmt.Errorf("Sandbox not ready, paused or stopped, impossible to delete")
	}

	for _, c := range s.containers {
		if err := c.delete(ctx); err != nil {
			s.Logger().WithError(err).WithField("container`", c.id).Debug("failed to delete container")
		}
	}

	if !rootless.IsRootless() {
		if err := s.cgroupsDelete(); err != nil {
			s.Logger().WithError(err).Error("failed to cleanup cgroups")
		}
	}

	if s.monitor != nil {
		s.monitor.stop()
	}

	if err := s.hypervisor.cleanup(ctx); err != nil {
		s.Logger().WithError(err).Error("failed to cleanup hypervisor")
	}

	s.agent.cleanup(ctx, s)

	return s.store.Destroy(s.id)
}

func (s *Sandbox) startNetworkMonitor(ctx context.Context) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "startNetworkMonitor", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	binPath, err := os.Executable()
	if err != nil {
		return err
	}

	logLevel := "info"
	if s.config.NetworkConfig.NetmonConfig.Debug {
		logLevel = "debug"
	}

	params := netmonParams{
		netmonPath: s.config.NetworkConfig.NetmonConfig.Path,
		debug:      s.config.NetworkConfig.NetmonConfig.Debug,
		logLevel:   logLevel,
		runtime:    binPath,
		sandboxID:  s.id,
	}

	return s.network.Run(ctx, s.networkNS.NetNsPath, func() error {
		pid, err := startNetmon(params)
		if err != nil {
			return err
		}

		s.networkNS.NetmonPID = pid

		return nil
	})
}

func (s *Sandbox) createNetwork(ctx context.Context) error {
	if s.config.NetworkConfig.DisableNewNetNs ||
		s.config.NetworkConfig.NetNSPath == "" {
		return nil
	}

	span, ctx := katatrace.Trace(ctx, s.Logger(), "createNetwork", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	s.networkNS = NetworkNamespace{
		NetNsPath:    s.config.NetworkConfig.NetNSPath,
		NetNsCreated: s.config.NetworkConfig.NetNsCreated,
	}

	katatrace.AddTag(span, "networkNS", s.networkNS)
	katatrace.AddTag(span, "NetworkConfig", s.config.NetworkConfig)

	// In case there is a factory, network interfaces are hotplugged
	// after vm is started.
	if s.factory == nil {
		// Add the network
		endpoints, err := s.network.Add(ctx, &s.config.NetworkConfig, s, false)
		if err != nil {
			return err
		}

		s.networkNS.Endpoints = endpoints

		if s.config.NetworkConfig.NetmonConfig.Enable {
			if err := s.startNetworkMonitor(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Sandbox) postCreatedNetwork(ctx context.Context) error {

	return s.network.PostAdd(ctx, &s.networkNS, s.factory != nil)
}

func (s *Sandbox) removeNetwork(ctx context.Context) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "removeNetwork", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	if s.config.NetworkConfig.NetmonConfig.Enable {
		if err := stopNetmon(s.networkNS.NetmonPID); err != nil {
			return err
		}
	}

	return s.network.Remove(ctx, &s.networkNS, s.hypervisor)
}

func (s *Sandbox) generateNetInfo(inf *pbTypes.Interface) (NetworkInfo, error) {
	hw, err := net.ParseMAC(inf.HwAddr)
	if err != nil {
		return NetworkInfo{}, err
	}

	var addrs []netlink.Addr
	for _, addr := range inf.IPAddresses {
		netlinkAddrStr := fmt.Sprintf("%s/%s", addr.Address, addr.Mask)
		netlinkAddr, err := netlink.ParseAddr(netlinkAddrStr)
		if err != nil {
			return NetworkInfo{}, fmt.Errorf("could not parse %q: %v", netlinkAddrStr, err)
		}

		addrs = append(addrs, *netlinkAddr)
	}

	return NetworkInfo{
		Iface: NetlinkIface{
			LinkAttrs: netlink.LinkAttrs{
				Name:         inf.Name,
				HardwareAddr: hw,
				MTU:          int(inf.Mtu),
			},
			Type: inf.Type,
		},
		Addrs: addrs,
	}, nil
}

// AddInterface adds new nic to the sandbox.
func (s *Sandbox) AddInterface(ctx context.Context, inf *pbTypes.Interface) (*pbTypes.Interface, error) {
	netInfo, err := s.generateNetInfo(inf)
	if err != nil {
		return nil, err
	}

	endpoint, err := createEndpoint(netInfo, len(s.networkNS.Endpoints), s.config.NetworkConfig.InterworkingModel, nil)
	if err != nil {
		return nil, err
	}

	endpoint.SetProperties(netInfo)
	if err := doNetNS(s.networkNS.NetNsPath, func(_ ns.NetNS) error {
		s.Logger().WithField("endpoint-type", endpoint.Type()).Info("Hot attaching endpoint")
		return endpoint.HotAttach(ctx, s.hypervisor)
	}); err != nil {
		return nil, err
	}

	// Update the sandbox storage
	s.networkNS.Endpoints = append(s.networkNS.Endpoints, endpoint)
	if err := s.Save(); err != nil {
		return nil, err
	}

	// Add network for vm
	inf.PciPath = endpoint.PciPath().String()
	return s.agent.updateInterface(ctx, inf)
}

// RemoveInterface removes a nic of the sandbox.
func (s *Sandbox) RemoveInterface(ctx context.Context, inf *pbTypes.Interface) (*pbTypes.Interface, error) {
	for i, endpoint := range s.networkNS.Endpoints {
		if endpoint.HardwareAddr() == inf.HwAddr {
			s.Logger().WithField("endpoint-type", endpoint.Type()).Info("Hot detaching endpoint")
			if err := endpoint.HotDetach(ctx, s.hypervisor, s.networkNS.NetNsCreated, s.networkNS.NetNsPath); err != nil {
				return inf, err
			}
			s.networkNS.Endpoints = append(s.networkNS.Endpoints[:i], s.networkNS.Endpoints[i+1:]...)

			if err := s.Save(); err != nil {
				return inf, err
			}

			break
		}
	}
	return nil, nil
}

// ListInterfaces lists all nics and their configurations in the sandbox.
func (s *Sandbox) ListInterfaces(ctx context.Context) ([]*pbTypes.Interface, error) {
	return s.agent.listInterfaces(ctx)
}

// UpdateRoutes updates the sandbox route table (e.g. for portmapping support).
func (s *Sandbox) UpdateRoutes(ctx context.Context, routes []*pbTypes.Route) ([]*pbTypes.Route, error) {
	return s.agent.updateRoutes(ctx, routes)
}

// ListRoutes lists all routes and their configurations in the sandbox.
func (s *Sandbox) ListRoutes(ctx context.Context) ([]*pbTypes.Route, error) {
	return s.agent.listRoutes(ctx)
}

const (
	// unix socket type of console
	consoleProtoUnix = "unix"

	// pty type of console.
	consoleProtoPty = "pty"
)

// console watcher is designed to monitor guest console output.
type consoleWatcher struct {
	conn       net.Conn
	ptyConsole *os.File
	proto      string
	consoleURL string
}

func newConsoleWatcher(ctx context.Context, s *Sandbox) (*consoleWatcher, error) {
	var (
		err error
		cw  consoleWatcher
	)

	cw.proto, cw.consoleURL, err = s.hypervisor.getSandboxConsole(ctx, s.id)
	if err != nil {
		return nil, err
	}

	return &cw, nil
}

// start the console watcher
func (cw *consoleWatcher) start(s *Sandbox) (err error) {
	if cw.consoleWatched() {
		return fmt.Errorf("console watcher has already watched for sandbox %s", s.id)
	}

	var scanner *bufio.Scanner

	switch cw.proto {
	case consoleProtoUnix:
		cw.conn, err = net.Dial("unix", cw.consoleURL)
		if err != nil {
			return err
		}
		scanner = bufio.NewScanner(cw.conn)
	case consoleProtoPty:
		// read-only
		cw.ptyConsole, _ = os.Open(cw.consoleURL)
		scanner = bufio.NewScanner(cw.ptyConsole)
	default:
		return fmt.Errorf("unknown console proto %s", cw.proto)
	}

	go func() {
		for scanner.Scan() {
			s.Logger().WithFields(logrus.Fields{
				"console-protocol": cw.proto,
				"console-url":      cw.consoleURL,
				"sandbox":          s.id,
				"vmconsole":        scanner.Text(),
			}).Debug("reading guest console")
		}

		if err := scanner.Err(); err != nil {
			if err == io.EOF {
				s.Logger().Info("console watcher quits")
			} else {
				s.Logger().WithError(err).WithFields(logrus.Fields{
					"console-protocol": cw.proto,
					"console-url":      cw.consoleURL,
					"sandbox":          s.id,
				}).Error("Failed to read guest console logs")
			}
		}
	}()

	return nil
}

// check if the console watcher has already watched the vm console.
func (cw *consoleWatcher) consoleWatched() bool {
	return cw.conn != nil || cw.ptyConsole != nil
}

// stop the console watcher.
func (cw *consoleWatcher) stop() {
	if cw.conn != nil {
		cw.conn.Close()
		cw.conn = nil
	}

	if cw.ptyConsole != nil {
		cw.ptyConsole.Close()
		cw.ptyConsole = nil
	}
}

func (s *Sandbox) addSwap(ctx context.Context, swapID string, size int64) (*config.BlockDrive, error) {
	swapFile := filepath.Join(getSandboxPath(s.id), swapID)

	swapFD, err := os.OpenFile(swapFile, os.O_CREATE, 0600)
	if err != nil {
		err = fmt.Errorf("creat swapfile %s fail %s", swapFile, err.Error())
		s.Logger().WithError(err).Error("addSwap")
		return nil, err
	}
	swapFD.Close()
	defer func() {
		if err != nil {
			os.Remove(swapFile)
		}
	}()

	// Check the size
	pagesize := os.Getpagesize()
	// mkswap refuses areas smaller than 10 pages.
	size = int64(math.Max(float64(size), float64(pagesize*10)))
	// Swapfile need a page to store the metadata
	size += int64(pagesize)

	err = os.Truncate(swapFile, size)
	if err != nil {
		err = fmt.Errorf("truncate swapfile %s fail %s", swapFile, err.Error())
		s.Logger().WithError(err).Error("addSwap")
		return nil, err
	}

	var outbuf, errbuf bytes.Buffer
	cmd := exec.CommandContext(ctx, mkswapPath, swapFile)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("mkswap swapfile %s fail %s stdout %s stderr %s", swapFile, err.Error(), outbuf.String(), errbuf.String())
		s.Logger().WithError(err).Error("addSwap")
		return nil, err
	}

	blockDevice := &config.BlockDrive{
		File:   swapFile,
		Format: "raw",
		ID:     swapID,
		Swap:   true,
	}
	_, err = s.hypervisor.hotplugAddDevice(ctx, blockDevice, blockDev)
	if err != nil {
		err = fmt.Errorf("add swapfile %s device to VM fail %s", swapFile, err.Error())
		s.Logger().WithError(err).Error("addSwap")
		return nil, err
	}
	defer func() {
		if err != nil {
			_, e := s.hypervisor.hotplugRemoveDevice(ctx, blockDevice, blockDev)
			if e != nil {
				s.Logger().Errorf("remove swapfile %s to VM fail %s", swapFile, e.Error())
			}
		}
	}()

	err = s.agent.addSwap(ctx, blockDevice.PCIPath)
	if err != nil {
		err = fmt.Errorf("agent add swapfile %s PCIPath %+v to VM fail %s", swapFile, blockDevice.PCIPath, err.Error())
		s.Logger().WithError(err).Error("addSwap")
		return nil, err
	}

	s.Logger().Infof("add swapfile %s size %d PCIPath %+v to VM success", swapFile, size, blockDevice.PCIPath)

	return blockDevice, nil
}

func (s *Sandbox) removeSwap(ctx context.Context, blockDevice *config.BlockDrive) error {
	err := os.Remove(blockDevice.File)
	if err != nil {
		err = fmt.Errorf("remove swapfile %s fail %s", blockDevice.File, err.Error())
		s.Logger().WithError(err).Error("removeSwap")
	} else {
		s.Logger().Infof("remove swapfile %s success", blockDevice.File)
	}
	return err
}

func (s *Sandbox) setupSwap(ctx context.Context, sizeBytes int64) error {
	if sizeBytes > s.swapSizeBytes {
		dev, err := s.addSwap(ctx, fmt.Sprintf("swap%d", s.swapDeviceNum), sizeBytes-s.swapSizeBytes)
		if err != nil {
			return err
		}

		s.swapDeviceNum += 1
		s.swapSizeBytes = sizeBytes
		s.swapDevices = append(s.swapDevices, dev)
	}

	return nil
}

func (s *Sandbox) cleanSwap(ctx context.Context) {
	for _, dev := range s.swapDevices {
		err := s.removeSwap(ctx, dev)
		if err != nil {
			s.Logger().Warnf("remove swap device %+v got error %s", dev, err)
		}
	}
}

// startVM starts the VM.
func (s *Sandbox) startVM(ctx context.Context) (err error) {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "startVM", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	s.Logger().Info("Starting VM")

	if s.config.HypervisorConfig.Debug {
		// create console watcher
		consoleWatcher, err := newConsoleWatcher(ctx, s)
		if err != nil {
			return err
		}
		s.cw = consoleWatcher
	}

	defer func() {
		if err != nil {
			s.hypervisor.stopSandbox(ctx, false)
		}
	}()

	if err := s.network.Run(ctx, s.networkNS.NetNsPath, func() error {
		if s.factory != nil {
			vm, err := s.factory.GetVM(ctx, VMConfig{
				HypervisorType:   s.config.HypervisorType,
				HypervisorConfig: s.config.HypervisorConfig,
				AgentConfig:      s.config.AgentConfig,
			})
			if err != nil {
				return err
			}

			return vm.assignSandbox(s)
		}

		return s.hypervisor.startSandbox(ctx, vmStartTimeout)
	}); err != nil {
		return err
	}

	// In case of vm factory, network interfaces are hotplugged
	// after vm is started.
	if s.factory != nil {
		endpoints, err := s.network.Add(ctx, &s.config.NetworkConfig, s, true)
		if err != nil {
			return err
		}

		s.networkNS.Endpoints = endpoints

		if s.config.NetworkConfig.NetmonConfig.Enable {
			if err := s.startNetworkMonitor(ctx); err != nil {
				return err
			}
		}
	}

	s.Logger().Info("VM started")

	if s.cw != nil {
		s.Logger().Debug("console watcher starts")
		if err := s.cw.start(s); err != nil {
			s.cw.stop()
			return err
		}
	}

	// Once the hypervisor is done starting the sandbox,
	// we want to guarantee that it is manageable.
	// For that we need to ask the agent to start the
	// sandbox inside the VM.
	if err := s.agent.startSandbox(ctx, s); err != nil {
		return err
	}

	s.Logger().Info("Agent started in the sandbox")

	defer func() {
		if err != nil {
			if e := s.agent.stopSandbox(ctx, s); e != nil {
				s.Logger().WithError(e).WithField("sandboxid", s.id).Warning("Agent did not stop sandbox")
			}
		}
	}()

	return nil
}

// stopVM: stop the sandbox's VM
func (s *Sandbox) stopVM(ctx context.Context) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "stopVM", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	s.Logger().Info("Stopping sandbox in the VM")
	if err := s.agent.stopSandbox(ctx, s); err != nil {
		s.Logger().WithError(err).WithField("sandboxid", s.id).Warning("Agent did not stop sandbox")
	}

	s.Logger().Info("Stopping VM")

	return s.hypervisor.stopSandbox(ctx, s.disableVMShutdown)
}

func (s *Sandbox) addContainer(c *Container) error {
	if _, ok := s.containers[c.id]; ok {
		return fmt.Errorf("Duplicated container: %s", c.id)
	}
	s.containers[c.id] = c

	return nil
}

// CreateContainer creates a new container in the sandbox
// This should be called only when the sandbox is already created.
// It will add new container config to sandbox.config.Containers
func (s *Sandbox) CreateContainer(ctx context.Context, contConfig ContainerConfig) (VCContainer, error) {
	// Update sandbox config to include the new container's config
	s.config.Containers = append(s.config.Containers, contConfig)

	var err error

	defer func() {
		if err != nil {
			if len(s.config.Containers) > 0 {
				// delete container config
				s.config.Containers = s.config.Containers[:len(s.config.Containers)-1]
			}
		}
	}()

	// Create the container object, add devices to the sandbox's device-manager:
	c, err := newContainer(ctx, s, &s.config.Containers[len(s.config.Containers)-1])
	if err != nil {
		return nil, err
	}

	// create and start the container
	if err = c.create(ctx); err != nil {
		return nil, err
	}

	// Add the container to the containers list in the sandbox.
	if err = s.addContainer(c); err != nil {
		return nil, err
	}

	defer func() {
		// Rollback if error happens.
		if err != nil {
			logger := s.Logger().WithFields(logrus.Fields{"container": c.id, "sandbox": s.id, "rollback": true})
			logger.WithError(err).Error("Cleaning up partially created container")

			if errStop := c.stop(ctx, true); errStop != nil {
				logger.WithError(errStop).Error("Could not stop container")
			}

			logger.Debug("Removing stopped container from sandbox store")
			s.removeContainer(c.id)
		}
	}()

	// Sandbox is responsible to update VM resources needed by Containers
	// Update resources after having added containers to the sandbox, since
	// container status is requiered to know if more resources should be added.
	if err = s.updateResources(ctx); err != nil {
		return nil, err
	}

	if err = s.cgroupsUpdate(ctx); err != nil {
		return nil, err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// StartContainer starts a container in the sandbox
func (s *Sandbox) StartContainer(ctx context.Context, containerID string) (VCContainer, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Start it.
	if err = c.start(ctx); err != nil {
		return nil, err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return nil, err
	}

	s.Logger().WithField("container", containerID).Info("Container is started")

	// Update sandbox resources in case a stopped container
	// is started
	if err = s.updateResources(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// StopContainer stops a container in the sandbox
func (s *Sandbox) StopContainer(ctx context.Context, containerID string, force bool) (VCContainer, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Stop it.
	if err := c.stop(ctx, force); err != nil {
		return nil, err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// KillContainer signals a container in the sandbox
func (s *Sandbox) KillContainer(ctx context.Context, containerID string, signal syscall.Signal, all bool) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Send a signal to the process.
	err = c.kill(ctx, signal, all)

	// SIGKILL should never fail otherwise it is
	// impossible to clean things up.
	if signal == syscall.SIGKILL {
		return nil
	}

	return err
}

// DeleteContainer deletes a container from the sandbox
func (s *Sandbox) DeleteContainer(ctx context.Context, containerID string) (VCContainer, error) {
	if containerID == "" {
		return nil, vcTypes.ErrNeedContainerID
	}

	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Delete it.
	if err = c.delete(ctx); err != nil {
		return nil, err
	}

	// Update sandbox config
	for idx, contConfig := range s.config.Containers {
		if contConfig.ID == containerID {
			s.config.Containers = append(s.config.Containers[:idx], s.config.Containers[idx+1:]...)
			break
		}
	}

	// update the sandbox cgroup
	if err = s.cgroupsUpdate(ctx); err != nil {
		return nil, err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// StatusContainer gets the status of a container
func (s *Sandbox) StatusContainer(containerID string) (ContainerStatus, error) {
	if containerID == "" {
		return ContainerStatus{}, vcTypes.ErrNeedContainerID
	}

	if c, ok := s.containers[containerID]; ok {
		rootfs := c.config.RootFs.Source
		if c.config.RootFs.Mounted {
			rootfs = c.config.RootFs.Target
		}

		return ContainerStatus{
			ID:          c.id,
			State:       c.state,
			PID:         c.process.Pid,
			StartTime:   c.process.StartTime,
			RootFs:      rootfs,
			Annotations: c.config.Annotations,
		}, nil
	}

	return ContainerStatus{}, vcTypes.ErrNoSuchContainer
}

// EnterContainer is the virtcontainers container command execution entry point.
// EnterContainer enters an already running container and runs a given command.
func (s *Sandbox) EnterContainer(ctx context.Context, containerID string, cmd types.Cmd) (VCContainer, *Process, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, nil, err
	}

	// Enter it.
	process, err := c.enter(ctx, cmd)
	if err != nil {
		return nil, nil, err
	}

	return c, process, nil
}

// UpdateContainer update a running container.
func (s *Sandbox) UpdateContainer(ctx context.Context, containerID string, resources specs.LinuxResources) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	if err = c.update(ctx, resources); err != nil {
		return err
	}

	if err := s.cgroupsUpdate(ctx); err != nil {
		return err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return err
	}
	return nil
}

// StatsContainer return the stats of a running container
func (s *Sandbox) StatsContainer(ctx context.Context, containerID string) (ContainerStats, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return ContainerStats{}, err
	}

	stats, err := c.stats(ctx)
	if err != nil {
		return ContainerStats{}, err
	}
	return *stats, nil
}

// Stats returns the stats of a running sandbox
func (s *Sandbox) Stats(ctx context.Context) (SandboxStats, error) {
	if s.state.CgroupPath == "" {
		return SandboxStats{}, fmt.Errorf("sandbox cgroup path is empty")
	}

	var path string
	var cgroupSubsystems cgroups.Hierarchy

	if s.config.SandboxCgroupOnly {
		cgroupSubsystems = cgroups.V1
		path = s.state.CgroupPath
	} else {
		cgroupSubsystems = V1NoConstraints
		path = cgroupNoConstraintsPath(s.state.CgroupPath)
	}

	cgroup, err := cgroupsLoadFunc(cgroupSubsystems, cgroups.StaticPath(path))
	if err != nil {
		return SandboxStats{}, fmt.Errorf("Could not load sandbox cgroup in %v: %v", s.state.CgroupPath, err)
	}

	metrics, err := cgroup.Stat(cgroups.ErrorHandler(cgroups.IgnoreNotExist))
	if err != nil {
		return SandboxStats{}, err
	}

	stats := SandboxStats{}

	stats.CgroupStats.CPUStats.CPUUsage.TotalUsage = metrics.CPU.Usage.Total
	stats.CgroupStats.MemoryStats.Usage.Usage = metrics.Memory.Usage.Usage
	tids, err := s.hypervisor.getThreadIDs(ctx)
	if err != nil {
		return stats, err
	}
	stats.Cpus = len(tids.vcpus)

	return stats, nil
}

// PauseContainer pauses a running container.
func (s *Sandbox) PauseContainer(ctx context.Context, containerID string) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Pause the container.
	if err := c.pause(ctx); err != nil {
		return err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return err
	}
	return nil
}

// ResumeContainer resumes a paused container.
func (s *Sandbox) ResumeContainer(ctx context.Context, containerID string) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Resume the container.
	if err := c.resume(ctx); err != nil {
		return err
	}

	if err = s.storeSandbox(ctx); err != nil {
		return err
	}
	return nil
}

// createContainers registers all containers, create the
// containers in the guest and starts one shim per container.
func (s *Sandbox) createContainers(ctx context.Context) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "createContainers", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	for i := range s.config.Containers {

		c, err := newContainer(ctx, s, &s.config.Containers[i])
		if err != nil {
			return err
		}
		if err := c.create(ctx); err != nil {
			return err
		}

		if err := s.addContainer(c); err != nil {
			return err
		}
	}

	// Update resources after having added containers to the sandbox, since
	// container status is requiered to know if more resources should be added.
	if err := s.updateResources(ctx); err != nil {
		return err
	}

	if err := s.cgroupsUpdate(ctx); err != nil {
		return err
	}
	if err := s.storeSandbox(ctx); err != nil {
		return err
	}

	return nil
}

// Start starts a sandbox. The containers that are making the sandbox
// will be started.
func (s *Sandbox) Start(ctx context.Context) error {
	if err := s.state.ValidTransition(s.state.State, types.StateRunning); err != nil {
		return err
	}

	prevState := s.state.State

	if err := s.setSandboxState(types.StateRunning); err != nil {
		return err
	}

	var startErr error
	defer func() {
		if startErr != nil {
			s.setSandboxState(prevState)
		}
	}()
	for _, c := range s.containers {
		if startErr = c.start(ctx); startErr != nil {
			return startErr
		}
	}

	if err := s.storeSandbox(ctx); err != nil {
		return err
	}

	s.Logger().Info("Sandbox is started")

	return nil
}

// Stop stops a sandbox. The containers that are making the sandbox
// will be destroyed.
// When force is true, ignore guest related stop failures.
func (s *Sandbox) Stop(ctx context.Context, force bool) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "Stop", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	if s.state.State == types.StateStopped {
		s.Logger().Info("sandbox already stopped")
		return nil
	}

	if err := s.state.ValidTransition(s.state.State, types.StateStopped); err != nil {
		return err
	}

	for _, c := range s.containers {
		if err := c.stop(ctx, force); err != nil {
			return err
		}
	}

	if err := s.stopVM(ctx); err != nil && !force {
		return err
	}

	// shutdown console watcher if exists
	if s.cw != nil {
		s.Logger().Debug("stop the console watcher")
		s.cw.stop()
	}

	if err := s.setSandboxState(types.StateStopped); err != nil {
		return err
	}

	// Remove the network.
	if err := s.removeNetwork(ctx); err != nil && !force {
		return err
	}

	if err := s.storeSandbox(ctx); err != nil {
		return err
	}

	// Stop communicating with the agent.
	if err := s.agent.disconnect(ctx); err != nil && !force {
		return err
	}

	s.cleanSwap(ctx)

	return nil
}

// setSandboxState sets the in-memory state of the sandbox.
func (s *Sandbox) setSandboxState(state types.StateString) error {
	if state == "" {
		return vcTypes.ErrNeedState
	}

	// update in-memory state
	s.state.State = state

	return nil
}

const maxBlockIndex = 65535

// getAndSetSandboxBlockIndex retrieves an unused sandbox block index from
// the BlockIndexMap and marks it as used. This index is used to maintain the
// index at which a block device is assigned to a container in the sandbox.
func (s *Sandbox) getAndSetSandboxBlockIndex() (int, error) {
	currentIndex := -1
	for i := 0; i < maxBlockIndex; i++ {
		if _, ok := s.state.BlockIndexMap[i]; !ok {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		return -1, errors.New("no available block index")
	}
	s.state.BlockIndexMap[currentIndex] = struct{}{}

	return currentIndex, nil
}

// unsetSandboxBlockIndex deletes the current sandbox block index from BlockIndexMap.
// This is used to recover from failure while adding a block device.
func (s *Sandbox) unsetSandboxBlockIndex(index int) error {
	var err error
	original := index
	delete(s.state.BlockIndexMap, index)
	defer func() {
		if err != nil {
			s.state.BlockIndexMap[original] = struct{}{}
		}
	}()

	return nil
}

// HotplugAddDevice is used for add a device to sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) HotplugAddDevice(ctx context.Context, device api.Device, devType config.DeviceType) error {
	span, ctx := katatrace.Trace(ctx, s.Logger(), "HotplugAddDevice", sandboxTracingTags, map[string]string{"sandbox_id": s.id})
	defer span.End()

	if s.config.SandboxCgroupOnly {
		// We are about to add a device to the hypervisor,
		// the device cgroup MUST be updated since the hypervisor
		// will need access to such device
		hdev := device.GetHostPath()
		if err := s.cgroupMgr.AddDevice(ctx, hdev); err != nil {
			s.Logger().WithError(err).WithField("device", hdev).
				Warn("Could not add device to cgroup")
		}
	}

	switch devType {
	case config.DeviceVFIO:
		vfioDevices, ok := device.GetDeviceInfo().([]*config.VFIODev)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}

		// adding a group of VFIO devices
		for _, dev := range vfioDevices {
			if _, err := s.hypervisor.hotplugAddDevice(ctx, dev, vfioDev); err != nil {
				s.Logger().
					WithFields(logrus.Fields{
						"sandbox":         s.id,
						"vfio-device-ID":  dev.ID,
						"vfio-device-BDF": dev.BDF,
					}).WithError(err).Error("failed to hotplug VFIO device")
				return err
			}
		}
		return nil
	case config.DeviceBlock:
		blockDevice, ok := device.(*drivers.BlockDevice)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}
		_, err := s.hypervisor.hotplugAddDevice(ctx, blockDevice.BlockDrive, blockDev)
		return err
	case config.VhostUserBlk:
		vhostUserBlkDevice, ok := device.(*drivers.VhostUserBlkDevice)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}
		_, err := s.hypervisor.hotplugAddDevice(ctx, vhostUserBlkDevice.VhostUserDeviceAttrs, vhostuserDev)
		return err
	case config.DeviceGeneric:
		// TODO: what?
		return nil
	}
	return nil
}

// HotplugRemoveDevice is used for removing a device from sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) HotplugRemoveDevice(ctx context.Context, device api.Device, devType config.DeviceType) error {
	defer func() {
		if s.config.SandboxCgroupOnly {
			// Remove device from cgroup, the hypervisor
			// should not have access to such device anymore.
			hdev := device.GetHostPath()
			if err := s.cgroupMgr.RemoveDevice(hdev); err != nil {
				s.Logger().WithError(err).WithField("device", hdev).
					Warn("Could not remove device from cgroup")
			}
		}
	}()

	switch devType {
	case config.DeviceVFIO:
		vfioDevices, ok := device.GetDeviceInfo().([]*config.VFIODev)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}

		// remove a group of VFIO devices
		for _, dev := range vfioDevices {
			if _, err := s.hypervisor.hotplugRemoveDevice(ctx, dev, vfioDev); err != nil {
				s.Logger().WithError(err).
					WithFields(logrus.Fields{
						"sandbox":         s.id,
						"vfio-device-ID":  dev.ID,
						"vfio-device-BDF": dev.BDF,
					}).Error("failed to hot unplug VFIO device")
				return err
			}
		}
		return nil
	case config.DeviceBlock:
		blockDrive, ok := device.GetDeviceInfo().(*config.BlockDrive)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}
		// PMEM devices cannot be hot removed
		if blockDrive.Pmem {
			s.Logger().WithField("path", blockDrive.File).Infof("Skip device: cannot hot remove PMEM devices")
			return nil
		}
		_, err := s.hypervisor.hotplugRemoveDevice(ctx, blockDrive, blockDev)
		return err
	case config.VhostUserBlk:
		vhostUserDeviceAttrs, ok := device.GetDeviceInfo().(*config.VhostUserDeviceAttrs)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}
		_, err := s.hypervisor.hotplugRemoveDevice(ctx, vhostUserDeviceAttrs, vhostuserDev)
		return err
	case config.DeviceGeneric:
		// TODO: what?
		return nil
	}
	return nil
}

// GetAndSetSandboxBlockIndex is used for getting and setting virtio-block indexes
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) GetAndSetSandboxBlockIndex() (int, error) {
	return s.getAndSetSandboxBlockIndex()
}

// UnsetSandboxBlockIndex unsets block indexes
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) UnsetSandboxBlockIndex(index int) error {
	return s.unsetSandboxBlockIndex(index)
}

// AppendDevice can only handle vhost user device currently, it adds a
// vhost user device to sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) AppendDevice(ctx context.Context, device api.Device) error {
	switch device.DeviceType() {
	case config.VhostUserSCSI, config.VhostUserNet, config.VhostUserBlk, config.VhostUserFS:
		return s.hypervisor.addDevice(ctx, device.GetDeviceInfo().(*config.VhostUserDeviceAttrs), vhostuserDev)
	case config.DeviceVFIO:
		vfioDevs := device.GetDeviceInfo().([]*config.VFIODev)
		for _, d := range vfioDevs {
			return s.hypervisor.addDevice(ctx, *d, vfioDev)
		}
	default:
		s.Logger().WithField("device-type", device.DeviceType()).
			Warn("Could not append device: unsupported device type")
	}

	return fmt.Errorf("unsupported device type")
}

// AddDevice will add a device to sandbox
func (s *Sandbox) AddDevice(ctx context.Context, info config.DeviceInfo) (api.Device, error) {
	if s.devManager == nil {
		return nil, fmt.Errorf("device manager isn't initialized")
	}

	var err error
	b, err := s.devManager.NewDevice(info)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			s.devManager.RemoveDevice(b.DeviceID())
		}
	}()

	if err = s.devManager.AttachDevice(ctx, b.DeviceID(), s); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			s.devManager.DetachDevice(ctx, b.DeviceID(), s)
		}
	}()

	return b, nil
}

// updateResources will:
// - calculate the resources required for the virtual machine, and adjust the virtual machine
// sizing accordingly. For a given sandbox, it will calculate the number of vCPUs required based
// on the sum of container requests, plus default CPUs for the VM. Similar is done for memory.
// If changes in memory or CPU are made, the VM will be updated and the agent will online the
// applicable CPU and memory.
func (s *Sandbox) updateResources(ctx context.Context) error {
	if s == nil {
		return errors.New("sandbox is nil")
	}

	if s.config == nil {
		return fmt.Errorf("sandbox config is nil")
	}

	sandboxVCPUs, err := s.calculateSandboxCPUs()
	if err != nil {
		return err
	}
	// Add default vcpus for sandbox
	sandboxVCPUs += s.hypervisor.hypervisorConfig().NumVCPUs

	sandboxMemoryByte, sandboxneedPodSwap, sandboxSwapByte := s.calculateSandboxMemory()
	// Add default / rsvd memory for sandbox.
	hypervisorMemoryByte := int64(s.hypervisor.hypervisorConfig().MemorySize) << utils.MibToBytesShift
	sandboxMemoryByte += hypervisorMemoryByte
	if sandboxneedPodSwap {
		sandboxSwapByte += hypervisorMemoryByte
	}
	s.Logger().WithField("sandboxMemoryByte", sandboxMemoryByte).WithField("sandboxneedPodSwap", sandboxneedPodSwap).WithField("sandboxSwapByte", sandboxSwapByte).Debugf("updateResources: after calculateSandboxMemory")

	// Setup the SWAP in the guest
	if sandboxSwapByte > 0 {
		err = s.setupSwap(ctx, sandboxSwapByte)
		if err != nil {
			return err
		}
	}

	// Update VCPUs
	s.Logger().WithField("cpus-sandbox", sandboxVCPUs).Debugf("Request to hypervisor to update vCPUs")
	oldCPUs, newCPUs, err := s.hypervisor.resizeVCPUs(ctx, sandboxVCPUs)
	if err != nil {
		return err
	}

	s.Logger().Debugf("Request to hypervisor to update oldCPUs/newCPUs: %d/%d", oldCPUs, newCPUs)
	// If the CPUs were increased, ask agent to online them
	if oldCPUs < newCPUs {
		vcpusAdded := newCPUs - oldCPUs
		s.Logger().Debugf("Request to onlineCPUMem with %d CPUs", vcpusAdded)
		if err := s.agent.onlineCPUMem(ctx, vcpusAdded, true); err != nil {
			return err
		}
	}
	s.Logger().Debugf("Sandbox CPUs: %d", newCPUs)

	// Update Memory
	s.Logger().WithField("memory-sandbox-size-byte", sandboxMemoryByte).Debugf("Request to hypervisor to update memory")
	newMemory, updatedMemoryDevice, err := s.hypervisor.resizeMemory(ctx, uint32(sandboxMemoryByte>>utils.MibToBytesShift), s.state.GuestMemoryBlockSizeMB, s.state.GuestMemoryHotplugProbe)
	if err != nil {
		if err == noGuestMemHotplugErr {
			s.Logger().Warnf("%s, memory specifications cannot be guaranteed", err)
		} else {
			return err
		}
	}
	s.Logger().Debugf("Sandbox memory size: %d MB", newMemory)
	if s.state.GuestMemoryHotplugProbe && updatedMemoryDevice.addr != 0 {
		// notify the guest kernel about memory hot-add event, before onlining them
		s.Logger().Debugf("notify guest kernel memory hot-add event via probe interface, memory device located at 0x%x", updatedMemoryDevice.addr)
		if err := s.agent.memHotplugByProbe(ctx, updatedMemoryDevice.addr, uint32(updatedMemoryDevice.sizeMB), s.state.GuestMemoryBlockSizeMB); err != nil {
			return err
		}
	}
	if err := s.agent.onlineCPUMem(ctx, 0, false); err != nil {
		return err
	}

	return nil
}

func (s *Sandbox) calculateSandboxMemory() (int64, bool, int64) {
	memorySandbox := int64(0)
	needPodSwap := false
	swapSandbox := int64(0)
	for _, c := range s.config.Containers {
		// Do not hot add again non-running containers resources
		if cont, ok := s.containers[c.ID]; ok && cont.state.State == types.StateStopped {
			s.Logger().WithField("container", c.ID).Debug("Do not taking into account memory resources of not running containers")
			continue
		}

		if m := c.Resources.Memory; m != nil {
			currentLimit := int64(0)
			if m.Limit != nil {
				currentLimit = *m.Limit
				memorySandbox += currentLimit
			}
			if s.config.HypervisorConfig.GuestSwap && m.Swappiness != nil && *m.Swappiness > 0 {
				currentSwap := int64(0)
				if m.Swap != nil {
					currentSwap = *m.Swap
				}
				if currentSwap == 0 {
					if currentLimit == 0 {
						needPodSwap = true
					} else {
						swapSandbox += currentLimit
					}
				} else if currentSwap > currentLimit {
					swapSandbox = currentSwap - currentLimit
				}
			}
		}
	}
	return memorySandbox, needPodSwap, swapSandbox
}

func (s *Sandbox) calculateSandboxCPUs() (uint32, error) {
	mCPU := uint32(0)
	cpusetCount := int(0)

	for _, c := range s.config.Containers {
		// Do not hot add again non-running containers resources
		if cont, ok := s.containers[c.ID]; ok && cont.state.State == types.StateStopped {
			s.Logger().WithField("container", c.ID).Debug("Do not taking into account CPU resources of not running containers")
			continue
		}

		if cpu := c.Resources.CPU; cpu != nil {
			if cpu.Period != nil && cpu.Quota != nil {
				mCPU += utils.CalculateMilliCPUs(*cpu.Quota, *cpu.Period)
			}

			set, err := cpuset.Parse(cpu.Cpus)
			if err != nil {
				return 0, nil
			}
			cpusetCount += set.Size()
		}
	}

	// If we aren't being constrained, then we could have two scenarios:
	//  1. BestEffort QoS: no proper support today in Kata.
	//  2. We could be constrained only by CPUSets. Check for this:
	if mCPU == 0 && cpusetCount > 0 {
		return uint32(cpusetCount), nil
	}

	return utils.CalculateVCpusFromMilliCpus(mCPU), nil
}

// GetHypervisorType is used for getting Hypervisor name currently used.
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) GetHypervisorType() string {
	return string(s.config.HypervisorType)
}

// cgroupsUpdate will:
//  1) get the v1constraints cgroup associated with the stored cgroup path
//  2) (re-)add hypervisor vCPU threads to the appropriate cgroup
//  3) If we are managing sandbox cgroup, update the v1constraints cgroup size
func (s *Sandbox) cgroupsUpdate(ctx context.Context) error {

	// If Kata is configured for SandboxCgroupOnly, the VMM and its processes are already
	// in the Kata sandbox cgroup (inherited). Check to see if sandbox cpuset needs to be
	// updated.
	if s.config.SandboxCgroupOnly {
		cpuset, memset, err := s.getSandboxCPUSet()
		if err != nil {
			return err
		}

		if err := s.cgroupMgr.SetCPUSet(cpuset, memset); err != nil {
			return err
		}

		return nil
	}

	if s.state.CgroupPath == "" {
		s.Logger().Warn("sandbox's cgroup won't be updated: cgroup path is empty")
		return nil
	}

	cgroup, err := cgroupsLoadFunc(V1Constraints, cgroups.StaticPath(s.state.CgroupPath))
	if err != nil {
		return fmt.Errorf("Could not load cgroup %v: %v", s.state.CgroupPath, err)
	}

	if err := s.constrainHypervisor(ctx, cgroup); err != nil {
		return err
	}

	if len(s.containers) <= 1 {
		// nothing to update
		return nil
	}

	resources, err := s.resources()
	if err != nil {
		return err
	}

	if err := cgroup.Update(&resources); err != nil {
		return fmt.Errorf("Could not update sandbox cgroup path='%v' error='%v'", s.state.CgroupPath, err)
	}

	return nil
}

// cgroupsDelete will move the running processes in the sandbox cgroup
// to the parent and then delete the sandbox cgroup
func (s *Sandbox) cgroupsDelete() error {
	s.Logger().Debug("Deleting sandbox cgroup")
	if s.state.CgroupPath == "" {
		s.Logger().Warnf("sandbox cgroups path is empty")
		return nil
	}

	var path string
	var cgroupSubsystems cgroups.Hierarchy

	if s.config.SandboxCgroupOnly {
		return s.cgroupMgr.Destroy()
	}

	cgroupSubsystems = V1NoConstraints
	path = cgroupNoConstraintsPath(s.state.CgroupPath)
	s.Logger().WithField("path", path).Debug("Deleting no constraints cgroup")

	sandboxCgroups, err := cgroupsLoadFunc(cgroupSubsystems, cgroups.StaticPath(path))
	if err == cgroups.ErrCgroupDeleted {
		// cgroup already deleted
		s.Logger().Warnf("cgroup already deleted: '%s'", err)
		return nil
	}

	if err != nil {
		return fmt.Errorf("Could not load cgroups %v: %v", path, err)
	}

	// move running process here, that way cgroup can be removed
	parent, err := parentCgroup(cgroupSubsystems, path)
	if err != nil {
		// parent cgroup doesn't exist, that means there are no process running
		// and the no constraints cgroup was removed.
		s.Logger().WithError(err).Warn("Parent cgroup doesn't exist")
		return nil
	}

	if err := sandboxCgroups.MoveTo(parent); err != nil {
		// Don't fail, cgroup can be deleted
		s.Logger().WithError(err).Warnf("Could not move process from %s to parent cgroup", path)
	}

	return sandboxCgroups.Delete()
}

// constrainHypervisor will place the VMM and vCPU threads into cgroups.
func (s *Sandbox) constrainHypervisor(ctx context.Context, cgroup cgroups.Cgroup) error {
	// VMM threads are only placed into the constrained cgroup if SandboxCgroupOnly is being set.
	// This is the "correct" behavior, but if the parent cgroup isn't set up correctly to take
	// Kata/VMM into account, Kata may fail to boot due to being overconstrained.
	// If !SandboxCgroupOnly, place the VMM into an unconstrained cgroup, and the vCPU threads into constrained
	// cgroup
	if s.config.SandboxCgroupOnly {
		// Kata components were moved into the sandbox-cgroup already, so VMM
		// will already land there as well. No need to take action
		return nil
	}

	pids := s.hypervisor.getPids()
	if len(pids) == 0 || pids[0] == 0 {
		return fmt.Errorf("Invalid hypervisor PID: %+v", pids)
	}

	// VMM threads are only placed into the constrained cgroup if SandboxCgroupOnly is being set.
	// This is the "correct" behavior, but if the parent cgroup isn't set up correctly to take
	// Kata/VMM into account, Kata may fail to boot due to being overconstrained.
	// If !SandboxCgroupOnly, place the VMM into an unconstrained cgroup, and the vCPU threads into constrained
	// cgroup
	// Move the VMM into cgroups without constraints, those cgroups are not yet supported.
	resources := &specs.LinuxResources{}
	path := cgroupNoConstraintsPath(s.state.CgroupPath)
	vmmCgroup, err := cgroupsNewFunc(V1NoConstraints, cgroups.StaticPath(path), resources)
	if err != nil {
		return fmt.Errorf("Could not create cgroup %v: %v", path, err)
	}

	for _, pid := range pids {
		if pid <= 0 {
			s.Logger().Warnf("Invalid hypervisor pid: %d", pid)
			continue
		}

		if err := vmmCgroup.Add(cgroups.Process{Pid: pid}); err != nil {
			return fmt.Errorf("Could not add hypervisor PID %d to cgroup: %v", pid, err)
		}
	}

	// when new container joins, new CPU could be hotplugged, so we
	// have to query fresh vcpu info from hypervisor every time.
	tids, err := s.hypervisor.getThreadIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get thread ids from hypervisor: %v", err)
	}
	if len(tids.vcpus) == 0 {
		// If there's no tid returned from the hypervisor, this is not
		// a bug. It simply means there is nothing to constrain, hence
		// let's return without any error from here.
		return nil
	}

	// Move vcpus (threads) into cgroups with constraints.
	// Move whole hypervisor process would be easier but the IO/network performance
	// would be over-constrained.
	for _, i := range tids.vcpus {
		// In contrast, AddTask will write thread id to `tasks`
		// After this, vcpu threads are in "vcpu" sub-cgroup, other threads in
		// qemu will be left in parent cgroup untouched.
		if err := cgroup.AddTask(cgroups.Process{
			Pid: i,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sandbox) resources() (specs.LinuxResources, error) {
	resources := specs.LinuxResources{
		CPU: s.cpuResources(),
	}

	return resources, nil
}

func (s *Sandbox) cpuResources() *specs.LinuxCPU {
	// Use default period and quota if they are not specified.
	// Container will inherit the constraints from its parent.
	quota := int64(0)
	period := uint64(0)
	shares := uint64(0)
	realtimePeriod := uint64(0)
	realtimeRuntime := int64(0)

	cpu := &specs.LinuxCPU{
		Quota:           &quota,
		Period:          &period,
		Shares:          &shares,
		RealtimePeriod:  &realtimePeriod,
		RealtimeRuntime: &realtimeRuntime,
	}

	for _, c := range s.containers {
		ann := c.GetAnnotations()
		if ann[annotations.ContainerTypeKey] == string(PodSandbox) {
			// skip sandbox container
			continue
		}

		if c.config.Resources.CPU == nil {
			continue
		}

		if c.config.Resources.CPU.Shares != nil {
			shares = uint64(math.Max(float64(*c.config.Resources.CPU.Shares), float64(shares)))
		}

		if c.config.Resources.CPU.Quota != nil {
			quota += *c.config.Resources.CPU.Quota
		}

		if c.config.Resources.CPU.Period != nil {
			period = uint64(math.Max(float64(*c.config.Resources.CPU.Period), float64(period)))
		}

		if c.config.Resources.CPU.Cpus != "" {
			cpu.Cpus += c.config.Resources.CPU.Cpus + ","
		}

		if c.config.Resources.CPU.RealtimeRuntime != nil {
			realtimeRuntime += *c.config.Resources.CPU.RealtimeRuntime
		}

		if c.config.Resources.CPU.RealtimePeriod != nil {
			realtimePeriod += *c.config.Resources.CPU.RealtimePeriod
		}

		if c.config.Resources.CPU.Mems != "" {
			cpu.Mems += c.config.Resources.CPU.Mems + ","
		}
	}

	cpu.Cpus = strings.Trim(cpu.Cpus, " \n\t,")

	return validCPUResources(cpu)
}

// setupSandboxCgroup creates and joins sandbox cgroups for the sandbox config
func (s *Sandbox) setupSandboxCgroup() error {
	var err error
	spec := s.GetPatchedOCISpec()
	if spec == nil {
		return errorMissingOCISpec
	}

	if spec.Linux == nil {
		s.Logger().WithField("sandboxid", s.id).Warning("no cgroup path provided for pod sandbox, not creating sandbox cgroup")
		return nil
	}

	s.state.CgroupPath, err = vccgroups.ValidCgroupPath(spec.Linux.CgroupsPath, s.config.SystemdCgroup)
	if err != nil {
		return fmt.Errorf("Invalid cgroup path: %v", err)
	}

	runtimePid := os.Getpid()
	// Add the runtime to the Kata sandbox cgroup
	if err = s.cgroupMgr.Add(runtimePid); err != nil {
		return fmt.Errorf("Could not add runtime PID %d to sandbox cgroup:  %v", runtimePid, err)
	}

	// `Apply` updates manager's Cgroups and CgroupPaths,
	// they both need to be saved since are used to create
	// or restore a cgroup managers.
	if s.config.Cgroups, err = s.cgroupMgr.GetCgroups(); err != nil {
		return fmt.Errorf("Could not get cgroup configuration:  %v", err)
	}

	s.state.CgroupPaths = s.cgroupMgr.GetPaths()

	if err = s.cgroupMgr.Apply(); err != nil {
		return fmt.Errorf("Could not constrain cgroup: %v", err)
	}

	return nil
}

// GetPatchedOCISpec returns sandbox's OCI specification
// This OCI specification was patched when the sandbox was created
// by containerCapabilities(), SetEphemeralStorageType() and others
// in order to support:
// * capabilities
// * Ephemeral storage
// * k8s empty dir
// If you need the original (vanilla) OCI spec,
// use compatoci.GetContainerSpec() instead.
func (s *Sandbox) GetPatchedOCISpec() *specs.Spec {
	if s.config == nil {
		return nil
	}

	// get the container associated with the PodSandbox annotation. In Kubernetes, this
	// represents the pause container. In Docker, this is the container. We derive the
	// cgroup path from this container.
	for _, cConfig := range s.config.Containers {
		if cConfig.Annotations[annotations.ContainerTypeKey] == string(PodSandbox) {
			return cConfig.CustomSpec
		}
	}

	return nil
}

func (s *Sandbox) GetOOMEvent(ctx context.Context) (string, error) {
	return s.agent.getOOMEvent(ctx)
}

func (s *Sandbox) GetAgentURL() (string, error) {
	return s.agent.getAgentURL()
}

// getSandboxCPUSet returns the union of each of the sandbox's containers' CPU sets'
// cpus and mems as a string in canonical linux CPU/mems list format
func (s *Sandbox) getSandboxCPUSet() (string, string, error) {
	if s.config == nil {
		return "", "", nil
	}

	cpuResult := cpuset.NewCPUSet()
	memResult := cpuset.NewCPUSet()
	for _, ctr := range s.config.Containers {
		if ctr.Resources.CPU != nil {
			currCPUSet, err := cpuset.Parse(ctr.Resources.CPU.Cpus)
			if err != nil {
				return "", "", fmt.Errorf("unable to parse CPUset.cpus for container %s: %v", ctr.ID, err)
			}
			cpuResult = cpuResult.Union(currCPUSet)

			currMemSet, err := cpuset.Parse(ctr.Resources.CPU.Mems)
			if err != nil {
				return "", "", fmt.Errorf("unable to parse CPUset.mems for container %s: %v", ctr.ID, err)
			}
			memResult = memResult.Union(currMemSet)
		}
	}

	return cpuResult.String(), memResult.String(), nil
}

// fetchSandbox fetches a sandbox config from a sandbox ID and returns a sandbox.
func fetchSandbox(ctx context.Context, sandboxID string) (sandbox *Sandbox, err error) {
	virtLog.Info("fetch sandbox")
	if sandboxID == "" {
		return nil, vcTypes.ErrNeedSandboxID
	}

	var config SandboxConfig

	// load sandbox config fromld store.
	c, err := loadSandboxConfig(sandboxID)
	if err != nil {
		virtLog.WithError(err).Warning("failed to get sandbox config from store")
		return nil, err
	}

	config = *c

	// fetchSandbox is not suppose to create new sandbox VM.
	sandbox, err = createSandbox(ctx, config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox with config %+v: %v", config, err)
	}

	if sandbox.config.SandboxCgroupOnly {
		if err := sandbox.createCgroupManager(); err != nil {
			return nil, err
		}
	}

	// This sandbox already exists, we don't need to recreate the containers in the guest.
	// We only need to fetch the containers from storage and create the container structs.
	if err := sandbox.fetchContainers(ctx); err != nil {
		return nil, err
	}

	return sandbox, nil
}

// fetchContainers creates new containers structure and
// adds them to the sandbox. It does not create the containers
// in the guest. This should only be used when fetching a
// sandbox that already exists.
func (s *Sandbox) fetchContainers(ctx context.Context) error {
	for i, contConfig := range s.config.Containers {
		// Add spec from bundle path
		spec, err := compatoci.GetContainerSpec(contConfig.Annotations)
		if err != nil {
			return err
		}
		contConfig.CustomSpec = &spec
		s.config.Containers[i] = contConfig

		c, err := newContainer(ctx, s, &s.config.Containers[i])
		if err != nil {
			return err
		}

		if err := s.addContainer(c); err != nil {
			return err
		}
	}

	return nil
}
