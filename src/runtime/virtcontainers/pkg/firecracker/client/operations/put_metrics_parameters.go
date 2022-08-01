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

// NewPutMetricsParams creates a new PutMetricsParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPutMetricsParams() *PutMetricsParams {
	return &PutMetricsParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewPutMetricsParamsWithTimeout creates a new PutMetricsParams object
// with the ability to set a timeout on a request.
func NewPutMetricsParamsWithTimeout(timeout time.Duration) *PutMetricsParams {
	return &PutMetricsParams{
		timeout: timeout,
	}
}

// NewPutMetricsParamsWithContext creates a new PutMetricsParams object
// with the ability to set a context for a request.
func NewPutMetricsParamsWithContext(ctx context.Context) *PutMetricsParams {
	return &PutMetricsParams{
		Context: ctx,
	}
}

// NewPutMetricsParamsWithHTTPClient creates a new PutMetricsParams object
// with the ability to set a custom HTTPClient for a request.
func NewPutMetricsParamsWithHTTPClient(client *http.Client) *PutMetricsParams {
	return &PutMetricsParams{
		HTTPClient: client,
	}
}

/* PutMetricsParams contains all the parameters to send to the API endpoint
   for the put metrics operation.

   Typically these are written to a http.Request.
*/
type PutMetricsParams struct {

	/* Body.

	   Metrics system description
	*/
	Body *models.Metrics

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the put metrics params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutMetricsParams) WithDefaults() *PutMetricsParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the put metrics params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PutMetricsParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the put metrics params
func (o *PutMetricsParams) WithTimeout(timeout time.Duration) *PutMetricsParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the put metrics params
func (o *PutMetricsParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the put metrics params
func (o *PutMetricsParams) WithContext(ctx context.Context) *PutMetricsParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the put metrics params
func (o *PutMetricsParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the put metrics params
func (o *PutMetricsParams) WithHTTPClient(client *http.Client) *PutMetricsParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the put metrics params
func (o *PutMetricsParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the put metrics params
func (o *PutMetricsParams) WithBody(body *models.Metrics) *PutMetricsParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the put metrics params
func (o *PutMetricsParams) SetBody(body *models.Metrics) {
	o.Body = body
}

// WriteToRequest writes these params to a swagger request
func (o *PutMetricsParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

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
