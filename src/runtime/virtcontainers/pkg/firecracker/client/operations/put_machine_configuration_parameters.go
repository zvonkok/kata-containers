// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/firecracker/client/models"
)

// NewPutMachineConfigurationParams creates a new PutMachineConfigurationParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPutMachineConfigurationParams() *PutMachineConfigurationParams {
	return &PutMachineConfigurationParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewPutMachineConfigurationParamsWithTimeout creates a new PutMachineConfigurationParams object
// with the ability to set a timeout on a request.
func NewPutMachineConfigurationParamsWithTimeout(timeout time.Duration) *PutMachineConfigurationParams {
	return &PutMachineConfigurationParams{
		timeout: timeout,
	}
}

// NewPutMachineConfigurationParamsWithContext creates a new PutMachineConfigurationParams object
// with the ability to set a context for a request.
func NewPutMachineConfigurationParamsWithContext(ctx context.Context) *PutMachineConfigurationParams {
	return &PutMachineConfigurationParams{
		Context: ctx,
	}
}

// NewPutMachineConfigurationParamsWithHTTPClient creates a new PutMachineConfigurationParams object
// with the ability to set a custom HTTPClient for a request.
func NewPutMachineConfigurationParamsWithHTTPClient(client *http.Client) *PutMachineConfigurationParams {
	return &PutMachineConfigurationParams{
		HTTPClient: client,
	}
}

/* PutMachineConfigurationParams contains all the parameters to send to the API endpoint
   for the put machine configuration operation.

   Typically these are written to a http.Request.
*/
type PutMachineConfigurationParams struct {

	/* Body.

	   Machine Configuration Parameters
	*/
	Body *models.MachineConfiguration

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the put machine configuration params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutMachineConfigurationParams) WithDefaults() *PutMachineConfigurationParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the put machine configuration params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutMachineConfigurationParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the put machine configuration params
func (o *PutMachineConfigurationParams) WithTimeout(timeout time.Duration) *PutMachineConfigurationParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the put machine configuration params
func (o *PutMachineConfigurationParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the put machine configuration params
func (o *PutMachineConfigurationParams) WithContext(ctx context.Context) *PutMachineConfigurationParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the put machine configuration params
func (o *PutMachineConfigurationParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the put machine configuration params
func (o *PutMachineConfigurationParams) WithHTTPClient(client *http.Client) *PutMachineConfigurationParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the put machine configuration params
func (o *PutMachineConfigurationParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the put machine configuration params
func (o *PutMachineConfigurationParams) WithBody(body *models.MachineConfiguration) *PutMachineConfigurationParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the put machine configuration params
func (o *PutMachineConfigurationParams) SetBody(body *models.MachineConfiguration) {
	o.Body = body
}

// WriteToRequest writes these params to a swagger request
func (o *PutMachineConfigurationParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error
	if o.Body != nil {
		if err := r.SetBodyParam(o.Body); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
