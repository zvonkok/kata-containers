// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// New creates a new operations API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) ClientService {
	return &Client{transport: transport, formats: formats}
}

/*
Client for operations API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

// ClientOption is the option for Client methods
type ClientOption func(*runtime.ClientOperation)

// ClientService is the interface for Client methods
type ClientService interface {
	CreateSnapshot(params *CreateSnapshotParams, opts ...ClientOption) (*CreateSnapshotNoContent, error)

	CreateSyncAction(params *CreateSyncActionParams, opts ...ClientOption) (*CreateSyncActionNoContent, error)

	DescribeBalloonConfig(params *DescribeBalloonConfigParams, opts ...ClientOption) (*DescribeBalloonConfigOK, error)

	DescribeBalloonStats(params *DescribeBalloonStatsParams, opts ...ClientOption) (*DescribeBalloonStatsOK, error)

	DescribeInstance(params *DescribeInstanceParams, opts ...ClientOption) (*DescribeInstanceOK, error)

	GetExportVMConfig(params *GetExportVMConfigParams, opts ...ClientOption) (*GetExportVMConfigOK, error)

	GetFirecrackerVersion(params *GetFirecrackerVersionParams, opts ...ClientOption) (*GetFirecrackerVersionOK, error)

	GetMachineConfiguration(params *GetMachineConfigurationParams, opts ...ClientOption) (*GetMachineConfigurationOK, error)

	GetMmds(params *GetMmdsParams, opts ...ClientOption) (*GetMmdsOK, error)

	LoadSnapshot(params *LoadSnapshotParams, opts ...ClientOption) (*LoadSnapshotNoContent, error)

	PatchBalloon(params *PatchBalloonParams, opts ...ClientOption) (*PatchBalloonNoContent, error)

	PatchBalloonStatsInterval(params *PatchBalloonStatsIntervalParams, opts ...ClientOption) (*PatchBalloonStatsIntervalNoContent, error)

	PatchGuestDriveByID(params *PatchGuestDriveByIDParams, opts ...ClientOption) (*PatchGuestDriveByIDNoContent, error)

	PatchGuestNetworkInterfaceByID(params *PatchGuestNetworkInterfaceByIDParams, opts ...ClientOption) (*PatchGuestNetworkInterfaceByIDNoContent, error)

	PatchMachineConfiguration(params *PatchMachineConfigurationParams, opts ...ClientOption) (*PatchMachineConfigurationNoContent, error)

	PatchMmds(params *PatchMmdsParams, opts ...ClientOption) (*PatchMmdsNoContent, error)

	PatchVM(params *PatchVMParams, opts ...ClientOption) (*PatchVMNoContent, error)

	PutBalloon(params *PutBalloonParams, opts ...ClientOption) (*PutBalloonNoContent, error)

	PutGuestBootSource(params *PutGuestBootSourceParams, opts ...ClientOption) (*PutGuestBootSourceNoContent, error)

	PutGuestDriveByID(params *PutGuestDriveByIDParams, opts ...ClientOption) (*PutGuestDriveByIDNoContent, error)

	PutGuestNetworkInterfaceByID(params *PutGuestNetworkInterfaceByIDParams, opts ...ClientOption) (*PutGuestNetworkInterfaceByIDNoContent, error)

	PutGuestVsock(params *PutGuestVsockParams, opts ...ClientOption) (*PutGuestVsockNoContent, error)

	PutLogger(params *PutLoggerParams, opts ...ClientOption) (*PutLoggerNoContent, error)

	PutMachineConfiguration(params *PutMachineConfigurationParams, opts ...ClientOption) (*PutMachineConfigurationNoContent, error)

	PutMetrics(params *PutMetricsParams, opts ...ClientOption) (*PutMetricsNoContent, error)

	PutMmds(params *PutMmdsParams, opts ...ClientOption) (*PutMmdsNoContent, error)

	PutMmdsConfig(params *PutMmdsConfigParams, opts ...ClientOption) (*PutMmdsConfigNoContent, error)

	SetTransport(transport runtime.ClientTransport)
}

/*
  CreateSnapshot creates a full or diff snapshot post boot only

  Creates a snapshot of the microVM state. The microVM should be in the `Paused` state.
*/
func (a *Client) CreateSnapshot(params *CreateSnapshotParams, opts ...ClientOption) (*CreateSnapshotNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateSnapshotParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "createSnapshot",
		Method:             "PUT",
		PathPattern:        "/snapshot/create",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateSnapshotReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateSnapshotNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*CreateSnapshotDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  CreateSyncAction creates a synchronous action
*/
func (a *Client) CreateSyncAction(params *CreateSyncActionParams, opts ...ClientOption) (*CreateSyncActionNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateSyncActionParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "createSyncAction",
		Method:             "PUT",
		PathPattern:        "/actions",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateSyncActionReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateSyncActionNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*CreateSyncActionDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  DescribeBalloonConfig returns the current balloon device configuration
*/
func (a *Client) DescribeBalloonConfig(params *DescribeBalloonConfigParams, opts ...ClientOption) (*DescribeBalloonConfigOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDescribeBalloonConfigParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "describeBalloonConfig",
		Method:             "GET",
		PathPattern:        "/balloon",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DescribeBalloonConfigReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DescribeBalloonConfigOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*DescribeBalloonConfigDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  DescribeBalloonStats returns the latest balloon device statistics only if enabled pre boot
*/
func (a *Client) DescribeBalloonStats(params *DescribeBalloonStatsParams, opts ...ClientOption) (*DescribeBalloonStatsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDescribeBalloonStatsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "describeBalloonStats",
		Method:             "GET",
		PathPattern:        "/balloon/statistics",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DescribeBalloonStatsReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DescribeBalloonStatsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*DescribeBalloonStatsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  DescribeInstance returns general information about an instance
*/
func (a *Client) DescribeInstance(params *DescribeInstanceParams, opts ...ClientOption) (*DescribeInstanceOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDescribeInstanceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "describeInstance",
		Method:             "GET",
		PathPattern:        "/",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DescribeInstanceReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*DescribeInstanceOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*DescribeInstanceDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  GetExportVMConfig gets the full VM configuration

  Gets configuration for all VM resources. If the VM is restored from a snapshot, the boot-source, machine-config.smt and machine-config.cpu_template will be empty.
*/
func (a *Client) GetExportVMConfig(params *GetExportVMConfigParams, opts ...ClientOption) (*GetExportVMConfigOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetExportVMConfigParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "getExportVmConfig",
		Method:             "GET",
		PathPattern:        "/vm/config",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetExportVMConfigReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetExportVMConfigOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetExportVMConfigDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  GetFirecrackerVersion gets the firecracker version
*/
func (a *Client) GetFirecrackerVersion(params *GetFirecrackerVersionParams, opts ...ClientOption) (*GetFirecrackerVersionOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetFirecrackerVersionParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "getFirecrackerVersion",
		Method:             "GET",
		PathPattern:        "/version",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetFirecrackerVersionReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetFirecrackerVersionOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetFirecrackerVersionDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  GetMachineConfiguration gets the machine configuration of the VM

  Gets the machine configuration of the VM. When called before the PUT operation, it will return the default values for the vCPU count (=1), memory size (=128 MiB). By default SMT is disabled and there is no CPU Template.
*/
func (a *Client) GetMachineConfiguration(params *GetMachineConfigurationParams, opts ...ClientOption) (*GetMachineConfigurationOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetMachineConfigurationParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "getMachineConfiguration",
		Method:             "GET",
		PathPattern:        "/machine-config",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetMachineConfigurationReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetMachineConfigurationOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetMachineConfigurationDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  GetMmds gets the m m d s data store
*/
func (a *Client) GetMmds(params *GetMmdsParams, opts ...ClientOption) (*GetMmdsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetMmdsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "getMmds",
		Method:             "GET",
		PathPattern:        "/mmds",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetMmdsReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetMmdsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetMmdsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  LoadSnapshot loads a snapshot pre boot only

  Loads the microVM state from a snapshot. Only accepted on a fresh Firecracker process (before configuring any resource other than the Logger and Metrics).
*/
func (a *Client) LoadSnapshot(params *LoadSnapshotParams, opts ...ClientOption) (*LoadSnapshotNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewLoadSnapshotParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "loadSnapshot",
		Method:             "PUT",
		PathPattern:        "/snapshot/load",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &LoadSnapshotReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*LoadSnapshotNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*LoadSnapshotDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchBalloon updates a balloon device

  Updates an existing balloon device, before or after machine startup. Will fail if update is not possible.
*/
func (a *Client) PatchBalloon(params *PatchBalloonParams, opts ...ClientOption) (*PatchBalloonNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchBalloonParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchBalloon",
		Method:             "PATCH",
		PathPattern:        "/balloon",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchBalloonReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchBalloonNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchBalloonDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchBalloonStatsInterval updates a balloon device statistics polling interval

  Updates an existing balloon device statistics interval, before or after machine startup. Will fail if update is not possible.
*/
func (a *Client) PatchBalloonStatsInterval(params *PatchBalloonStatsIntervalParams, opts ...ClientOption) (*PatchBalloonStatsIntervalNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchBalloonStatsIntervalParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchBalloonStatsInterval",
		Method:             "PATCH",
		PathPattern:        "/balloon/statistics",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchBalloonStatsIntervalReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchBalloonStatsIntervalNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchBalloonStatsIntervalDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchGuestDriveByID updates the properties of a drive post boot only

  Updates the properties of the drive with the ID specified by drive_id path parameter. Will fail if update is not possible.
*/
func (a *Client) PatchGuestDriveByID(params *PatchGuestDriveByIDParams, opts ...ClientOption) (*PatchGuestDriveByIDNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchGuestDriveByIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchGuestDriveByID",
		Method:             "PATCH",
		PathPattern:        "/drives/{drive_id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchGuestDriveByIDReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchGuestDriveByIDNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchGuestDriveByIDDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchGuestNetworkInterfaceByID updates the rate limiters applied to a network interface post boot only

  Updates the rate limiters applied to a network interface.
*/
func (a *Client) PatchGuestNetworkInterfaceByID(params *PatchGuestNetworkInterfaceByIDParams, opts ...ClientOption) (*PatchGuestNetworkInterfaceByIDNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchGuestNetworkInterfaceByIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchGuestNetworkInterfaceByID",
		Method:             "PATCH",
		PathPattern:        "/network-interfaces/{iface_id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchGuestNetworkInterfaceByIDReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchGuestNetworkInterfaceByIDNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchGuestNetworkInterfaceByIDDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchMachineConfiguration partiallies updates the machine configuration of the VM pre boot only

  Partially updates the Virtual Machine Configuration with the specified input. If any of the parameters has an incorrect value, the whole update fails.
*/
func (a *Client) PatchMachineConfiguration(params *PatchMachineConfigurationParams, opts ...ClientOption) (*PatchMachineConfigurationNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchMachineConfigurationParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchMachineConfiguration",
		Method:             "PATCH",
		PathPattern:        "/machine-config",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchMachineConfigurationReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchMachineConfigurationNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchMachineConfigurationDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchMmds updates the m m d s data store
*/
func (a *Client) PatchMmds(params *PatchMmdsParams, opts ...ClientOption) (*PatchMmdsNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchMmdsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchMmds",
		Method:             "PATCH",
		PathPattern:        "/mmds",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchMmdsReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchMmdsNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchMmdsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PatchVM updates the micro VM state

  Sets the desired state (Paused or Resumed) for the microVM.
*/
func (a *Client) PatchVM(params *PatchVMParams, opts ...ClientOption) (*PatchVMNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPatchVMParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "patchVm",
		Method:             "PATCH",
		PathPattern:        "/vm",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PatchVMReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PatchVMNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PatchVMDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutBalloon creates or updates a balloon device

  Creates a new balloon device if one does not already exist, otherwise updates it, before machine startup. This will fail after machine startup. Will fail if update is not possible.
*/
func (a *Client) PutBalloon(params *PutBalloonParams, opts ...ClientOption) (*PutBalloonNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutBalloonParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putBalloon",
		Method:             "PUT",
		PathPattern:        "/balloon",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutBalloonReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutBalloonNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutBalloonDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutGuestBootSource creates or updates the boot source pre boot only

  Creates new boot source if one does not already exist, otherwise updates it. Will fail if update is not possible.
*/
func (a *Client) PutGuestBootSource(params *PutGuestBootSourceParams, opts ...ClientOption) (*PutGuestBootSourceNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutGuestBootSourceParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putGuestBootSource",
		Method:             "PUT",
		PathPattern:        "/boot-source",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutGuestBootSourceReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutGuestBootSourceNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutGuestBootSourceDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutGuestDriveByID creates or updates a drive pre boot only

  Creates new drive with ID specified by drive_id path parameter. If a drive with the specified ID already exists, updates its state based on new input. Will fail if update is not possible.
*/
func (a *Client) PutGuestDriveByID(params *PutGuestDriveByIDParams, opts ...ClientOption) (*PutGuestDriveByIDNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutGuestDriveByIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putGuestDriveByID",
		Method:             "PUT",
		PathPattern:        "/drives/{drive_id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutGuestDriveByIDReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutGuestDriveByIDNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutGuestDriveByIDDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutGuestNetworkInterfaceByID creates a network interface pre boot only

  Creates new network interface with ID specified by iface_id path parameter.
*/
func (a *Client) PutGuestNetworkInterfaceByID(params *PutGuestNetworkInterfaceByIDParams, opts ...ClientOption) (*PutGuestNetworkInterfaceByIDNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutGuestNetworkInterfaceByIDParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putGuestNetworkInterfaceByID",
		Method:             "PUT",
		PathPattern:        "/network-interfaces/{iface_id}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutGuestNetworkInterfaceByIDReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutGuestNetworkInterfaceByIDNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutGuestNetworkInterfaceByIDDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutGuestVsock creates updates a vsock device pre boot only

  The first call creates the device with the configuration specified in body. Subsequent calls will update the device configuration. May fail if update is not possible.
*/
func (a *Client) PutGuestVsock(params *PutGuestVsockParams, opts ...ClientOption) (*PutGuestVsockNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutGuestVsockParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putGuestVsock",
		Method:             "PUT",
		PathPattern:        "/vsock",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutGuestVsockReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutGuestVsockNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutGuestVsockDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutLogger initializes the logger by specifying a named pipe or a file for the logs output
*/
func (a *Client) PutLogger(params *PutLoggerParams, opts ...ClientOption) (*PutLoggerNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutLoggerParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putLogger",
		Method:             "PUT",
		PathPattern:        "/logger",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutLoggerReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutLoggerNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutLoggerDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutMachineConfiguration updates the machine configuration of the VM pre boot only

  Updates the Virtual Machine Configuration with the specified input. Firecracker starts with default values for vCPU count (=1) and memory size (=128 MiB). The vCPU count is restricted to the [1, 32] range. With SMT enabled, the vCPU count is required to be either 1 or an even number in the range. otherwise there are no restrictions regarding the vCPU count. If any of the parameters has an incorrect value, the whole update fails. All parameters that are optional and are not specified are set to their default values (smt = false, track_dirty_pages = false, cpu_template = None).
*/
func (a *Client) PutMachineConfiguration(params *PutMachineConfigurationParams, opts ...ClientOption) (*PutMachineConfigurationNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutMachineConfigurationParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putMachineConfiguration",
		Method:             "PUT",
		PathPattern:        "/machine-config",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutMachineConfigurationReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutMachineConfigurationNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutMachineConfigurationDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutMetrics initializes the metrics system by specifying a named pipe or a file for the metrics output
*/
func (a *Client) PutMetrics(params *PutMetricsParams, opts ...ClientOption) (*PutMetricsNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutMetricsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putMetrics",
		Method:             "PUT",
		PathPattern:        "/metrics",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutMetricsReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutMetricsNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutMetricsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutMmds creates a m m d s microvm metadata service data store
*/
func (a *Client) PutMmds(params *PutMmdsParams, opts ...ClientOption) (*PutMmdsNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutMmdsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putMmds",
		Method:             "PUT",
		PathPattern:        "/mmds",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutMmdsReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutMmdsNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutMmdsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
  PutMmdsConfig sets m m d s configuration pre boot only

  Configures MMDS version, IPv4 address used by the MMDS network stack and interfaces that allow MMDS requests.
*/
func (a *Client) PutMmdsConfig(params *PutMmdsConfigParams, opts ...ClientOption) (*PutMmdsConfigNoContent, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewPutMmdsConfigParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "putMmdsConfig",
		Method:             "PUT",
		PathPattern:        "/mmds/config",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &PutMmdsConfigReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*PutMmdsConfigNoContent)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*PutMmdsConfigDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}
