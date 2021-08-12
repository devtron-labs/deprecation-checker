/*
 * Copyright (c) 2021 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/devtron-labs/deprecation-checker/pkg/log"
	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
	"strings"
)

func (ks *kubernetesSpec) ValidateYaml(spec string) (ValidationResult, error) {
	var err error
	jsonSpec, err := yaml.YAMLToJSON([]byte(spec))
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		return ValidationResult{}, err
	}
	return ks.ValidateJson(string(jsonSpec))
}

func (ks *kubernetesSpec) ValidateJson(spec string) (ValidationResult, error) {
	var err error
	object := make(map[string]interface{})
	err = json.Unmarshal([]byte(spec), &object)
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		return ValidationResult{}, err
	}
	return ks.ValidateObject(object)
}

func (ks *kubernetesSpec) ValidateObject(object map[string]interface{}) (ValidationResult, error) {
	validationResult, err := ks.populateValidationResult(object)
	validationResult.ValidatedAgainstSchema = true
	if err != nil {
		return validationResult, err
	}
	original, latest, err := ks.getKindsMappings(object)
	if err != nil {
		return validationResult, err
	}
	if len(original) > 0 {
		var ves []*openapi3.SchemaError
		var des []*SchemaError
		validationError, deprecated := ks.applySchema(object, original)
		if validationError != nil && len(validationError) > 0 {
			errs := []error(validationError)
			for _, e := range errs {
				//TODO: handle error of type SchemaError along with openapi3.SchemaError
				if se, ok := e.(*openapi3.SchemaError); ok {
					ves = append(ves, se)
				} else if de, ok := e.(*SchemaError); ok {
					des = append(des, de)
				}
			}
		}
		validationResult.ErrorsForOriginal = ves
		validationResult.DeprecationForOriginal = des
		validationResult.Deprecated = deprecated
	} else {
		validationResult.Deleted = true
	}
	if len(latest) > 0 && original != latest {
		var ves []*openapi3.SchemaError
		var des []*SchemaError
		validationError, _ := ks.applySchema(object, latest)
		if validationError != nil && len(validationError) > 0 {
			errs := []error(validationError)
			for _, e := range errs {
				//TODO: handle error of type SchemaError along with openapi3.SchemaError
				if se, ok := e.(*openapi3.SchemaError); ok {
					ves = append(ves, se)
				} else if de, ok := e.(*SchemaError); ok {
					des = append(des, de)
				}
			}
		}
		validationResult.ErrorsForLatest = ves
		validationResult.DeprecationForLatest = des
		validationResult.LatestAPIVersion, err = ks.getGVFromToken(latest)
	}
	return validationResult, nil
}

func (ks *kubernetesSpec) populateValidationResult(object map[string]interface{}) (ValidationResult, error) {
	validationResult := ValidationResult{}
	namespace := "undefined"
	if object == nil {
		return validationResult, fmt.Errorf("missing k8s object")
	}
	apiVersion, ok := object["apiVersion"].(string)
	kind, ok := object["kind"].(string)
	if !ok {
		return validationResult, fmt.Errorf("missing kind")
	}
	metadata, ok := object["metadata"].(map[string]interface{})
	if !ok {
		return validationResult, fmt.Errorf("missing metadata")
	}
	if ns, ok := metadata["namespace"]; ok {
		namespace = ns.(string)
	}
	name, ok := metadata["name"].(string)
	if !ok {
		return validationResult, fmt.Errorf("missing resource name")
	}
	validationResult.Kind = kind
	validationResult.APIVersion = apiVersion
	validationResult.ResourceNamespace = namespace
	validationResult.ResourceName = name
	return validationResult, nil
}

func (ks *kubernetesSpec) applySchema(object map[string]interface{}, token string) (openapi3.MultiError, bool) {
	deprecated := false
	var validationError openapi3.MultiError
	dp, err := ks.Components.Schemas.JSONLookup(token)
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		validationError = append(validationError, err)
		return validationError, deprecated
	}
	scm := dp.(*openapi3.Schema)
	opts := []openapi3.SchemaValidationOption{openapi3.MultiErrors()}
	depError := VisitJSON(token, scm, object, SchemaSettings{MultiError: true})
	if len(depError) > 0 {
		deprecated = true
	}
	validationError = append(validationError, depError...)

	err = scm.VisitJSON(object, opts...)
	if err != nil {
		e := err.(openapi3.MultiError)
		validationError = append(validationError, e...)
	}
	return validationError, deprecated
}

func (ks *kubernetesSpec) getGVFromToken(token string) (string, error) {
	dp, err := ks.Components.Schemas.JSONLookup(token)
	if err != nil {
		return "", err
	}
	scm := dp.(*openapi3.Schema)
	gv, err := getGV(scm.Extensions["x-kubernetes-group-version-kind"].(json.RawMessage))
	if err != nil {
		return "", err
	}
	return gv, nil
}

func (ks *kubernetesSpec) getKindsMappings(object map[string]interface{}) (original, latest string, err error) {
	original = ""
	latest = ""
	if object == nil {
		return "", "", fmt.Errorf("missing k8s object")
	}
	apiVersion, ok := object["apiVersion"].(string)
	kind, ok := object["kind"].(string)
	if !ok {
		return "", "", fmt.Errorf("missing kind")
	}
	gvk := strings.ToLower(fmt.Sprintf("%s/%s", apiVersion, kind))
	if _, ok := ks.pathMap[gvk]; ok {
		if component, ok := ks.componentMap[gvk]; ok {
			original = component
		}
	}

	if _, ok := ks.kindMap[strings.ToLower(kind)]; !ok {
		return original, "", nil
	}
	latest = ks.kindMap[strings.ToLower(kind)]
	return original, latest, nil
}