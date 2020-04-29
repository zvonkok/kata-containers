// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/validate"
)

// CPUTemplate The CPU Template defines a set of flags to be disabled from the microvm so that the features exposed to the guest are the same as in the selected instance type.
// swagger:model CpuTemplate
type CPUTemplate string

const (

	// CPUTemplateC3 captures enum value "C3"
	CPUTemplateC3 CPUTemplate = "C3"

	// CPUTemplateT2 captures enum value "T2"
	CPUTemplateT2 CPUTemplate = "T2"
)

// for schema
var cpuTemplateEnum []interface{}

func init() {
	var res []CPUTemplate
	if err := json.Unmarshal([]byte(`["C3","T2"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		cpuTemplateEnum = append(cpuTemplateEnum, v)
	}
}

func (m CPUTemplate) validateCPUTemplateEnum(path, location string, value CPUTemplate) error {
	if err := validate.Enum(path, location, value, cpuTemplateEnum); err != nil {
		return err
	}
	return nil
}

// Validate validates this Cpu template
func (m CPUTemplate) Validate(formats strfmt.Registry) error {
	var res []error

	// value enum
	if err := m.validateCPUTemplateEnum("", "body", m); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
