// Copyright (c) 2016 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"

	"github.com/kata-containers/agent/protocols/grpc"
	"github.com/kata-containers/runtime/virtcontainers/device/api"
	"github.com/kata-containers/runtime/virtcontainers/device/config"
	"github.com/kata-containers/runtime/virtcontainers/device/drivers"
	deviceManager "github.com/kata-containers/runtime/virtcontainers/device/manager"
	vcTypes "github.com/kata-containers/runtime/virtcontainers/pkg/types"
	"github.com/kata-containers/runtime/virtcontainers/types"
	"github.com/kata-containers/runtime/virtcontainers/utils"
	"github.com/vishvananda/netlink"
)

const (
	// vmStartTimeout represents the time in seconds a sandbox can wait before
	// to consider the VM starting operation failed.
	vmStartTimeout = 10
)

// SandboxStatus describes a sandbox status.
type SandboxStatus struct {
	ID               string
	State            types.State
	Hypervisor       HypervisorType
	HypervisorConfig HypervisorConfig
	Agent            AgentType
	ContainersStatus []ContainerStatus

	// Annotations allow clients to store arbitrary values,
	// for example to add additional status values required
	// to support particular specifications.
	Annotations map[string]string
}

// SandboxConfig is a Sandbox configuration.
type SandboxConfig struct {
	ID string

	Hostname string

	HypervisorType   HypervisorType
	HypervisorConfig HypervisorConfig

	AgentType   AgentType
	AgentConfig interface{}

	ProxyType   ProxyType
	ProxyConfig ProxyConfig

	ShimType   ShimType
	ShimConfig interface{}

	NetworkModel  NetworkModel
	NetworkConfig NetworkConfig

	// Volumes is a list of shared volumes between the host and the Sandbox.
	Volumes []types.Volume

	// Containers describe the list of containers within a Sandbox.
	// This list can be empty and populated by adding containers
	// to the Sandbox a posteriori.
	//TODO: this should be a map to avoid duplicated containers
	Containers []ContainerConfig

	// Annotations keys must be unique strings and must be name-spaced
	// with e.g. reverse domain notation (org.clearlinux.key).
	Annotations map[string]string

	ShmSize uint64

	// SharePidNs sets all containers to share the same sandbox level pid namespace.
	SharePidNs bool

	// types.Stateful keeps sandbox resources in memory across APIs. Users will be responsible
	// for calling Release() to release the memory resources.
	Stateful bool

	// SystemdCgroup enables systemd cgroup support
	SystemdCgroup bool

	DisableGuestSeccomp bool
}

func (s *Sandbox) trace(name string) (opentracing.Span, context.Context) {
	if s.ctx == nil {
		s.Logger().WithField("type", "bug").Error("trace called before context set")
		s.ctx = context.Background()
	}

	span, ctx := opentracing.StartSpanFromContext(s.ctx, name)

	span.SetTag("subsystem", "sandbox")

	return span, ctx
}

func (s *Sandbox) startProxy() error {

	// If the proxy is KataBuiltInProxyType type, it needs to restart the proxy
	// to watch the guest console if it hadn't been watched.
	if s.agent == nil {
		return fmt.Errorf("sandbox %s missed agent pointer", s.ID())
	}

	return s.agent.startProxy(s)
}

// valid checks that the sandbox configuration is valid.
func (sandboxConfig *SandboxConfig) valid() bool {
	if sandboxConfig.ID == "" {
		return false
	}

	if _, err := newHypervisor(sandboxConfig.HypervisorType); err != nil {
		sandboxConfig.HypervisorType = QemuHypervisor
	}

	return true
}

const (
	// R/W lock
	exclusiveLock = syscall.LOCK_EX

	// Read only lock
	sharedLock = syscall.LOCK_SH
)

// rLockSandbox locks the sandbox with a shared lock.
func rLockSandbox(sandboxID string) (*os.File, error) {
	return lockSandbox(sandboxID, sharedLock)
}

// rwLockSandbox locks the sandbox with an exclusive lock.
func rwLockSandbox(sandboxID string) (*os.File, error) {
	return lockSandbox(sandboxID, exclusiveLock)
}

// lock locks any sandbox to prevent it from being accessed by other processes.
func lockSandbox(sandboxID string, lockType int) (*os.File, error) {
	if sandboxID == "" {
		return nil, errNeedSandboxID
	}

	fs := filesystem{}
	sandboxlockFile, _, err := fs.sandboxURI(sandboxID, lockFileType)
	if err != nil {
		return nil, err
	}

	lockFile, err := os.Open(sandboxlockFile)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(lockFile.Fd()), lockType); err != nil {
		return nil, err
	}

	return lockFile, nil
}

// unlock unlocks any sandbox to allow it being accessed by other processes.
func unlockSandbox(lockFile *os.File) error {
	if lockFile == nil {
		return fmt.Errorf("lockFile cannot be empty")
	}

	err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	if err != nil {
		return err
	}

	lockFile.Close()

	return nil
}

// Sandbox is composed of a set of containers and a runtime environment.
// A Sandbox can be created, deleted, started, paused, stopped, listed, entered, and restored.
type Sandbox struct {
	id string

	sync.Mutex
	factory    Factory
	hypervisor hypervisor
	agent      agent
	storage    resourceStorage
	network    network
	monitor    *monitor

	config *SandboxConfig

	devManager api.DeviceManager

	volumes []types.Volume

	containers map[string]*Container

	runPath    string
	configPath string

	state types.State

	networkNS NetworkNamespace

	annotationsLock *sync.RWMutex

	wg *sync.WaitGroup

	shmSize          uint64
	sharePidNs       bool
	stateful         bool
	seccompSupported bool

	ctx context.Context

	cgroup *sandboxCgroups
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
	value, exist := s.config.Annotations[key]
	if exist == false {
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

	err := s.storage.storeSandboxResource(s.id, configFileType, *(s.config))
	if err != nil {
		return err
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
	for id, c := range s.containers {
		if id == containerID {
			return c
		}
	}
	return nil
}

// Release closes the agent connection and removes sandbox from internal list.
func (s *Sandbox) Release() error {
	s.Logger().Info("release sandbox")
	globalSandboxList.removeSandbox(s.id)
	if s.monitor != nil {
		s.monitor.stop()
	}
	s.hypervisor.disconnect()
	return s.agent.disconnect()
}

func (s *Sandbox) releaseStatelessSandbox() error {
	if s.stateful {
		return nil
	}

	return s.Release()
}

// Status gets the status of the sandbox
// TODO: update container status properly, see kata-containers/runtime#253
func (s *Sandbox) Status() SandboxStatus {
	var contStatusList []ContainerStatus
	for _, c := range s.containers {
		contStatusList = append(contStatusList, ContainerStatus{
			ID:          c.id,
			State:       c.state,
			PID:         c.process.Pid,
			StartTime:   c.process.StartTime,
			RootFs:      c.config.RootFs,
			Annotations: c.config.Annotations,
		})
	}

	return SandboxStatus{
		ID:               s.id,
		State:            s.state,
		Hypervisor:       s.config.HypervisorType,
		HypervisorConfig: s.config.HypervisorConfig,
		Agent:            s.config.AgentType,
		ContainersStatus: contStatusList,
		Annotations:      s.config.Annotations,
	}
}

// Monitor returns a error channel for watcher to watch at
func (s *Sandbox) Monitor() (chan error, error) {
	if s.state.State != types.StateRunning {
		return nil, fmt.Errorf("Sandbox is not running")
	}

	s.Lock()
	if s.monitor == nil {
		s.monitor = newMonitor(s)
	}
	s.Unlock()

	return s.monitor.newWatcher()
}

// WaitProcess waits on a container process and return its exit code
func (s *Sandbox) WaitProcess(containerID, processID string) (int32, error) {
	if s.state.State != types.StateRunning {
		return 0, fmt.Errorf("Sandbox not running")
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return 0, err
	}

	return c.wait(processID)
}

// SignalProcess sends a signal to a process of a container when all is false.
// When all is true, it sends the signal to all processes of a container.
func (s *Sandbox) SignalProcess(containerID, processID string, signal syscall.Signal, all bool) error {
	if s.state.State != types.StateRunning {
		return fmt.Errorf("Sandbox not running")
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	return c.signalProcess(processID, signal, all)
}

// WinsizeProcess resizes the tty window of a process
func (s *Sandbox) WinsizeProcess(containerID, processID string, height, width uint32) error {
	if s.state.State != types.StateRunning {
		return fmt.Errorf("Sandbox not running")
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	return c.winsizeProcess(processID, height, width)
}

// IOStream returns stdin writer, stdout reader and stderr reader of a process
func (s *Sandbox) IOStream(containerID, processID string) (io.WriteCloser, io.Reader, io.Reader, error) {
	if s.state.State != types.StateRunning {
		return nil, nil, nil, fmt.Errorf("Sandbox not running")
	}

	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, nil, nil, err
	}

	return c.ioStream(processID)
}

func createAssets(ctx context.Context, sandboxConfig *SandboxConfig) error {
	span, _ := trace(ctx, "createAssets")
	defer span.Finish()

	kernel, err := types.NewAsset(sandboxConfig.Annotations, types.KernelAsset)
	if err != nil {
		return err
	}

	image, err := types.NewAsset(sandboxConfig.Annotations, types.ImageAsset)
	if err != nil {
		return err
	}

	initrd, err := types.NewAsset(sandboxConfig.Annotations, types.InitrdAsset)
	if err != nil {
		return err
	}

	if image != nil && initrd != nil {
		return fmt.Errorf("%s and %s cannot be both set", types.ImageAsset, types.InitrdAsset)
	}

	for _, a := range []*types.Asset{kernel, image, initrd} {
		if err := sandboxConfig.HypervisorConfig.addCustomAsset(a); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sandbox) getAndStoreGuestDetails() error {
	guestDetailRes, err := s.agent.getGuestDetails(&grpc.GuestDetailsRequest{
		MemBlockSize: true,
	})
	if err != nil {
		return err
	}

	if guestDetailRes != nil {
		s.state.GuestMemoryBlockSizeMB = uint32(guestDetailRes.MemBlockSizeBytes >> 20)
		if guestDetailRes.AgentDetails != nil {
			s.seccompSupported = guestDetailRes.AgentDetails.SupportsSeccomp
		}

		if err = s.storage.storeSandboxResource(s.id, stateFileType, s.state); err != nil {
			return err
		}
	}

	return nil
}

// createSandbox creates a sandbox from a sandbox description, the containers list, the hypervisor
// and the agent passed through the Config structure.
// It will create and store the sandbox structure, and then ask the hypervisor
// to physically create that sandbox i.e. starts a VM for that sandbox to eventually
// be started.
func createSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (*Sandbox, error) {
	span, ctx := trace(ctx, "createSandbox")
	defer span.Finish()

	if err := createAssets(ctx, &sandboxConfig); err != nil {
		return nil, err
	}

	s, err := newSandbox(ctx, sandboxConfig, factory)
	if err != nil {
		return nil, err
	}

	// Fetch sandbox network to be able to access it from the sandbox structure.
	networkNS, err := s.storage.fetchSandboxNetwork(s.id)
	if err == nil {
		s.networkNS = networkNS
	}

	devices, err := s.storage.fetchSandboxDevices(s.id)
	if err != nil {
		s.Logger().WithError(err).WithField("sandboxid", s.id).Warning("fetch sandbox device failed")
	}
	s.devManager = deviceManager.NewDeviceManager(sandboxConfig.HypervisorConfig.BlockDeviceDriver, devices)

	// We first try to fetch the sandbox state from storage.
	// If it exists, this means this is a re-creation, i.e.
	// we don't need to talk to the guest's agent, but only
	// want to create the sandbox and its containers in memory.
	state, err := s.storage.fetchSandboxState(s.id)
	if err == nil && state.State != "" {
		s.state = state
		return s, nil
	}

	// Below code path is called only during create, because of earlier check.
	if err := s.agent.createSandbox(s); err != nil {
		return nil, err
	}

	// Set sandbox state
	if err := s.setSandboxState(types.StateReady); err != nil {
		return nil, err
	}

	return s, nil
}

func newSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory) (*Sandbox, error) {
	span, ctx := trace(ctx, "newSandbox")
	defer span.Finish()

	if sandboxConfig.valid() == false {
		return nil, fmt.Errorf("Invalid sandbox configuration")
	}

	agent := newAgent(sandboxConfig.AgentType)

	hypervisor, err := newHypervisor(sandboxConfig.HypervisorType)
	if err != nil {
		return nil, err
	}

	network := newNetwork(sandboxConfig.NetworkModel)

	s := &Sandbox{
		id:              sandboxConfig.ID,
		factory:         factory,
		hypervisor:      hypervisor,
		agent:           agent,
		storage:         &filesystem{},
		network:         network,
		config:          &sandboxConfig,
		volumes:         sandboxConfig.Volumes,
		containers:      map[string]*Container{},
		runPath:         filepath.Join(runStoragePath, sandboxConfig.ID),
		configPath:      filepath.Join(configStoragePath, sandboxConfig.ID),
		state:           types.State{},
		annotationsLock: &sync.RWMutex{},
		wg:              &sync.WaitGroup{},
		shmSize:         sandboxConfig.ShmSize,
		sharePidNs:      sandboxConfig.SharePidNs,
		stateful:        sandboxConfig.Stateful,
		ctx:             ctx,
	}

	if err = globalSandboxList.addSandbox(s); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.Logger().WithError(err).WithField("sandboxid", s.id).Error("Create new sandbox failed")
			globalSandboxList.removeSandbox(s.id)
		}
	}()

	if err = s.storage.createAllResources(ctx, s); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.storage.deleteSandboxResources(s.id, nil)
		}
	}()

	if err = s.hypervisor.createSandbox(ctx, s.id, &sandboxConfig.HypervisorConfig, s.storage); err != nil {
		return nil, err
	}

	agentConfig := newAgentConfig(sandboxConfig.AgentType, sandboxConfig.AgentConfig)
	if err = s.agent.init(ctx, s, agentConfig); err != nil {
		return nil, err
	}

	// create new cgroup for sandbox
	if err := s.newCgroups(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Sandbox) storeSandboxDevices() error {
	return s.storage.storeSandboxDevices(s.id, s.devManager.GetAllDevices())
}

// storeSandbox stores a sandbox config.
func (s *Sandbox) storeSandbox() error {
	span, _ := s.trace("storeSandbox")
	defer span.Finish()

	err := s.storage.storeSandboxResource(s.id, configFileType, *(s.config))
	if err != nil {
		return err
	}

	for id, container := range s.containers {
		err = s.storage.storeContainerResource(s.id, id, configFileType, *(container.config))
		if err != nil {
			return err
		}
	}

	return nil
}

// fetchSandbox fetches a sandbox config from a sandbox ID and returns a sandbox.
func fetchSandbox(ctx context.Context, sandboxID string) (sandbox *Sandbox, err error) {
	virtLog.Info("fetch sandbox")
	if sandboxID == "" {
		return nil, errNeedSandboxID
	}

	sandbox, err = globalSandboxList.lookupSandbox(sandboxID)
	if sandbox != nil && err == nil {
		return sandbox, err
	}

	fs := filesystem{}
	config, err := fs.fetchSandboxConfig(sandboxID)
	if err != nil {
		return nil, err
	}

	// fetchSandbox is not suppose to create new sandbox VM.
	sandbox, err = createSandbox(ctx, config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox with config %+v: %v", config, err)
	}

	// This sandbox already exists, we don't need to recreate the containers in the guest.
	// We only need to fetch the containers from storage and create the container structs.
	if err := sandbox.fetchContainers(); err != nil {
		return nil, err
	}

	return sandbox, nil
}

// findContainer returns a container from the containers list held by the
// sandbox structure, based on a container ID.
func (s *Sandbox) findContainer(containerID string) (*Container, error) {
	if s == nil {
		return nil, errNeedSandbox
	}

	if containerID == "" {
		return nil, errNeedContainerID
	}

	for id, c := range s.containers {
		if containerID == id {
			return c, nil
		}
	}

	return nil, fmt.Errorf("Could not find the container %q from the sandbox %q containers list",
		containerID, s.id)
}

// removeContainer removes a container from the containers list held by the
// sandbox structure, based on a container ID.
func (s *Sandbox) removeContainer(containerID string) error {
	if s == nil {
		return errNeedSandbox
	}

	if containerID == "" {
		return errNeedContainerID
	}

	if _, ok := s.containers[containerID]; !ok {
		return fmt.Errorf("Could not remove the container %q from the sandbox %q containers list",
			containerID, s.id)
	}

	delete(s.containers, containerID)

	return nil
}

// Delete deletes an already created sandbox.
// The VM in which the sandbox is running will be shut down.
func (s *Sandbox) Delete() error {
	if s.state.State != types.StateReady &&
		s.state.State != types.StatePaused &&
		s.state.State != types.StateStopped {
		return fmt.Errorf("Sandbox not ready, paused or stopped, impossible to delete")
	}

	for _, c := range s.containers {
		if err := c.delete(); err != nil {
			return err
		}
	}

	// destroy sandbox cgroup
	if err := s.destroyCgroups(); err != nil {
		// continue the removal process even cgroup failed to destroy
		s.Logger().WithError(err).Error("failed to destroy cgroup")
	}

	globalSandboxList.removeSandbox(s.id)

	if s.monitor != nil {
		s.monitor.stop()
	}

	if err := s.hypervisor.cleanup(); err != nil {
		s.Logger().WithError(err).Error("failed to cleanup hypervisor")
	}

	return s.storage.deleteSandboxResources(s.id, nil)
}

func (s *Sandbox) startNetworkMonitor() error {
	span, _ := s.trace("startNetworkMonitor")
	defer span.Finish()

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

	return s.network.run(s.networkNS.NetNsPath, func() error {
		pid, err := startNetmon(params)
		if err != nil {
			return err
		}

		s.networkNS.NetmonPID = pid

		return nil
	})
}

func (s *Sandbox) createNetwork() error {
	if s.config.NetworkConfig.DisableNewNetNs {
		return nil
	}

	span, _ := s.trace("createNetwork")
	defer span.Finish()

	s.networkNS = NetworkNamespace{
		NetNsPath:    s.config.NetworkConfig.NetNSPath,
		NetNsCreated: s.config.NetworkConfig.NetNsCreated,
	}

	// In case there is a factory, network interfaces are hotplugged
	// after vm is started.
	if s.factory == nil {
		// Add the network
		if err := s.network.add(s, false); err != nil {
			return err
		}

		if s.config.NetworkConfig.NetmonConfig.Enable {
			if err := s.startNetworkMonitor(); err != nil {
				return err
			}
		}
	}

	// Store the network
	return s.storage.storeSandboxNetwork(s.id, s.networkNS)
}

func (s *Sandbox) removeNetwork() error {
	span, _ := s.trace("removeNetwork")
	defer span.Finish()

	if s.config.NetworkConfig.NetmonConfig.Enable {
		if err := stopNetmon(s.networkNS.NetmonPID); err != nil {
			return err
		}
	}

	return s.network.remove(s, s.factory != nil)
}

func (s *Sandbox) generateNetInfo(inf *vcTypes.Interface) (NetworkInfo, error) {
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
			Type: inf.LinkType,
		},
		Addrs: addrs,
	}, nil
}

// AddInterface adds new nic to the sandbox.
func (s *Sandbox) AddInterface(inf *vcTypes.Interface) (*vcTypes.Interface, error) {
	netInfo, err := s.generateNetInfo(inf)
	if err != nil {
		return nil, err
	}

	endpoint, err := createEndpoint(netInfo, len(s.networkNS.Endpoints), s.config.NetworkConfig.InterworkingModel)
	if err != nil {
		return nil, err
	}

	endpoint.SetProperties(netInfo)
	if err := doNetNS(s.networkNS.NetNsPath, func(_ ns.NetNS) error {
		s.Logger().WithField("endpoint-type", endpoint.Type()).Info("Hot attaching endpoint")
		return endpoint.HotAttach(s.hypervisor)
	}); err != nil {
		return nil, err
	}

	// Update the sandbox storage
	s.networkNS.Endpoints = append(s.networkNS.Endpoints, endpoint)
	if err := s.storage.storeSandboxNetwork(s.id, s.networkNS); err != nil {
		return nil, err
	}

	// Add network for vm
	inf.PciAddr = endpoint.PciAddr()
	return s.agent.updateInterface(inf)
}

// RemoveInterface removes a nic of the sandbox.
func (s *Sandbox) RemoveInterface(inf *vcTypes.Interface) (*vcTypes.Interface, error) {
	for i, endpoint := range s.networkNS.Endpoints {
		if endpoint.HardwareAddr() == inf.HwAddr {
			s.Logger().WithField("endpoint-type", endpoint.Type()).Info("Hot detaching endpoint")
			if err := endpoint.HotDetach(s.hypervisor, s.networkNS.NetNsCreated, s.networkNS.NetNsPath); err != nil {
				return inf, err
			}
			s.networkNS.Endpoints = append(s.networkNS.Endpoints[:i], s.networkNS.Endpoints[i+1:]...)
			if err := s.storage.storeSandboxNetwork(s.id, s.networkNS); err != nil {
				return inf, err
			}
			break
		}
	}
	return nil, nil
}

// ListInterfaces lists all nics and their configurations in the sandbox.
func (s *Sandbox) ListInterfaces() ([]*vcTypes.Interface, error) {
	return s.agent.listInterfaces()
}

// UpdateRoutes updates the sandbox route table (e.g. for portmapping support).
func (s *Sandbox) UpdateRoutes(routes []*vcTypes.Route) ([]*vcTypes.Route, error) {
	return s.agent.updateRoutes(routes)
}

// ListRoutes lists all routes and their configurations in the sandbox.
func (s *Sandbox) ListRoutes() ([]*vcTypes.Route, error) {
	return s.agent.listRoutes()
}

// startVM starts the VM.
func (s *Sandbox) startVM() error {
	span, ctx := s.trace("startVM")
	defer span.Finish()

	s.Logger().Info("Starting VM")

	if err := s.network.run(s.networkNS.NetNsPath, func() error {
		if s.factory != nil {
			vm, err := s.factory.GetVM(ctx, VMConfig{
				HypervisorType:   s.config.HypervisorType,
				HypervisorConfig: s.config.HypervisorConfig,
				AgentType:        s.config.AgentType,
				AgentConfig:      s.config.AgentConfig,
				ProxyType:        s.config.ProxyType,
				ProxyConfig:      s.config.ProxyConfig,
			})
			if err != nil {
				return err
			}
			err = vm.assignSandbox(s)
			if err != nil {
				return err
			}
			return nil
		}

		return s.hypervisor.startSandbox(vmStartTimeout)
	}); err != nil {
		return err
	}

	// In case of vm factory, network interfaces are hotplugged
	// after vm is started.
	if s.factory != nil {
		if err := s.network.add(s, true); err != nil {
			return err
		}

		if s.config.NetworkConfig.NetmonConfig.Enable {
			if err := s.startNetworkMonitor(); err != nil {
				return err
			}
		}
		if err := s.storage.storeSandboxNetwork(s.id, s.networkNS); err != nil {
			return err
		}
	}

	s.Logger().Info("VM started")

	// Once the hypervisor is done starting the sandbox,
	// we want to guarantee that it is manageable.
	// For that we need to ask the agent to start the
	// sandbox inside the VM.
	if err := s.agent.startSandbox(s); err != nil {
		return err
	}

	s.Logger().Info("Agent started in the sandbox")

	return nil
}

// stopVM: stop the sandbox's VM
func (s *Sandbox) stopVM() error {
	span, _ := s.trace("stopVM")
	defer span.Finish()

	s.Logger().Info("Stopping sandbox in the VM")
	if err := s.agent.stopSandbox(s); err != nil {
		s.Logger().WithError(err).WithField("sandboxid", s.id).Warning("Agent did not stop sandbox")
	}

	s.Logger().Info("Stopping VM")
	return s.hypervisor.stopSandbox()
}

func (s *Sandbox) addContainer(c *Container) error {
	if _, ok := s.containers[c.id]; ok {
		return fmt.Errorf("Duplicated container: %s", c.id)
	}
	s.containers[c.id] = c

	return nil
}

// newContainers creates new containers structure and
// adds them to the sandbox. It does not create the containers
// in the guest. This should only be used when fetching a
// sandbox that already exists.
func (s *Sandbox) fetchContainers() error {
	for _, contConfig := range s.config.Containers {
		c, err := newContainer(s, contConfig)
		if err != nil {
			return err
		}

		if err := s.addContainer(c); err != nil {
			return err
		}
	}

	return nil
}

// CreateContainer creates a new container in the sandbox
// This should be called only when the sandbox is already created.
// It will add new container config to sandbox.config.Containers
func (s *Sandbox) CreateContainer(contConfig ContainerConfig) (VCContainer, error) {
	// Create the container.
	c, err := newContainer(s, contConfig)
	if err != nil {
		return nil, err
	}

	// Update sandbox config.
	s.config.Containers = append(s.config.Containers, contConfig)

	// Sandbox is reponsable to update VM resources needed by Containers
	s.updateResources()
	if err != nil {
		return nil, err
	}

	err = c.create()
	if err != nil {
		return nil, err
	}

	// Add the container to the containers list in the sandbox.
	if err := s.addContainer(c); err != nil {
		return nil, err
	}

	// Store it.
	err = c.storeContainer()
	if err != nil {
		return nil, err
	}

	err = s.storage.storeSandboxResource(s.id, configFileType, *(s.config))
	if err != nil {
		return nil, err
	}

	// Setup host cgroups for new container
	if err := s.setupCgroups(); err != nil {
		return nil, err
	}
	return c, nil
}

// StartContainer starts a container in the sandbox
func (s *Sandbox) StartContainer(containerID string) (VCContainer, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Start it.
	err = c.start()
	if err != nil {
		return nil, err
	}
	//Fixme Container delete from sandbox, need to update resources

	return c, nil
}

// StopContainer stops a container in the sandbox
func (s *Sandbox) StopContainer(containerID string) (VCContainer, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Stop it.
	if err := c.stop(); err != nil {
		return nil, err
	}

	return c, nil
}

// KillContainer signals a container in the sandbox
func (s *Sandbox) KillContainer(containerID string, signal syscall.Signal, all bool) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Send a signal to the process.
	return c.kill(signal, all)
}

// DeleteContainer deletes a container from the sandbox
func (s *Sandbox) DeleteContainer(containerID string) (VCContainer, error) {
	if containerID == "" {
		return nil, errNeedContainerID
	}

	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Delete it.
	err = c.delete()
	if err != nil {
		return nil, err
	}

	// Update sandbox config
	for idx, contConfig := range s.config.Containers {
		if contConfig.ID == containerID {
			s.config.Containers = append(s.config.Containers[:idx], s.config.Containers[idx+1:]...)
			break
		}
	}

	// Store sandbox config
	err = s.storage.storeSandboxResource(s.id, configFileType, *(s.config))
	if err != nil {
		return nil, err
	}

	return c, nil
}

// ProcessListContainer lists every process running inside a specific
// container in the sandbox.
func (s *Sandbox) ProcessListContainer(containerID string, options ProcessListOptions) (ProcessList, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Get the process list related to the container.
	return c.processList(options)
}

// StatusContainer gets the status of a container
// TODO: update container status properly, see kata-containers/runtime#253
func (s *Sandbox) StatusContainer(containerID string) (ContainerStatus, error) {
	if containerID == "" {
		return ContainerStatus{}, errNeedContainerID
	}

	for id, c := range s.containers {
		if id == containerID {
			return ContainerStatus{
				ID:          c.id,
				State:       c.state,
				PID:         c.process.Pid,
				StartTime:   c.process.StartTime,
				RootFs:      c.config.RootFs,
				Annotations: c.config.Annotations,
			}, nil
		}
	}

	return ContainerStatus{}, errNoSuchContainer
}

// EnterContainer is the virtcontainers container command execution entry point.
// EnterContainer enters an already running container and runs a given command.
func (s *Sandbox) EnterContainer(containerID string, cmd types.Cmd) (VCContainer, *Process, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return nil, nil, err
	}

	// Enter it.
	process, err := c.enter(cmd)
	if err != nil {
		return nil, nil, err
	}

	return c, process, nil
}

// UpdateContainer update a running container.
func (s *Sandbox) UpdateContainer(containerID string, resources specs.LinuxResources) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	err = c.update(resources)
	if err != nil {
		return err
	}

	return c.storeContainer()
}

// StatsContainer return the stats of a running container
func (s *Sandbox) StatsContainer(containerID string) (ContainerStats, error) {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return ContainerStats{}, err
	}

	stats, err := c.stats()
	if err != nil {
		return ContainerStats{}, err
	}
	return *stats, nil
}

// PauseContainer pauses a running container.
func (s *Sandbox) PauseContainer(containerID string) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Pause the container.
	return c.pause()
}

// ResumeContainer resumes a paused container.
func (s *Sandbox) ResumeContainer(containerID string) error {
	// Fetch the container.
	c, err := s.findContainer(containerID)
	if err != nil {
		return err
	}

	// Resume the container.
	return c.resume()
}

// createContainers registers all containers to the proxy, create the
// containers in the guest and starts one shim per container.
func (s *Sandbox) createContainers() error {
	span, _ := s.trace("createContainers")
	defer span.Finish()

	if err := s.updateResources(); err != nil {
		return err
	}

	for _, contConfig := range s.config.Containers {

		c, err := newContainer(s, contConfig)
		if err != nil {
			return err
		}
		if err := c.create(); err != nil {
			return err
		}

		if err := s.addContainer(c); err != nil {
			return err
		}
	}

	return nil
}

// Start starts a sandbox. The containers that are making the sandbox
// will be started.
func (s *Sandbox) Start() error {
	if err := s.state.ValidTransition(s.state.State, types.StateRunning); err != nil {
		return err
	}

	if err := s.setSandboxState(types.StateRunning); err != nil {
		return err
	}

	for _, c := range s.containers {
		if err := c.start(); err != nil {
			return err
		}
	}

	s.Logger().Info("Sandbox is started")

	return nil
}

// Stop stops a sandbox. The containers that are making the sandbox
// will be destroyed.
func (s *Sandbox) Stop() error {
	span, _ := s.trace("stop")
	defer span.Finish()

	if s.state.State == types.StateStopped {
		s.Logger().Info("sandbox already stopped")
		return nil
	}

	if err := s.state.ValidTransition(s.state.State, types.StateStopped); err != nil {
		return err
	}

	for _, c := range s.containers {
		if err := c.stop(); err != nil {
			return err
		}
	}

	if err := s.stopVM(); err != nil {
		return err
	}

	if err := s.setSandboxState(types.StateStopped); err != nil {
		return err
	}

	// Remove the network.
	return s.removeNetwork()
}

// Pause pauses the sandbox
func (s *Sandbox) Pause() error {
	if err := s.hypervisor.pauseSandbox(); err != nil {
		return err
	}

	//After the sandbox is paused, it's needed to stop its monitor,
	//Otherwise, its monitors will receive timeout errors if it is
	//paused for a long time, thus its monitor will not tell it's a
	//crash caused timeout or just a paused timeout.
	if s.monitor != nil {
		s.monitor.stop()
	}

	return s.pauseSetStates()
}

// Resume resumes the sandbox
func (s *Sandbox) Resume() error {
	if err := s.hypervisor.resumeSandbox(); err != nil {
		return err
	}

	return s.resumeSetStates()
}

// list lists all sandbox running on the host.
func (s *Sandbox) list() ([]Sandbox, error) {
	return nil, nil
}

// enter runs an executable within a sandbox.
func (s *Sandbox) enter(args []string) error {
	return nil
}

// setSandboxState sets both the in-memory and on-disk state of the
// sandbox.
func (s *Sandbox) setSandboxState(state types.StateString) error {
	if state == "" {
		return errNeedState
	}

	// update in-memory state
	s.state.State = state

	// update on-disk state
	return s.storage.storeSandboxResource(s.id, stateFileType, s.state)
}

func (s *Sandbox) pauseSetStates() error {
	// XXX: When a sandbox is paused, all its containers are forcibly
	// paused too.
	if err := s.setContainersState(types.StatePaused); err != nil {
		return err
	}

	return s.setSandboxState(types.StatePaused)
}

func (s *Sandbox) resumeSetStates() error {
	// XXX: Resuming a paused sandbox puts all containers back into the
	// running state.
	if err := s.setContainersState(types.StateRunning); err != nil {
		return err
	}

	return s.setSandboxState(types.StateRunning)
}

// getAndSetSandboxBlockIndex retrieves sandbox block index and increments it for
// subsequent accesses. This index is used to maintain the index at which a
// block device is assigned to a container in the sandbox.
func (s *Sandbox) getAndSetSandboxBlockIndex() (int, error) {
	currentIndex := s.state.BlockIndex

	// Increment so that container gets incremented block index
	s.state.BlockIndex++

	// update on-disk state
	err := s.storage.storeSandboxResource(s.id, stateFileType, s.state)
	if err != nil {
		return -1, err
	}

	return currentIndex, nil
}

// decrementSandboxBlockIndex decrements the current sandbox block index.
// This is used to recover from failure while adding a block device.
func (s *Sandbox) decrementSandboxBlockIndex() error {
	s.state.BlockIndex--

	// update on-disk state
	err := s.storage.storeSandboxResource(s.id, stateFileType, s.state)
	if err != nil {
		return err
	}

	return nil
}

// setSandboxPid sets the Pid of the the shim process belonging to the
// sandbox container as the Pid of the sandbox.
func (s *Sandbox) setSandboxPid(pid int) error {
	s.state.Pid = pid

	// update on-disk state
	return s.storage.storeSandboxResource(s.id, stateFileType, s.state)
}

func (s *Sandbox) setContainersState(state types.StateString) error {
	if state == "" {
		return errNeedState
	}

	for _, c := range s.containers {
		if err := c.setContainerState(state); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sandbox) deleteContainerState(containerID string) error {
	if containerID == "" {
		return errNeedContainerID
	}

	err := s.storage.deleteContainerResources(s.id, containerID, []sandboxResource{stateFileType})
	if err != nil {
		return err
	}

	return nil
}

func (s *Sandbox) deleteContainersState() error {
	for _, container := range s.config.Containers {
		err := s.deleteContainerState(container.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// togglePauseSandbox pauses a sandbox if pause is set to true, else it resumes
// it.
func togglePauseSandbox(ctx context.Context, sandboxID string, pause bool) (*Sandbox, error) {
	span, ctx := trace(ctx, "togglePauseSandbox")
	defer span.Finish()

	if sandboxID == "" {
		return nil, errNeedSandbox
	}

	lockFile, err := rwLockSandbox(sandboxID)
	if err != nil {
		return nil, err
	}
	defer unlockSandbox(lockFile)

	// Fetch the sandbox from storage and create it.
	s, err := fetchSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	defer s.releaseStatelessSandbox()

	if pause {
		err = s.Pause()
	} else {
		err = s.Resume()
	}

	if err != nil {
		return nil, err
	}

	return s, nil
}

// HotplugAddDevice is used for add a device to sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) HotplugAddDevice(device api.Device, devType config.DeviceType) error {
	span, _ := s.trace("HotplugAddDevice")
	defer span.Finish()

	switch devType {
	case config.DeviceVFIO:
		vfioDevices, ok := device.GetDeviceInfo().([]*config.VFIODev)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}

		// adding a group of VFIO devices
		for _, dev := range vfioDevices {
			if _, err := s.hypervisor.hotplugAddDevice(dev, vfioDev); err != nil {
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
		_, err := s.hypervisor.hotplugAddDevice(blockDevice.BlockDrive, blockDev)
		return err
	case config.DeviceGeneric:
		// TODO: what?
		return nil
	}
	return nil
}

// HotplugRemoveDevice is used for removing a device from sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) HotplugRemoveDevice(device api.Device, devType config.DeviceType) error {
	switch devType {
	case config.DeviceVFIO:
		vfioDevices, ok := device.GetDeviceInfo().([]*config.VFIODev)
		if !ok {
			return fmt.Errorf("device type mismatch, expect device type to be %s", devType)
		}

		// remove a group of VFIO devices
		for _, dev := range vfioDevices {
			if _, err := s.hypervisor.hotplugRemoveDevice(dev, vfioDev); err != nil {
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
		_, err := s.hypervisor.hotplugRemoveDevice(blockDrive, blockDev)
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

// DecrementSandboxBlockIndex decrease block indexes
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) DecrementSandboxBlockIndex() error {
	return s.decrementSandboxBlockIndex()
}

// AppendDevice can only handle vhost user device currently, it adds a
// vhost user device to sandbox
// Sandbox implement DeviceReceiver interface from device/api/interface.go
func (s *Sandbox) AppendDevice(device api.Device) error {
	switch device.DeviceType() {
	case config.VhostUserSCSI, config.VhostUserNet, config.VhostUserBlk:
		return s.hypervisor.addDevice(device.GetDeviceInfo().(*config.VhostUserDeviceAttrs), vhostuserDev)
	}
	return fmt.Errorf("unsupported device type")
}

// AddDevice will add a device to sandbox
func (s *Sandbox) AddDevice(info config.DeviceInfo) (api.Device, error) {
	if s.devManager == nil {
		return nil, fmt.Errorf("device manager isn't initialized")
	}

	b, err := s.devManager.NewDevice(info)
	if err != nil {
		return nil, err
	}

	if err := s.devManager.AttachDevice(b.DeviceID(), s); err != nil {
		return nil, err
	}

	if err := s.storeSandboxDevices(); err != nil {
		return nil, err
	}

	return b, nil
}

func (s *Sandbox) updateResources() error {
	// the hypervisor.MemorySize is the amount of memory reserved for
	// the VM and contaniners without memory limit

	sumResources := specs.LinuxResources{
		Memory: &specs.LinuxMemory{
			Limit: new(int64),
		},
		CPU: &specs.LinuxCPU{
			Period: new(uint64),
			Quota:  new(int64),
		},
	}

	for _, c := range s.config.Containers {
		if m := c.Resources.Memory; m != nil && m.Limit != nil {
			*sumResources.Memory.Limit += *m.Limit
		}
		if cpu := c.Resources.CPU; cpu != nil {
			if cpu.Period != nil && cpu.Quota != nil {
				*sumResources.CPU.Period += *cpu.Period
				*sumResources.CPU.Quota += *cpu.Quota
			}
		}
	}

	sandboxVCPUs := uint32(utils.ConstraintsToVCPUs(*sumResources.CPU.Quota, *sumResources.CPU.Period))
	sandboxVCPUs += s.hypervisor.hypervisorConfig().NumVCPUs

	sandboxMemoryByte := int64(s.hypervisor.hypervisorConfig().MemorySize) << utils.MibToBytesShift
	sandboxMemoryByte += *sumResources.Memory.Limit

	// Update VCPUs
	s.Logger().WithField("cpus-sandbox", sandboxVCPUs).Debugf("Request to hypervisor to update vCPUs")
	oldCPUs, newCPUs, err := s.hypervisor.resizeVCPUs(sandboxVCPUs)
	if err != nil {
		return err
	}
	// The CPUs were increased, ask agent to online them
	if oldCPUs < newCPUs {
		vcpusAdded := newCPUs - oldCPUs
		if err := s.agent.onlineCPUMem(vcpusAdded, true); err != nil {
			return err
		}
	}
	s.Logger().Debugf("Sandbox CPUs: %d", newCPUs)

	// Update Memory
	s.Logger().WithField("memory-sandbox-size-byte", sandboxMemoryByte).Debugf("Request to hypervisor to update memory")
	newMemory, err := s.hypervisor.resizeMemory(uint32(sandboxMemoryByte>>utils.MibToBytesShift), s.state.GuestMemoryBlockSizeMB)
	if err != nil {
		return err
	}
	s.Logger().Debugf("Sandbox memory size: %d Byte", newMemory)
	if err := s.agent.onlineCPUMem(0, false); err != nil {
		return err
	}
	return nil
}
