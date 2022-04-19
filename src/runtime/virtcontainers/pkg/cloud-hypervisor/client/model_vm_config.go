/*
Cloud Hypervisor API

Local HTTP based API for managing and inspecting a cloud-hypervisor virtual machine.

API version: 0.3.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"encoding/json"
)

// VmConfig Virtual machine configuration
type VmConfig struct {
	Cpus      *CpusConfig             `json:"cpus,omitempty"`
	Memory    *MemoryConfig           `json:"memory,omitempty"`
	Kernel    KernelConfig            `json:"kernel"`
	Initramfs NullableInitramfsConfig `json:"initramfs,omitempty"`
	Cmdline   *CmdLineConfig          `json:"cmdline,omitempty"`
	Disks     *[]DiskConfig           `json:"disks,omitempty"`
	Net       *[]NetConfig            `json:"net,omitempty"`
	Rng       *RngConfig              `json:"rng,omitempty"`
	Balloon   *BalloonConfig          `json:"balloon,omitempty"`
	Fs        *[]FsConfig             `json:"fs,omitempty"`
	Pmem      *[]PmemConfig           `json:"pmem,omitempty"`
	Serial    *ConsoleConfig          `json:"serial,omitempty"`
	Console   *ConsoleConfig          `json:"console,omitempty"`
	Devices   *[]DeviceConfig         `json:"devices,omitempty"`
	Vdpa      *[]VdpaConfig           `json:"vdpa,omitempty"`
	Vsock     *VsockConfig            `json:"vsock,omitempty"`
	SgxEpc    *[]SgxEpcConfig         `json:"sgx_epc,omitempty"`
	Tdx       *TdxConfig              `json:"tdx,omitempty"`
	Numa      *[]NumaConfig           `json:"numa,omitempty"`
	Iommu     *bool                   `json:"iommu,omitempty"`
	Watchdog  *bool                   `json:"watchdog,omitempty"`
	Platform  *PlatformConfig         `json:"platform,omitempty"`
}

// NewVmConfig instantiates a new VmConfig object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewVmConfig(kernel KernelConfig) *VmConfig {
	this := VmConfig{}
	this.Kernel = kernel
	var iommu bool = false
	this.Iommu = &iommu
	var watchdog bool = false
	this.Watchdog = &watchdog
	return &this
}

// NewVmConfigWithDefaults instantiates a new VmConfig object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewVmConfigWithDefaults() *VmConfig {
	this := VmConfig{}
	var iommu bool = false
	this.Iommu = &iommu
	var watchdog bool = false
	this.Watchdog = &watchdog
	return &this
}

// GetCpus returns the Cpus field value if set, zero value otherwise.
func (o *VmConfig) GetCpus() CpusConfig {
	if o == nil || o.Cpus == nil {
		var ret CpusConfig
		return ret
	}
	return *o.Cpus
}

// GetCpusOk returns a tuple with the Cpus field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetCpusOk() (*CpusConfig, bool) {
	if o == nil || o.Cpus == nil {
		return nil, false
	}
	return o.Cpus, true
}

// HasCpus returns a boolean if a field has been set.
func (o *VmConfig) HasCpus() bool {
	if o != nil && o.Cpus != nil {
		return true
	}

	return false
}

// SetCpus gets a reference to the given CpusConfig and assigns it to the Cpus field.
func (o *VmConfig) SetCpus(v CpusConfig) {
	o.Cpus = &v
}

// GetMemory returns the Memory field value if set, zero value otherwise.
func (o *VmConfig) GetMemory() MemoryConfig {
	if o == nil || o.Memory == nil {
		var ret MemoryConfig
		return ret
	}
	return *o.Memory
}

// GetMemoryOk returns a tuple with the Memory field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetMemoryOk() (*MemoryConfig, bool) {
	if o == nil || o.Memory == nil {
		return nil, false
	}
	return o.Memory, true
}

// HasMemory returns a boolean if a field has been set.
func (o *VmConfig) HasMemory() bool {
	if o != nil && o.Memory != nil {
		return true
	}

	return false
}

// SetMemory gets a reference to the given MemoryConfig and assigns it to the Memory field.
func (o *VmConfig) SetMemory(v MemoryConfig) {
	o.Memory = &v
}

// GetKernel returns the Kernel field value
func (o *VmConfig) GetKernel() KernelConfig {
	if o == nil {
		var ret KernelConfig
		return ret
	}

	return o.Kernel
}

// GetKernelOk returns a tuple with the Kernel field value
// and a boolean to check if the value has been set.
func (o *VmConfig) GetKernelOk() (*KernelConfig, bool) {
	if o == nil {
		return nil, false
	}
	return &o.Kernel, true
}

// SetKernel sets field value
func (o *VmConfig) SetKernel(v KernelConfig) {
	o.Kernel = v
}

// GetInitramfs returns the Initramfs field value if set, zero value otherwise (both if not set or set to explicit null).
func (o *VmConfig) GetInitramfs() InitramfsConfig {
	if o == nil || o.Initramfs.Get() == nil {
		var ret InitramfsConfig
		return ret
	}
	return *o.Initramfs.Get()
}

// GetInitramfsOk returns a tuple with the Initramfs field value if set, nil otherwise
// and a boolean to check if the value has been set.
// NOTE: If the value is an explicit nil, `nil, true` will be returned
func (o *VmConfig) GetInitramfsOk() (*InitramfsConfig, bool) {
	if o == nil {
		return nil, false
	}
	return o.Initramfs.Get(), o.Initramfs.IsSet()
}

// HasInitramfs returns a boolean if a field has been set.
func (o *VmConfig) HasInitramfs() bool {
	if o != nil && o.Initramfs.IsSet() {
		return true
	}

	return false
}

// SetInitramfs gets a reference to the given NullableInitramfsConfig and assigns it to the Initramfs field.
func (o *VmConfig) SetInitramfs(v InitramfsConfig) {
	o.Initramfs.Set(&v)
}

// SetInitramfsNil sets the value for Initramfs to be an explicit nil
func (o *VmConfig) SetInitramfsNil() {
	o.Initramfs.Set(nil)
}

// UnsetInitramfs ensures that no value is present for Initramfs, not even an explicit nil
func (o *VmConfig) UnsetInitramfs() {
	o.Initramfs.Unset()
}

// GetCmdline returns the Cmdline field value if set, zero value otherwise.
func (o *VmConfig) GetCmdline() CmdLineConfig {
	if o == nil || o.Cmdline == nil {
		var ret CmdLineConfig
		return ret
	}
	return *o.Cmdline
}

// GetCmdlineOk returns a tuple with the Cmdline field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetCmdlineOk() (*CmdLineConfig, bool) {
	if o == nil || o.Cmdline == nil {
		return nil, false
	}
	return o.Cmdline, true
}

// HasCmdline returns a boolean if a field has been set.
func (o *VmConfig) HasCmdline() bool {
	if o != nil && o.Cmdline != nil {
		return true
	}

	return false
}

// SetCmdline gets a reference to the given CmdLineConfig and assigns it to the Cmdline field.
func (o *VmConfig) SetCmdline(v CmdLineConfig) {
	o.Cmdline = &v
}

// GetDisks returns the Disks field value if set, zero value otherwise.
func (o *VmConfig) GetDisks() []DiskConfig {
	if o == nil || o.Disks == nil {
		var ret []DiskConfig
		return ret
	}
	return *o.Disks
}

// GetDisksOk returns a tuple with the Disks field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetDisksOk() (*[]DiskConfig, bool) {
	if o == nil || o.Disks == nil {
		return nil, false
	}
	return o.Disks, true
}

// HasDisks returns a boolean if a field has been set.
func (o *VmConfig) HasDisks() bool {
	if o != nil && o.Disks != nil {
		return true
	}

	return false
}

// SetDisks gets a reference to the given []DiskConfig and assigns it to the Disks field.
func (o *VmConfig) SetDisks(v []DiskConfig) {
	o.Disks = &v
}

// GetNet returns the Net field value if set, zero value otherwise.
func (o *VmConfig) GetNet() []NetConfig {
	if o == nil || o.Net == nil {
		var ret []NetConfig
		return ret
	}
	return *o.Net
}

// GetNetOk returns a tuple with the Net field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetNetOk() (*[]NetConfig, bool) {
	if o == nil || o.Net == nil {
		return nil, false
	}
	return o.Net, true
}

// HasNet returns a boolean if a field has been set.
func (o *VmConfig) HasNet() bool {
	if o != nil && o.Net != nil {
		return true
	}

	return false
}

// SetNet gets a reference to the given []NetConfig and assigns it to the Net field.
func (o *VmConfig) SetNet(v []NetConfig) {
	o.Net = &v
}

// GetRng returns the Rng field value if set, zero value otherwise.
func (o *VmConfig) GetRng() RngConfig {
	if o == nil || o.Rng == nil {
		var ret RngConfig
		return ret
	}
	return *o.Rng
}

// GetRngOk returns a tuple with the Rng field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetRngOk() (*RngConfig, bool) {
	if o == nil || o.Rng == nil {
		return nil, false
	}
	return o.Rng, true
}

// HasRng returns a boolean if a field has been set.
func (o *VmConfig) HasRng() bool {
	if o != nil && o.Rng != nil {
		return true
	}

	return false
}

// SetRng gets a reference to the given RngConfig and assigns it to the Rng field.
func (o *VmConfig) SetRng(v RngConfig) {
	o.Rng = &v
}

// GetBalloon returns the Balloon field value if set, zero value otherwise.
func (o *VmConfig) GetBalloon() BalloonConfig {
	if o == nil || o.Balloon == nil {
		var ret BalloonConfig
		return ret
	}
	return *o.Balloon
}

// GetBalloonOk returns a tuple with the Balloon field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetBalloonOk() (*BalloonConfig, bool) {
	if o == nil || o.Balloon == nil {
		return nil, false
	}
	return o.Balloon, true
}

// HasBalloon returns a boolean if a field has been set.
func (o *VmConfig) HasBalloon() bool {
	if o != nil && o.Balloon != nil {
		return true
	}

	return false
}

// SetBalloon gets a reference to the given BalloonConfig and assigns it to the Balloon field.
func (o *VmConfig) SetBalloon(v BalloonConfig) {
	o.Balloon = &v
}

// GetFs returns the Fs field value if set, zero value otherwise.
func (o *VmConfig) GetFs() []FsConfig {
	if o == nil || o.Fs == nil {
		var ret []FsConfig
		return ret
	}
	return *o.Fs
}

// GetFsOk returns a tuple with the Fs field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetFsOk() (*[]FsConfig, bool) {
	if o == nil || o.Fs == nil {
		return nil, false
	}
	return o.Fs, true
}

// HasFs returns a boolean if a field has been set.
func (o *VmConfig) HasFs() bool {
	if o != nil && o.Fs != nil {
		return true
	}

	return false
}

// SetFs gets a reference to the given []FsConfig and assigns it to the Fs field.
func (o *VmConfig) SetFs(v []FsConfig) {
	o.Fs = &v
}

// GetPmem returns the Pmem field value if set, zero value otherwise.
func (o *VmConfig) GetPmem() []PmemConfig {
	if o == nil || o.Pmem == nil {
		var ret []PmemConfig
		return ret
	}
	return *o.Pmem
}

// GetPmemOk returns a tuple with the Pmem field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetPmemOk() (*[]PmemConfig, bool) {
	if o == nil || o.Pmem == nil {
		return nil, false
	}
	return o.Pmem, true
}

// HasPmem returns a boolean if a field has been set.
func (o *VmConfig) HasPmem() bool {
	if o != nil && o.Pmem != nil {
		return true
	}

	return false
}

// SetPmem gets a reference to the given []PmemConfig and assigns it to the Pmem field.
func (o *VmConfig) SetPmem(v []PmemConfig) {
	o.Pmem = &v
}

// GetSerial returns the Serial field value if set, zero value otherwise.
func (o *VmConfig) GetSerial() ConsoleConfig {
	if o == nil || o.Serial == nil {
		var ret ConsoleConfig
		return ret
	}
	return *o.Serial
}

// GetSerialOk returns a tuple with the Serial field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetSerialOk() (*ConsoleConfig, bool) {
	if o == nil || o.Serial == nil {
		return nil, false
	}
	return o.Serial, true
}

// HasSerial returns a boolean if a field has been set.
func (o *VmConfig) HasSerial() bool {
	if o != nil && o.Serial != nil {
		return true
	}

	return false
}

// SetSerial gets a reference to the given ConsoleConfig and assigns it to the Serial field.
func (o *VmConfig) SetSerial(v ConsoleConfig) {
	o.Serial = &v
}

// GetConsole returns the Console field value if set, zero value otherwise.
func (o *VmConfig) GetConsole() ConsoleConfig {
	if o == nil || o.Console == nil {
		var ret ConsoleConfig
		return ret
	}
	return *o.Console
}

// GetConsoleOk returns a tuple with the Console field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetConsoleOk() (*ConsoleConfig, bool) {
	if o == nil || o.Console == nil {
		return nil, false
	}
	return o.Console, true
}

// HasConsole returns a boolean if a field has been set.
func (o *VmConfig) HasConsole() bool {
	if o != nil && o.Console != nil {
		return true
	}

	return false
}

// SetConsole gets a reference to the given ConsoleConfig and assigns it to the Console field.
func (o *VmConfig) SetConsole(v ConsoleConfig) {
	o.Console = &v
}

// GetDevices returns the Devices field value if set, zero value otherwise.
func (o *VmConfig) GetDevices() []DeviceConfig {
	if o == nil || o.Devices == nil {
		var ret []DeviceConfig
		return ret
	}
	return *o.Devices
}

// GetDevicesOk returns a tuple with the Devices field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetDevicesOk() (*[]DeviceConfig, bool) {
	if o == nil || o.Devices == nil {
		return nil, false
	}
	return o.Devices, true
}

// HasDevices returns a boolean if a field has been set.
func (o *VmConfig) HasDevices() bool {
	if o != nil && o.Devices != nil {
		return true
	}

	return false
}

// SetDevices gets a reference to the given []DeviceConfig and assigns it to the Devices field.
func (o *VmConfig) SetDevices(v []DeviceConfig) {
	o.Devices = &v
}

// GetVdpa returns the Vdpa field value if set, zero value otherwise.
func (o *VmConfig) GetVdpa() []VdpaConfig {
	if o == nil || o.Vdpa == nil {
		var ret []VdpaConfig
		return ret
	}
	return *o.Vdpa
}

// GetVdpaOk returns a tuple with the Vdpa field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetVdpaOk() (*[]VdpaConfig, bool) {
	if o == nil || o.Vdpa == nil {
		return nil, false
	}
	return o.Vdpa, true
}

// HasVdpa returns a boolean if a field has been set.
func (o *VmConfig) HasVdpa() bool {
	if o != nil && o.Vdpa != nil {
		return true
	}

	return false
}

// SetVdpa gets a reference to the given []VdpaConfig and assigns it to the Vdpa field.
func (o *VmConfig) SetVdpa(v []VdpaConfig) {
	o.Vdpa = &v
}

// GetVsock returns the Vsock field value if set, zero value otherwise.
func (o *VmConfig) GetVsock() VsockConfig {
	if o == nil || o.Vsock == nil {
		var ret VsockConfig
		return ret
	}
	return *o.Vsock
}

// GetVsockOk returns a tuple with the Vsock field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetVsockOk() (*VsockConfig, bool) {
	if o == nil || o.Vsock == nil {
		return nil, false
	}
	return o.Vsock, true
}

// HasVsock returns a boolean if a field has been set.
func (o *VmConfig) HasVsock() bool {
	if o != nil && o.Vsock != nil {
		return true
	}

	return false
}

// SetVsock gets a reference to the given VsockConfig and assigns it to the Vsock field.
func (o *VmConfig) SetVsock(v VsockConfig) {
	o.Vsock = &v
}

// GetSgxEpc returns the SgxEpc field value if set, zero value otherwise.
func (o *VmConfig) GetSgxEpc() []SgxEpcConfig {
	if o == nil || o.SgxEpc == nil {
		var ret []SgxEpcConfig
		return ret
	}
	return *o.SgxEpc
}

// GetSgxEpcOk returns a tuple with the SgxEpc field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetSgxEpcOk() (*[]SgxEpcConfig, bool) {
	if o == nil || o.SgxEpc == nil {
		return nil, false
	}
	return o.SgxEpc, true
}

// HasSgxEpc returns a boolean if a field has been set.
func (o *VmConfig) HasSgxEpc() bool {
	if o != nil && o.SgxEpc != nil {
		return true
	}

	return false
}

// SetSgxEpc gets a reference to the given []SgxEpcConfig and assigns it to the SgxEpc field.
func (o *VmConfig) SetSgxEpc(v []SgxEpcConfig) {
	o.SgxEpc = &v
}

// GetTdx returns the Tdx field value if set, zero value otherwise.
func (o *VmConfig) GetTdx() TdxConfig {
	if o == nil || o.Tdx == nil {
		var ret TdxConfig
		return ret
	}
	return *o.Tdx
}

// GetTdxOk returns a tuple with the Tdx field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetTdxOk() (*TdxConfig, bool) {
	if o == nil || o.Tdx == nil {
		return nil, false
	}
	return o.Tdx, true
}

// HasTdx returns a boolean if a field has been set.
func (o *VmConfig) HasTdx() bool {
	if o != nil && o.Tdx != nil {
		return true
	}

	return false
}

// SetTdx gets a reference to the given TdxConfig and assigns it to the Tdx field.
func (o *VmConfig) SetTdx(v TdxConfig) {
	o.Tdx = &v
}

// GetNuma returns the Numa field value if set, zero value otherwise.
func (o *VmConfig) GetNuma() []NumaConfig {
	if o == nil || o.Numa == nil {
		var ret []NumaConfig
		return ret
	}
	return *o.Numa
}

// GetNumaOk returns a tuple with the Numa field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetNumaOk() (*[]NumaConfig, bool) {
	if o == nil || o.Numa == nil {
		return nil, false
	}
	return o.Numa, true
}

// HasNuma returns a boolean if a field has been set.
func (o *VmConfig) HasNuma() bool {
	if o != nil && o.Numa != nil {
		return true
	}

	return false
}

// SetNuma gets a reference to the given []NumaConfig and assigns it to the Numa field.
func (o *VmConfig) SetNuma(v []NumaConfig) {
	o.Numa = &v
}

// GetIommu returns the Iommu field value if set, zero value otherwise.
func (o *VmConfig) GetIommu() bool {
	if o == nil || o.Iommu == nil {
		var ret bool
		return ret
	}
	return *o.Iommu
}

// GetIommuOk returns a tuple with the Iommu field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetIommuOk() (*bool, bool) {
	if o == nil || o.Iommu == nil {
		return nil, false
	}
	return o.Iommu, true
}

// HasIommu returns a boolean if a field has been set.
func (o *VmConfig) HasIommu() bool {
	if o != nil && o.Iommu != nil {
		return true
	}

	return false
}

// SetIommu gets a reference to the given bool and assigns it to the Iommu field.
func (o *VmConfig) SetIommu(v bool) {
	o.Iommu = &v
}

// GetWatchdog returns the Watchdog field value if set, zero value otherwise.
func (o *VmConfig) GetWatchdog() bool {
	if o == nil || o.Watchdog == nil {
		var ret bool
		return ret
	}
	return *o.Watchdog
}

// GetWatchdogOk returns a tuple with the Watchdog field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetWatchdogOk() (*bool, bool) {
	if o == nil || o.Watchdog == nil {
		return nil, false
	}
	return o.Watchdog, true
}

// HasWatchdog returns a boolean if a field has been set.
func (o *VmConfig) HasWatchdog() bool {
	if o != nil && o.Watchdog != nil {
		return true
	}

	return false
}

// SetWatchdog gets a reference to the given bool and assigns it to the Watchdog field.
func (o *VmConfig) SetWatchdog(v bool) {
	o.Watchdog = &v
}

// GetPlatform returns the Platform field value if set, zero value otherwise.
func (o *VmConfig) GetPlatform() PlatformConfig {
	if o == nil || o.Platform == nil {
		var ret PlatformConfig
		return ret
	}
	return *o.Platform
}

// GetPlatformOk returns a tuple with the Platform field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *VmConfig) GetPlatformOk() (*PlatformConfig, bool) {
	if o == nil || o.Platform == nil {
		return nil, false
	}
	return o.Platform, true
}

// HasPlatform returns a boolean if a field has been set.
func (o *VmConfig) HasPlatform() bool {
	if o != nil && o.Platform != nil {
		return true
	}

	return false
}

// SetPlatform gets a reference to the given PlatformConfig and assigns it to the Platform field.
func (o *VmConfig) SetPlatform(v PlatformConfig) {
	o.Platform = &v
}

func (o VmConfig) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Cpus != nil {
		toSerialize["cpus"] = o.Cpus
	}
	if o.Memory != nil {
		toSerialize["memory"] = o.Memory
	}
	if true {
		toSerialize["kernel"] = o.Kernel
	}
	if o.Initramfs.IsSet() {
		toSerialize["initramfs"] = o.Initramfs.Get()
	}
	if o.Cmdline != nil {
		toSerialize["cmdline"] = o.Cmdline
	}
	if o.Disks != nil {
		toSerialize["disks"] = o.Disks
	}
	if o.Net != nil {
		toSerialize["net"] = o.Net
	}
	if o.Rng != nil {
		toSerialize["rng"] = o.Rng
	}
	if o.Balloon != nil {
		toSerialize["balloon"] = o.Balloon
	}
	if o.Fs != nil {
		toSerialize["fs"] = o.Fs
	}
	if o.Pmem != nil {
		toSerialize["pmem"] = o.Pmem
	}
	if o.Serial != nil {
		toSerialize["serial"] = o.Serial
	}
	if o.Console != nil {
		toSerialize["console"] = o.Console
	}
	if o.Devices != nil {
		toSerialize["devices"] = o.Devices
	}
	if o.Vdpa != nil {
		toSerialize["vdpa"] = o.Vdpa
	}
	if o.Vsock != nil {
		toSerialize["vsock"] = o.Vsock
	}
	if o.SgxEpc != nil {
		toSerialize["sgx_epc"] = o.SgxEpc
	}
	if o.Tdx != nil {
		toSerialize["tdx"] = o.Tdx
	}
	if o.Numa != nil {
		toSerialize["numa"] = o.Numa
	}
	if o.Iommu != nil {
		toSerialize["iommu"] = o.Iommu
	}
	if o.Watchdog != nil {
		toSerialize["watchdog"] = o.Watchdog
	}
	if o.Platform != nil {
		toSerialize["platform"] = o.Platform
	}
	return json.Marshal(toSerialize)
}

type NullableVmConfig struct {
	value *VmConfig
	isSet bool
}

func (v NullableVmConfig) Get() *VmConfig {
	return v.value
}

func (v *NullableVmConfig) Set(val *VmConfig) {
	v.value = val
	v.isSet = true
}

func (v NullableVmConfig) IsSet() bool {
	return v.isSet
}

func (v *NullableVmConfig) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableVmConfig(val *VmConfig) *NullableVmConfig {
	return &NullableVmConfig{value: val, isSet: true}
}

func (v NullableVmConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableVmConfig) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
