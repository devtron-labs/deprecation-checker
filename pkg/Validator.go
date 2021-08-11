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

func (ks *kubernetesSpec) ValidateYaml(spec string) error {
	var err error
	jsonSpec, err := yaml.YAMLToJSON([]byte(spec))
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		return err
	}
	return ks.ValidateJson(string(jsonSpec))
}

func (ks *kubernetesSpec) ValidateJson(spec string) error {
	var err error
	object := make(map[string]interface{})
	err = json.Unmarshal([]byte(spec), &object)
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		return err
	}
	return ks.ValidateObject(object)
}

func (ks *kubernetesSpec) ValidateObject(object map[string]interface{}) error {
	var validationError openapi3.MultiError
	original, latest, err := ks.getKindsMappings(object)
	if err != nil {
		return err
	}
	if len(original) > 0 {
		validationError = ks.applySchema(object, original)
	}
	if len(latest) > 0 && original != latest {
		ve := ks.applySchema(object, latest)
		if ve != nil {
			validationError = append(validationError, ve...)
		}
	}
	if len(validationError) == 0 {
		return nil
	}
	return validationError
}

func (ks *kubernetesSpec) applySchema(object map[string]interface{}, token string) openapi3.MultiError {
	var validationError openapi3.MultiError
	dp, err := ks.Components.Schemas.JSONLookup(token)
	if err != nil {
		log.Debug(fmt.Sprintf("%v", err))
		validationError = append(validationError, err)
		return validationError
	}
	scm := dp.(*openapi3.Schema)
	opts := []openapi3.SchemaValidationOption{openapi3.MultiErrors()}
	depError := VisitJSON(token, scm, object, SchemaSettings{MultiError: true})
	validationError = append(validationError, depError...)

	err = scm.VisitJSON(object, opts...)
	if err != nil {
		e := err.(openapi3.MultiError)
		validationError = append(validationError, e...)
	}
	return validationError
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