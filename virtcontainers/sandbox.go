//
// Copyright (c) 2016 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package virtcontainers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

// controlSocket is the sandbox control socket.
// It is an hypervisor resource, and for example qemu's control
// socket is the QMP one.
const controlSocket = "ctl"

// monitorSocket is the sandbox monitoring socket.
// It is an hypervisor resource, and is a qmp socket in the qemu case.
// This is a socket that any monitoring entity will listen to in order
// to understand if the VM is still alive or not.
const monitorSocket = "mon"

// vmStartTimeout represents the time in seconds a sandbox can wait before
// to consider the VM starting operation failed.
const vmStartTimeout = 10

// stateString is a string representing a sandbox state.
type stateString string

const (
	// StateReady represents a sandbox/container that's ready to be run
	StateReady stateString = "ready"

	// StateRunning represents a sandbox/container that's currently running.
	StateRunning stateString = "running"

	// StatePaused represents a sandbox/container that has been paused.
	StatePaused stateString = "paused"

	// StateStopped represents a sandbox/container that has been stopped.
	StateStopped stateString = "stopped"
)

// State is a sandbox state structure.
type State struct {
	State stateString `json:"state"`

	// Index of the block device passed to hypervisor.
	BlockIndex int `json:"blockIndex"`

	// File system of the rootfs incase it is block device
	Fstype string `json:"fstype"`

	// Bool to indicate if the drive for a container was hotplugged.
	HotpluggedDrive bool `json:"hotpluggedDrive"`
}

// valid checks that the sandbox state is valid.
func (state *State) valid() bool {
	for _, validState := range []stateString{StateReady, StateRunning, StatePaused, StateStopped} {
		if state.State == validState {
			return true
		}
	}

	return false
}

// validTransition returns an error if we want to move to
// an unreachable state.
func (state *State) validTransition(oldState stateString, newState stateString) error {
	if state.State != oldState {
		return fmt.Errorf("Invalid state %s (Expecting %s)", state.State, oldState)
	}

	switch state.State {
	case StateReady:
		if newState == StateRunning || newState == StateStopped {
			return nil
		}

	case StateRunning:
		if newState == StatePaused || newState == StateStopped {
			return nil
		}

	case StatePaused:
		if newState == StateRunning || newState == StateStopped {
			return nil
		}

	case StateStopped:
		if newState == StateRunning {
			return nil
		}
	}

	return fmt.Errorf("Can not move from %s to %s",
		state.State, newState)
}

// Volume is a shared volume between the host and the VM,
// defined by its mount tag and its host path.
type Volume struct {
	// MountTag is a label used as a hint to the guest.
	MountTag string

	// HostPath is the host filesystem path for this volume.
	HostPath string
}

// Volumes is a Volume list.
type Volumes []Volume

// Set assigns volume values from string to a Volume.
func (v *Volumes) Set(volStr string) error {
	if volStr == "" {
		return fmt.Errorf("volStr cannot be empty")
	}

	volSlice := strings.Split(volStr, " ")
	const expectedVolLen = 2
	const volDelimiter = ":"

	for _, vol := range volSlice {
		volArgs := strings.Split(vol, volDelimiter)

		if len(volArgs) != expectedVolLen {
			return fmt.Errorf("Wrong string format: %s, expecting only %v parameters separated with %q",
				vol, expectedVolLen, volDelimiter)
		}

		if volArgs[0] == "" || volArgs[1] == "" {
			return fmt.Errorf("Volume parameters cannot be empty")
		}

		volume := Volume{
			MountTag: volArgs[0],
			HostPath: volArgs[1],
		}

		*v = append(*v, volume)
	}

	return nil
}

// String converts a Volume to a string.
func (v *Volumes) String() string {
	var volSlice []string

	for _, volume := range *v {
		volSlice = append(volSlice, fmt.Sprintf("%s:%s", volume.MountTag, volume.HostPath))
	}

	return strings.Join(volSlice, " ")
}

// Socket defines a socket to communicate between
// the host and any process inside the VM.
type Socket struct {
	DeviceID string
	ID       string
	HostPath string
	Name     string
}

// Sockets is a Socket list.
type Sockets []Socket

// Set assigns socket values from string to a Socket.
func (s *Sockets) Set(sockStr string) error {
	if sockStr == "" {
		return fmt.Errorf("sockStr cannot be empty")
	}

	sockSlice := strings.Split(sockStr, " ")
	const expectedSockCount = 4
	const sockDelimiter = ":"

	for _, sock := range sockSlice {
		sockArgs := strings.Split(sock, sockDelimiter)

		if len(sockArgs) != expectedSockCount {
			return fmt.Errorf("Wrong string format: %s, expecting only %v parameters separated with %q", sock, expectedSockCount, sockDelimiter)
		}

		for _, a := range sockArgs {
			if a == "" {
				return fmt.Errorf("Socket parameters cannot be empty")
			}
		}

		socket := Socket{
			DeviceID: sockArgs[0],
			ID:       sockArgs[1],
			HostPath: sockArgs[2],
			Name:     sockArgs[3],
		}

		*s = append(*s, socket)
	}

	return nil
}

// String converts a Socket to a string.
func (s *Sockets) String() string {
	var sockSlice []string

	for _, sock := range *s {
		sockSlice = append(sockSlice, fmt.Sprintf("%s:%s:%s:%s", sock.DeviceID, sock.ID, sock.HostPath, sock.Name))
	}

	return strings.Join(sockSlice, " ")
}

// Drive represents a block storage drive which may be used in case the storage
// driver has an underlying block storage device.
type Drive struct {

	// Path to the disk-image/device which will be used with this drive
	File string

	// Format of the drive
	Format string

	// ID is used to identify this drive in the hypervisor options.
	ID string

	// Index assigned to the drive. In case of virtio-scsi, this is used as SCSI LUN index
	Index int
}

// EnvVar is a key/value structure representing a command
// environment variable.
type EnvVar struct {
	Var   string
	Value string
}

// LinuxCapabilities specify the capabilities to keep when executing
// the process inside the container.
type LinuxCapabilities struct {
	// Bounding is the set of capabilities checked by the kernel.
	Bounding []string
	// Effective is the set of capabilities checked by the kernel.
	Effective []string
	// Inheritable is the capabilities preserved across execve.
	Inheritable []string
	// Permitted is the limiting superset for effective capabilities.
	Permitted []string
	// Ambient is the ambient set of capabilities that are kept.
	Ambient []string
}

// Cmd represents a command to execute in a running container.
type Cmd struct {
	Args                []string
	Envs                []EnvVar
	SupplementaryGroups []string

	// Note that these fields *MUST* remain as strings.
	//
	// The reason being that we want runtimes to be able to support CLI
	// operations like "exec --user=". That option allows the
	// specification of a user (either as a string username or a numeric
	// UID), and may optionally also include a group (groupame or GID).
	//
	// Since this type is the interface to allow the runtime to specify
	// the user and group the workload can run as, these user and group
	// fields cannot be encoded as integer values since that would imply
	// the runtime itself would need to perform a UID/GID lookup on the
	// user-specified username/groupname. But that isn't practically
	// possible given that to do so would require the runtime to access
	// the image to allow it to interrogate the appropriate databases to
	// convert the username/groupnames to UID/GID values.
	//
	// Note that this argument applies solely to the _runtime_ supporting
	// a "--user=" option when running in a "standalone mode" - there is
	// no issue when the runtime is called by a container manager since
	// all the user and group mapping is handled by the container manager
	// and specified to the runtime in terms of UID/GID's in the
	// configuration file generated by the container manager.
	User         string
	PrimaryGroup string
	WorkDir      string
	Console      string
	Capabilities LinuxCapabilities

	Interactive     bool
	Detach          bool
	NoNewPrivileges bool
}

// Resources describes VM resources configuration.
type Resources struct {
	// Memory is the amount of available memory in MiB.
	Memory uint
}

// SandboxStatus describes a sandbox status.
type SandboxStatus struct {
	ID               string
	State            State
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

	// Field specific to OCI specs, needed to setup all the hooks
	Hooks Hooks

	// VMConfig is the VM configuration to set for this sandbox.
	VMConfig Resources

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
	Volumes []Volume

	// Containers describe the list of containers within a Sandbox.
	// This list can be empty and populated by adding containers
	// to the Sandbox a posteriori.
	Containers []ContainerConfig

	// Annotations keys must be unique strings and must be name-spaced
	// with e.g. reverse domain notation (org.clearlinux.key).
	Annotations map[string]string
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

	hypervisor hypervisor
	agent      agent
	storage    resourceStorage
	network    network

	config *SandboxConfig

	volumes []Volume

	containers []*Container

	runPath    string
	configPath string

	state State

	networkNS NetworkNamespace

	annotationsLock *sync.RWMutex

	wg *sync.WaitGroup
}

// ID returns the sandbox identifier string.
func (s *Sandbox) ID() string {
	return s.id
}

// Logger returns a logrus logger appropriate for logging Sandbox messages
func (s *Sandbox) Logger() *logrus.Entry {
	return virtLog.WithFields(logrus.Fields{
		"subsystem":  "sandbox",
		"sandbox-id": s.id,
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

// GetAllContainers returns all containers.
func (s *Sandbox) GetAllContainers() []VCContainer {
	ifa := make([]VCContainer, len(s.containers))

	for i, v := range s.containers {
		ifa[i] = v
	}

	return ifa
}

// GetContainer returns the container named by the containerID.
func (s *Sandbox) GetContainer(containerID string) VCContainer {
	for _, c := range s.containers {
		if c.id == containerID {
			return c
		}
	}
	return nil
}

func createAssets(sandboxConfig *SandboxConfig) error {
	kernel, err := newAsset(sandboxConfig, kernelAsset)
	if err != nil {
		return err
	}

	image, err := newAsset(sandboxConfig, imageAsset)
	if err != nil {
		return err
	}

	initrd, err := newAsset(sandboxConfig, initrdAsset)
	if err != nil {
		return err
	}

	if image != nil && initrd != nil {
		return fmt.Errorf("%s and %s cannot be both set", imageAsset, initrdAsset)
	}

	for _, a := range []*asset{kernel, image, initrd} {
		if err := sandboxConfig.HypervisorConfig.addCustomAsset(a); err != nil {
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
func createSandbox(sandboxConfig SandboxConfig) (*Sandbox, error) {
	if err := createAssets(&sandboxConfig); err != nil {
		return nil, err
	}

	s, err := newSandbox(sandboxConfig)
	if err != nil {
		return nil, err
	}

	// Fetch sandbox network to be able to access it from the sandbox structure.
	networkNS, err := s.storage.fetchSandboxNetwork(s.id)
	if err == nil {
		s.networkNS = networkNS
	}

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
	if err := s.setSandboxState(StateReady); err != nil {
		return nil, err
	}

	return s, nil
}

func newSandbox(sandboxConfig SandboxConfig) (*Sandbox, error) {
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
		hypervisor:      hypervisor,
		agent:           agent,
		storage:         &filesystem{},
		network:         network,
		config:          &sandboxConfig,
		volumes:         sandboxConfig.Volumes,
		runPath:         filepath.Join(runStoragePath, sandboxConfig.ID),
		configPath:      filepath.Join(configStoragePath, sandboxConfig.ID),
		state:           State{},
		annotationsLock: &sync.RWMutex{},
		wg:              &sync.WaitGroup{},
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

	if err = s.storage.createAllResources(*s); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.storage.deleteSandboxResources(s.id, nil)
		}
	}()

	if err = s.hypervisor.init(s); err != nil {
		return nil, err
	}

	if err = s.hypervisor.createSandbox(sandboxConfig); err != nil {
		return nil, err
	}

	agentConfig := newAgentConfig(sandboxConfig)
	if err = s.agent.init(s, agentConfig); err != nil {
		return nil, err
	}

	return s, nil
}

// storeSandbox stores a sandbox config.
func (s *Sandbox) storeSandbox() error {
	err := s.storage.storeSandboxResource(s.id, configFileType, *(s.config))
	if err != nil {
		return err
	}

	for _, container := range s.containers {
		err = s.storage.storeContainerResource(s.id, container.id, configFileType, *(container.config))
		if err != nil {
			return err
		}
	}

	return nil
}

// fetchSandbox fetches a sandbox config from a sandbox ID and returns a sandbox.
func fetchSandbox(sandboxID string) (sandbox *Sandbox, err error) {
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

	sandbox, err = createSandbox(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox with config %+v: %v", config, err)
	}

	// This sandbox already exists, we don't need to recreate the containers in the guest.
	// We only need to fetch the containers from storage and create the container structs.
	if err := sandbox.newContainers(); err != nil {
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

	for _, c := range s.containers {
		if containerID == c.id {
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

	for idx, c := range s.containers {
		if containerID == c.id {
			s.containers = append(s.containers[:idx], s.containers[idx+1:]...)
			return nil
		}
	}

	return fmt.Errorf("Could not remove the container %q from the sandbox %q containers list",
		containerID, s.id)
}

// delete deletes an already created sandbox.
// The VM in which the sandbox is running will be shut down.
func (s *Sandbox) delete() error {
	if s.state.State != StateReady &&
		s.state.State != StatePaused &&
		s.state.State != StateStopped {
		return fmt.Errorf("Sandbox not ready, paused or stopped, impossible to delete")
	}

	for _, c := range s.containers {
		if err := c.delete(); err != nil {
			return err
		}
	}

	globalSandboxList.removeSandbox(s.id)

	return s.storage.deleteSandboxResources(s.id, nil)
}

func (s *Sandbox) createNetwork() error {
	// Initialize the network.
	netNsPath, netNsCreated, err := s.network.init(s.config.NetworkConfig)
	if err != nil {
		return err
	}

	// Execute prestart hooks inside netns
	if err := s.network.run(netNsPath, func() error {
		return s.config.Hooks.preStartHooks()
	}); err != nil {
		return err
	}

	// Add the network
	networkNS, err := s.network.add(*s, s.config.NetworkConfig, netNsPath, netNsCreated)
	if err != nil {
		return err
	}
	s.networkNS = networkNS

	// Store the network
	return s.storage.storeSandboxNetwork(s.id, networkNS)
}

func (s *Sandbox) removeNetwork() error {
	if s.networkNS.NetNsCreated {
		return s.network.remove(*s, s.networkNS)
	}

	return nil
}

// startVM starts the VM.
func (s *Sandbox) startVM() error {
	s.Logger().Info("Starting VM")

	if err := s.network.run(s.networkNS.NetNsPath, func() error {
		return s.hypervisor.startSandbox()
	}); err != nil {
		return err
	}

	if err := s.hypervisor.waitSandbox(vmStartTimeout); err != nil {
		return err
	}

	s.Logger().Info("VM started")

	// Once startVM is done, we want to guarantee
	// that the sandbox is manageable. For that we need
	// to start the sandbox inside the VM.
	return s.agent.startSandbox(*s)
}

func (s *Sandbox) addContainer(c *Container) error {
	s.containers = append(s.containers, c)

	return nil
}

// newContainers creates new containers structure and
// adds them to the sandbox. It does not create the containers
// in the guest. This should only be used when fetching a
// sandbox that already exists.
func (s *Sandbox) newContainers() error {
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

// createContainers registers all containers to the proxy, create the
// containers in the guest and starts one shim per container.
func (s *Sandbox) createContainers() error {
	for _, contConfig := range s.config.Containers {
		newContainer, err := createContainer(s, contConfig)
		if err != nil {
			return err
		}

		if err := s.addContainer(newContainer); err != nil {
			return err
		}
	}

	return nil
}

// start starts a sandbox. The containers that are making the sandbox
// will be started.
func (s *Sandbox) start() error {
	if err := s.state.validTransition(s.state.State, StateRunning); err != nil {
		return err
	}

	if err := s.setSandboxState(StateRunning); err != nil {
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

// stop stops a sandbox. The containers that are making the sandbox
// will be destroyed.
func (s *Sandbox) stop() error {
	if err := s.state.validTransition(s.state.State, StateStopped); err != nil {
		return err
	}

	for _, c := range s.containers {
		if err := c.stop(); err != nil {
			return err
		}
	}

	if err := s.agent.stopSandbox(*s); err != nil {
		return err
	}

	s.Logger().Info("Stopping VM")
	if err := s.hypervisor.stopSandbox(); err != nil {
		return err
	}

	return s.setSandboxState(StateStopped)
}

func (s *Sandbox) pause() error {
	if err := s.hypervisor.pauseSandbox(); err != nil {
		return err
	}

	return s.pauseSetStates()
}

func (s *Sandbox) resume() error {
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
func (s *Sandbox) setSandboxState(state stateString) error {
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
	if err := s.setContainersState(StatePaused); err != nil {
		return err
	}

	return s.setSandboxState(StatePaused)
}

func (s *Sandbox) resumeSetStates() error {
	// XXX: Resuming a paused sandbox puts all containers back into the
	// running state.
	if err := s.setContainersState(StateRunning); err != nil {
		return err
	}

	return s.setSandboxState(StateRunning)
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

func (s *Sandbox) setContainersState(state stateString) error {
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
func togglePauseSandbox(sandboxID string, pause bool) (*Sandbox, error) {
	if sandboxID == "" {
		return nil, errNeedSandbox
	}

	lockFile, err := rwLockSandbox(sandboxID)
	if err != nil {
		return nil, err
	}
	defer unlockSandbox(lockFile)

	// Fetch the sandbox from storage and create it.
	p, err := fetchSandbox(sandboxID)
	if err != nil {
		return nil, err
	}

	if pause {
		err = p.pause()
	} else {
		err = p.resume()
	}

	if err != nil {
		return nil, err
	}

	return p, nil
}
