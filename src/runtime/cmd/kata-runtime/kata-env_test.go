// Copyright (c) 2017 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	vc "github.com/kata-containers/kata-containers/src/runtime/virtcontainers"
	vcUtils "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/utils"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"

	"github.com/kata-containers/kata-containers/src/runtime/pkg/device/config"
	"github.com/kata-containers/kata-containers/src/runtime/pkg/katatestutils"
	"github.com/kata-containers/kata-containers/src/runtime/pkg/katautils"
	"github.com/kata-containers/kata-containers/src/runtime/pkg/oci"
	"github.com/stretchr/testify/assert"
)

var (
	testHypervisorVersion = "QEMU emulator version 2.7.0+git.741f430a96-6.1, Copyright (c) 2003-2016 Fabrice Bellard and the QEMU Project developers"
)

var (
	hypervisorDebug = false
	enableVirtioFS  = false
	runtimeDebug    = false
	runtimeTrace    = false
	agentDebug      = false
	agentTrace      = false
)

// makeVersionBinary creates a shell script with the specified file
// name. When run as "file --version", it will display the specified
// version to stdout and exit successfully.
func makeVersionBinary(file, version string) error {
	err := createFile(file,
		fmt.Sprintf(`#!/bin/sh
	[ "$1" = "--version" ] && echo "%s"`, version))
	if err != nil {
		return err
	}

	err = os.Chmod(file, testExeFileMode)
	if err != nil {
		return err
	}

	return nil
}

func createConfig(configPath string, fileData string) error {

	err := os.WriteFile(configPath, []byte(fileData), testFileMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create config file %s %v\n", configPath, err)
		return err
	}

	return nil
}

func makeRuntimeConfig(prefixDir string) (configFile string, ociConfig oci.RuntimeConfig, err error) {
	var hotPlugVFIO config.PCIePort
	var coldPlugVFIO config.PCIePort
	const logPath = "/log/path"
	hypervisorPath := filepath.Join(prefixDir, "hypervisor")
	kernelPath := filepath.Join(prefixDir, "kernel")
	imagePath := filepath.Join(prefixDir, "image")
	rootfsType := "ext4"
	kernelParams := "foo=bar xyz"
	machineType := "machineType"
	disableBlock := true
	blockStorageDriver := "virtio-scsi"
	enableIOThreads := true
	hotPlugVFIO = config.BridgePort
	coldPlugVFIO = config.NoPort
	pcieRootPort := uint32(0)
	pcieSwitchPort := uint32(0)
	disableNewNetNs := false
	sharedFS := "virtio-9p"
	virtioFSdaemon := filepath.Join(prefixDir, "virtiofsd")

	filesToCreate := []string{
		hypervisorPath,
		kernelPath,
		imagePath,
	}

	for _, file := range filesToCreate {
		// files must exist and be >0 bytes.
		err := katautils.WriteFile(file, "foo", testFileMode)
		if err != nil {
			return "", oci.RuntimeConfig{}, err
		}
	}

	err = makeVersionBinary(hypervisorPath, testHypervisorVersion)
	if err != nil {
		return "", oci.RuntimeConfig{}, err
	}

	if enableVirtioFS {
		sharedFS = "virtio-fs"
	}

	hypConfig := katautils.GetDefaultHypervisorConfig()

	configFileOptions := katatestutils.RuntimeConfigOptions{
		Hypervisor:           "qemu",
		HypervisorPath:       hypervisorPath,
		KernelPath:           kernelPath,
		ImagePath:            imagePath,
		RootfsType:           rootfsType,
		KernelParams:         kernelParams,
		MachineType:          machineType,
		LogPath:              logPath,
		DefaultGuestHookPath: hypConfig.GuestHookPath,
		DisableBlock:         disableBlock,
		BlockDeviceDriver:    blockStorageDriver,
		EnableIOThreads:      enableIOThreads,
		HotPlugVFIO:          hotPlugVFIO,
		ColdPlugVFIO:         coldPlugVFIO,
		PCIeRootPort:         pcieRootPort,
		PCIeSwitchPort:       pcieSwitchPort,
		DisableNewNetNs:      disableNewNetNs,
		DefaultVCPUCount:     hypConfig.NumVCPUs(),
		DefaultMaxVCPUCount:  hypConfig.DefaultMaxVCPUs,
		DefaultMemSize:       hypConfig.MemorySize,
		DefaultMsize9p:       hypConfig.Msize9p,
		HypervisorDebug:      hypervisorDebug,
		RuntimeDebug:         runtimeDebug,
		RuntimeTrace:         runtimeTrace,
		AgentDebug:           agentDebug,
		AgentTrace:           agentTrace,
		SharedFS:             sharedFS,
		VirtioFSDaemon:       virtioFSdaemon,
	}

	runtimeConfig := katatestutils.MakeRuntimeConfigFileData(configFileOptions)

	configFile = path.Join(prefixDir, "runtime.toml")
	err = createConfig(configFile, runtimeConfig)
	if err != nil {
		return "", oci.RuntimeConfig{}, err
	}

	_, ociConfig, err = katautils.LoadConfiguration(configFile, true)
	if err != nil {
		return "", oci.RuntimeConfig{}, err
	}

	return configFile, ociConfig, nil
}

func getExpectedAgentDetails(config oci.RuntimeConfig) (AgentInfo, error) {

	agentConfig := config.AgentConfig
	return AgentInfo{
		Debug: agentConfig.Debug,
		Trace: agentConfig.Trace,
	}, nil
}

// nolint: unused, deadcode
func genericGetExpectedHostDetails(tmpdir string, expectedVendor string, expectedModel string, expectedVMContainerCapable bool) (HostInfo, error) {
	type filesToCreate struct {
		file     string
		contents string
	}

	const expectedKernelVersion = "99.1"
	const expectedArch = goruntime.GOARCH

	expectedDistro := DistroInfo{
		Name:    "Foo",
		Version: "42",
	}

	expectedCPU := CPUInfo{
		Vendor: expectedVendor,
		Model:  expectedModel,
	}

	expectedSupportVSocks, _ := vcUtils.SupportsVsocks()
	expectedHostDetails := HostInfo{
		AvailableGuestProtections: vc.AvailableGuestProtections(),
		Kernel:                    expectedKernelVersion,
		Architecture:              expectedArch,
		Distro:                    expectedDistro,
		CPU:                       expectedCPU,
		VMContainerCapable:        expectedVMContainerCapable,
		SupportVSocks:             expectedSupportVSocks,
	}

	testProcCPUInfo := filepath.Join(tmpdir, "cpuinfo")
	testOSRelease := filepath.Join(tmpdir, "os-release")

	// XXX: This file is *NOT* created by this function on purpose
	// (to ensure the only file checked by the tests is
	// testOSRelease). osReleaseClr handling is tested in
	// utils_test.go.
	testOSReleaseClr := filepath.Join(tmpdir, "os-release-clr")

	testProcVersion := filepath.Join(tmpdir, "proc-version")

	// override
	procVersion = testProcVersion
	osRelease = testOSRelease
	osReleaseClr = testOSReleaseClr
	procCPUInfo = testProcCPUInfo

	procVersionContents := fmt.Sprintf("Linux version %s a b c",
		expectedKernelVersion)

	osReleaseContents := fmt.Sprintf(`
NAME="%s"
VERSION_ID="%s"
`, expectedDistro.Name, expectedDistro.Version)

	procCPUInfoContents := fmt.Sprintf(`
%s	: %s
%s	: %s
`,
		archCPUVendorField,
		expectedCPU.Vendor,
		archCPUModelField,
		expectedCPU.Model)

	data := []filesToCreate{
		{procVersion, procVersionContents},
		{osRelease, osReleaseContents},
		{procCPUInfo, procCPUInfoContents},
	}

	for _, d := range data {
		err := createFile(d.file, d.contents)
		if err != nil {
			return HostInfo{}, err
		}
	}

	if goruntime.GOARCH == "arm64" {
		expectedHostDetails.CPU.Vendor = "ARM Limited"
		expectedHostDetails.CPU.Model = "v8"
	}

	// set CPU num.
	// will not set memory info, because memory may be changed.
	expectedHostDetails.CPU.CPUs = runtime.NumCPU()

	return expectedHostDetails, nil
}

func getExpectedHypervisor(config oci.RuntimeConfig) HypervisorInfo {
	info := HypervisorInfo{
		Version:           testHypervisorVersion,
		Path:              config.HypervisorConfig.HypervisorPath,
		MachineType:       config.HypervisorConfig.HypervisorMachineType,
		BlockDeviceDriver: config.HypervisorConfig.BlockDeviceDriver,
		Msize9p:           config.HypervisorConfig.Msize9p,
		MemorySlots:       config.HypervisorConfig.MemSlots,
		Debug:             config.HypervisorConfig.Debug,
		EntropySource:     config.HypervisorConfig.EntropySource,
		SharedFS:          config.HypervisorConfig.SharedFS,
		VirtioFSDaemon:    config.HypervisorConfig.VirtioFSDaemon,
		HotPlugVFIO:       config.HypervisorConfig.HotPlugVFIO,
		ColdPlugVFIO:      config.HypervisorConfig.ColdPlugVFIO,
		PCIeRootPort:      config.HypervisorConfig.PCIeRootPort,
		PCIeSwitchPort:    config.HypervisorConfig.PCIeSwitchPort,
	}

	if os.Geteuid() == 0 {
		// This assumes the test hypervisor is a non-hybrid-vsock
		// one (such as QEMU).
		info.SocketPath = ""
	} else {
		info.SocketPath = unknown
	}

	return info
}

func getExpectedImage(config oci.RuntimeConfig) ImageInfo {
	return ImageInfo{
		Path: config.HypervisorConfig.ImagePath,
	}
}

func getExpectedKernel(config oci.RuntimeConfig) KernelInfo {
	return KernelInfo{
		Path:       config.HypervisorConfig.KernelPath,
		Parameters: strings.Join(vc.SerializeParams(config.HypervisorConfig.KernelParams, "="), " "),
	}
}

func getExpectedRuntimeDetails(config oci.RuntimeConfig, configFile string) RuntimeInfo {
	runtimePath, _ := os.Executable()

	runtimeVersionInfo := constructVersionInfo(katautils.VERSION)
	runtimeVersionInfo.Commit = katautils.COMMIT
	return RuntimeInfo{
		Version: RuntimeVersionInfo{
			Version: runtimeVersionInfo,
			OCI:     specs.Version,
		},
		Config: RuntimeConfigInfo{
			Path: configFile,
		},
		Path:            runtimePath,
		Debug:           config.Debug,
		Trace:           config.Trace,
		DisableNewNetNs: config.DisableNewNetNs,
	}
}

func getExpectedSettings(config oci.RuntimeConfig, tmpdir, configFile string) (EnvInfo, error) {
	meta := getExpectedMetaInfo()

	runtime := getExpectedRuntimeDetails(config, configFile)

	agent, err := getExpectedAgentDetails(config)
	if err != nil {
		return EnvInfo{}, err
	}

	host, err := getExpectedHostDetails(tmpdir)
	if err != nil {
		return EnvInfo{}, err
	}

	hypervisor := getExpectedHypervisor(config)
	kernel := getExpectedKernel(config)
	image := getExpectedImage(config)

	env := EnvInfo{
		Meta:       meta,
		Runtime:    runtime,
		Hypervisor: hypervisor,
		Image:      image,
		Kernel:     kernel,
		Agent:      agent,
		Host:       host,
	}

	return env, nil
}

func getExpectedMetaInfo() MetaInfo {
	return MetaInfo{
		Version: formatVersion,
	}
}

func TestEnvGetMetaInfo(t *testing.T) {
	expectedMeta := getExpectedMetaInfo()

	meta := getMetaInfo()

	assert.Equal(t, expectedMeta, meta)
}

func TestEnvGetHostInfo(t *testing.T) {
	if os.Getenv("GITHUB_RUNNER_CI_ARM64") == "true" {
		t.Skip("Skipping the test as the GitHub self hosted runners for ARM64 do not support Virtualization")
	}

	tmpdir := t.TempDir()

	expectedHostDetails, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	host, err := getHostInfo()
	assert.NoError(t, err)

	// Free/Available are changing
	expectedHostDetails.Memory = host.Memory

	assert.Equal(t, expectedHostDetails, host)

	// check CPU cores and memory info
	assert.Equal(t, true, host.CPU.CPUs > 0)
	assert.Equal(t, true, host.Memory.Total > 0)
	assert.Equal(t, true, host.Memory.Free > 0)
	assert.Equal(t, true, host.Memory.Available > 0)
}

func TestEnvGetHostInfoNoProcCPUInfo(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	err = os.Remove(procCPUInfo)
	assert.NoError(t, err)

	_, err = getHostInfo()
	assert.Error(t, err)
}

func TestEnvGetHostInfoNoOSRelease(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	err = os.Remove(osRelease)
	assert.NoError(t, err)

	_, err = getHostInfo()
	assert.NoError(t, err)
}

func TestEnvGetHostInfoNoProcVersion(t *testing.T) {
	tmpdir := t.TempDir()

	_, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	err = os.Remove(procVersion)
	assert.NoError(t, err)

	_, err = getHostInfo()
	assert.Error(t, err)
}

func TestEnvGetEnvInfo(t *testing.T) {
	if os.Getenv("GITHUB_RUNNER_CI_ARM64") == "true" {
		t.Skip("Skipping the test as the GitHub self hosted runners for ARM64 do not support Virtualization")
	}

	tmpdir := t.TempDir()

	// Run test twice to ensure the individual component debug+trace
	// options are tested.
	for _, toggle := range []bool{false, true} {
		hypervisorDebug = toggle
		enableVirtioFS = toggle
		runtimeDebug = toggle
		runtimeTrace = toggle
		agentDebug = toggle

		configFile, config, err := makeRuntimeConfig(tmpdir)
		assert.NoError(t, err)

		expectedEnv, err := getExpectedSettings(config, tmpdir, configFile)
		assert.NoError(t, err)

		env, err := getEnvInfo(configFile, config)
		assert.NoError(t, err)

		// Free/Available are changing
		expectedEnv.Host.Memory = env.Host.Memory

		assert.Equal(t, expectedEnv, env)
	}
}

func TestEnvGetEnvInfoNoHypervisorVersion(t *testing.T) {
	if os.Getenv("GITHUB_RUNNER_CI_ARM64") == "true" {
		t.Skip("Skipping the test as the GitHub self hosted runners for ARM64 do not support Virtualization")
	}

	assert := assert.New(t)

	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(err)

	expectedEnv, err := getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(err)

	err = os.Remove(config.HypervisorConfig.HypervisorPath)
	assert.NoError(err)

	expectedEnv.Hypervisor.Version = unknown

	env, err := getEnvInfo(configFile, config)
	assert.NoError(err)

	// Free/Available are changing
	expectedEnv.Host.Memory = env.Host.Memory

	assert.Equal(expectedEnv, env)
}

func TestEnvGetEnvInfoAgentError(t *testing.T) {
	assert := assert.New(t)

	tmpdir := t.TempDir()

	_, _, err := makeRuntimeConfig(tmpdir)
	assert.NoError(err)
}

func TestEnvGetEnvInfoNoOSRelease(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	err = os.Remove(osRelease)
	assert.NoError(t, err)

	_, err = getEnvInfo(configFile, config)
	assert.NoError(t, err)
}

func TestEnvGetEnvInfoNoProcCPUInfo(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	err = os.Remove(procCPUInfo)
	assert.NoError(t, err)

	_, err = getEnvInfo(configFile, config)
	assert.Error(t, err)
}

func TestEnvGetEnvInfoNoProcVersion(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	err = os.Remove(procVersion)
	assert.NoError(t, err)

	_, err = getEnvInfo(configFile, config)
	assert.Error(t, err)
}

func TestEnvGetRuntimeInfo(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	expectedRuntime := getExpectedRuntimeDetails(config, configFile)

	runtime := getRuntimeInfo(configFile, config)

	assert.Equal(t, expectedRuntime, runtime)
}

func TestEnvGetAgentInfo(t *testing.T) {
	tmpdir := t.TempDir()

	_, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	expectedAgent, err := getExpectedAgentDetails(config)
	assert.NoError(t, err)

	agent, err := getAgentInfo(config)
	assert.NoError(t, err)

	assert.Equal(t, expectedAgent, agent)

	agentConfig := config.AgentConfig

	agentConfig.Debug = true
	config.AgentConfig = agentConfig
	agent, err = getAgentInfo(config)
	assert.NoError(t, err)
	assert.True(t, agent.Debug)

	agentConfig.Trace = true
	config.AgentConfig = agentConfig
	agent, err = getAgentInfo(config)
	assert.NoError(t, err)
	assert.True(t, agent.Trace)
}

func testEnvShowTOMLSettings(t *testing.T, tmpdir string, tmpfile *os.File) error {

	runtime := RuntimeInfo{}

	hypervisor := HypervisorInfo{
		Path:        "/resolved/hypervisor/path",
		MachineType: "hypervisor-machine-type",
	}

	image := ImageInfo{
		Path: "/resolved/image/path",
	}

	kernel := KernelInfo{
		Path:       "/kernel/path",
		Parameters: "foo=bar xyz",
	}

	agent := AgentInfo{}

	expectedHostDetails, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	env := EnvInfo{
		Runtime:    runtime,
		Hypervisor: hypervisor,
		Image:      image,
		Kernel:     kernel,
		Agent:      agent,
		Host:       expectedHostDetails,
	}

	err = writeTOMLSettings(env, tmpfile)
	if err != nil {
		return err
	}

	contents, err := katautils.GetFileContents(tmpfile.Name())
	assert.NoError(t, err)

	buf := new(bytes.Buffer)
	encoder := toml.NewEncoder(buf)
	err = encoder.Encode(env)
	assert.NoError(t, err)

	expectedContents := buf.String()

	assert.Equal(t, expectedContents, contents)

	return nil
}

func testEnvShowJSONSettings(t *testing.T, tmpdir string, tmpfile *os.File) error {

	runtime := RuntimeInfo{}

	hypervisor := HypervisorInfo{
		Path:        "/resolved/hypervisor/path",
		MachineType: "hypervisor-machine-type",
	}

	image := ImageInfo{
		Path: "/resolved/image/path",
	}

	kernel := KernelInfo{
		Path:       "/kernel/path",
		Parameters: "foo=bar xyz",
	}

	agent := AgentInfo{}

	expectedHostDetails, err := getExpectedHostDetails(tmpdir)
	assert.NoError(t, err)

	env := EnvInfo{
		Runtime:    runtime,
		Hypervisor: hypervisor,
		Image:      image,
		Kernel:     kernel,
		Agent:      agent,
		Host:       expectedHostDetails,
	}

	err = writeJSONSettings(env, tmpfile)
	if err != nil {
		return err
	}

	contents, err := katautils.GetFileContents(tmpfile.Name())
	assert.NoError(t, err)

	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	// Ensure we have the same human readable layout
	encoder.SetIndent("", "  ")
	err = encoder.Encode(env)
	assert.NoError(t, err)

	expectedContents := buf.String()

	assert.Equal(t, expectedContents, contents)

	return nil
}

func TestEnvShowSettings(t *testing.T) {
	tmpdir := t.TempDir()

	tmpfile, err := os.CreateTemp("", "envShowSettings-")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = testEnvShowTOMLSettings(t, tmpdir, tmpfile)
	assert.NoError(t, err)

	// Reset the file to empty for next test
	tmpfile.Truncate(0)
	tmpfile.Seek(0, 0)
	err = testEnvShowJSONSettings(t, tmpdir, tmpfile)
	assert.NoError(t, err)
}

func TestEnvShowSettingsInvalidFile(t *testing.T) {
	tmpdir := t.TempDir()

	tmpfile, err := os.CreateTemp("", "envShowSettings-")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// close the file
	tmpfile.Close()

	err = testEnvShowTOMLSettings(t, tmpdir, tmpfile)
	assert.Error(t, err)

	// Reset the file to empty for next test
	tmpfile.Truncate(0)
	tmpfile.Seek(0, 0)
	err = testEnvShowJSONSettings(t, tmpdir, tmpfile)
	assert.Error(t, err)
}

func TestEnvHandleSettings(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := createCLIContext(set)
	ctx.App.Name = "foo"
	ctx.App.Metadata["configFile"] = configFile
	ctx.App.Metadata["runtimeConfig"] = config

	tmpfile, err := os.CreateTemp("", "")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	err = handleSettings(tmpfile, ctx)
	assert.NoError(t, err)

	var env EnvInfo

	_, err = toml.DecodeFile(tmpfile.Name(), &env)
	assert.NoError(t, err)
}

func TestEnvHandleSettingsInvalidParams(t *testing.T) {
	assert := assert.New(t)

	tmpdir := t.TempDir()

	configFile, _, err := makeRuntimeConfig(tmpdir)
	assert.NoError(err)

	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"
	ctx.App.Metadata["configFile"] = configFile

	err = handleSettings(nil, ctx)
	assert.Error(err)
}

func TestEnvHandleSettingsEmptyMap(t *testing.T) {
	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"
	ctx.App.Metadata = map[string]interface{}{}
	err := handleSettings(os.Stdout, ctx)
	assert.Error(t, err)
}

func TestEnvHandleSettingsInvalidFile(t *testing.T) {
	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"
	ctx.App.Metadata["configFile"] = "foo"
	ctx.App.Metadata["runtimeConfig"] = oci.RuntimeConfig{}

	err := handleSettings(nil, ctx)
	assert.Error(t, err)
}

func TestEnvHandleSettingsInvalidConfigFileType(t *testing.T) {
	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"
	ctx.App.Metadata["configFile"] = 123
	ctx.App.Metadata["runtimeConfig"] = oci.RuntimeConfig{}

	err := handleSettings(os.Stderr, ctx)
	assert.Error(t, err)
}

func TestEnvHandleSettingsInvalidRuntimeConfigType(t *testing.T) {
	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"
	ctx.App.Metadata["configFile"] = "/some/where"
	ctx.App.Metadata["runtimeConfig"] = true

	err := handleSettings(os.Stderr, ctx)
	assert.Error(t, err)
}

func TestEnvCLIFunction(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := createCLIContextWithApp(set, app)
	app.Name = "foo"

	ctx.App.Metadata["configFile"] = configFile
	ctx.App.Metadata["runtimeConfig"] = config

	fn, ok := kataEnvCLICommand.Action.(func(context *cli.Context) error)
	assert.True(t, ok)

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
	assert.NoError(t, err)

	// throw away output
	savedOutputFile := defaultOutputFile
	defaultOutputFile = devNull

	defer func() {
		defaultOutputFile = savedOutputFile
	}()

	err = fn(ctx)
	assert.NoError(t, err)

	set.Bool("json", true, "")
	ctx = createCLIContextWithApp(set, app)

	err = fn(ctx)
	assert.NoError(t, err)
}

func TestEnvCLIFunctionFail(t *testing.T) {
	tmpdir := t.TempDir()

	configFile, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(t, err)

	_, err = getExpectedSettings(config, tmpdir, configFile)
	assert.NoError(t, err)

	ctx := createCLIContext(nil)
	ctx.App.Name = "foo"

	ctx.App.Metadata["configFile"] = configFile
	ctx.App.Metadata["runtimeConfig"] = config

	fn, ok := kataEnvCLICommand.Action.(func(context *cli.Context) error)
	assert.True(t, ok)

	savedOutputFile := defaultOutputFile
	// invalidate
	defaultOutputFile = nil

	defer func() {
		defaultOutputFile = savedOutputFile
	}()

	err = fn(ctx)
	assert.Error(t, err)
}

func TestGetHypervisorInfo(t *testing.T) {
	assert := assert.New(t)

	tmpdir := t.TempDir()

	_, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(err)

	info, err := getHypervisorInfo(config)
	assert.NoError(err)
	assert.Equal(info.Version, testHypervisorVersion)

	err = os.Remove(config.HypervisorConfig.HypervisorPath)
	assert.NoError(err)

	info, err = getHypervisorInfo(config)
	assert.NoError(err)
	assert.Equal(info.Version, unknown)
}

func TestGetHypervisorInfoSocket(t *testing.T) {
	assert := assert.New(t)

	tmpdir := t.TempDir()

	_, config, err := makeRuntimeConfig(tmpdir)
	assert.NoError(err)

	type TestHypervisorDetails struct {
		hType       vc.HypervisorType
		hybridVsock bool
	}

	hypervisors := []TestHypervisorDetails{
		{vc.ClhHypervisor, true},
		{vc.FirecrackerHypervisor, true},
		{vc.MockHypervisor, false},
		{vc.QemuHypervisor, false},
	}

	config.HypervisorConfig.VMStorePath = "/foo"
	config.HypervisorConfig.RunStorePath = "/bar"

	for i, details := range hypervisors {
		msg := fmt.Sprintf("hypervisor[%d]: %+v", i, details)

		config.HypervisorType = details.hType
		info, err := getHypervisorInfo(config)
		assert.NoError(err, msg)

		if os.Geteuid() == 0 {
			if !details.hybridVsock {
				assert.Equal(info.SocketPath, "", msg)
			} else {
				assert.NotEmpty(info.SocketPath, msg)
				assert.True(strings.HasPrefix(info.SocketPath, "/"), msg)
			}
		} else {
			assert.Equal(info.SocketPath, unknown, msg)
		}
	}
}
