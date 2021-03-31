/*
 * Cloud Hypervisor API
 *
 * Local HTTP based API for managing and inspecting a cloud-hypervisor virtual machine.
 *
 * API version: 0.3.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package openapi
// VmResize struct for VmResize
type VmResize struct {
	DesiredVcpus int32 `json:"desired_vcpus,omitempty"`
	// desired memory ram in bytes
	DesiredRam int64 `json:"desired_ram,omitempty"`
	// desired balloon size in bytes
	DesiredBalloon int64 `json:"desired_balloon,omitempty"`
}
