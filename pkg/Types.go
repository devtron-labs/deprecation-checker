package pkg

import (
	"fmt"
	"github.com/xeipuuv/gojsonschema"
)



// ValidFormat is a type for quickly forcing
// new formats on the gojsonschema loader
type ValidFormat struct{}

// IsFormat always returns true and meets the
// gojsonschema.FormatChecker interface
func (f ValidFormat) IsFormat(input interface{}) bool {
	return true
}

// ValidationResult contains the details from
// validating a given Kubernetes resource
type ValidationResult struct {
	FileName               string
	Kind                   string
	APIVersion             string
	ValidatedAgainstSchema bool
	Errors                 []gojsonschema.ResultError
	ResourceName           string
	ResourceNamespace      string
}

// VersionKind returns a string representation of this result's apiVersion and kind
func (v *ValidationResult) VersionKind() string {
	return v.APIVersion + "/" + v.Kind
}

// QualifiedName returns a string of the [namespace.]name of the k8s resource
func (v *ValidationResult) QualifiedName() string {
	if v.ResourceName == "" {
		return "unknown"
	} else if v.ResourceNamespace == "" {
		return v.ResourceName
	} else {
		return fmt.Sprintf("%s.%s", v.ResourceNamespace, v.ResourceName)
	}
}
