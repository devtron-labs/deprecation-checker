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
	"github.com/getkin/kin-openapi/openapi3"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
)


func (ks *kubernetesSpec) ValidateYaml(spec string) error {
	var err error
	jsonSpec, err := yaml.YAMLToJSON([]byte(spec))
	if err != nil {
		return err
	}
	return ks.ValidateJson(string(jsonSpec))
}

func (ks *kubernetesSpec) ValidateJson(spec string) error {
	var err error
	object := make(map[string]interface{})
	err = json.Unmarshal([]byte(spec), &object)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return ks.ValidateObject(object)
}

func (ks *kubernetesSpec) ValidateObject(object map[string]interface{}) error {
	var err error
	originalApiVersion := object["apiVersion"].(string)
	apiVersion := object["apiVersion"].(string)
	if len(apiVersion) == 0 || apiVersion == "v1" {
		apiVersion = "core/v1"
	}
	if len(originalApiVersion) == 0 {
		originalApiVersion = "v1"
	}
	kind := object["kind"].(string)
	token := fmt.Sprintf("io.k8s.api.%s.%s", strings.ReplaceAll(apiVersion, "/", "."), kind)
	path := fmt.Sprintf("/apis/%s/%s", apiVersion, kind)
	regexPath := fmt.Sprintf("/api(s)?/%s/.*/%s($|s|es)$", originalApiVersion, strings.ToLower(kind))
	pathItem := ks.Paths.Find(strings.ToLower(path))
	re := regexp.MustCompile(regexPath)
	for key, value := range ks.Paths {
		if re.MatchString(key) && strings.Index(key, "watch") < 0 {
			pathItem = value
		}
	}
	if pathItem == nil {
		if _, ok := ks.kindMap[strings.ToLower(kind)]; !ok {
			errorMsg := fmt.Sprintf("unsupported api - apiVersion: %s, kind: %s", apiVersion, kind)
			fmt.Println(errorMsg)
			return fmt.Errorf(errorMsg)
		}
		token = ks.kindMap[strings.ToLower(kind)]
	}
	dp, err := ks.Components.Schemas.JSONLookup(token)
	if err != nil {
		fmt.Println(err)
		return err
	}
	scm := dp.(*openapi3.Schema)
	opts := []openapi3.SchemaValidationOption{openapi3.MultiErrors()}
	depError := VisitJSON(path, scm, object, SchemaSettings{MultiError: true})

	err = scm.VisitJSON(object, opts...)
	if err != nil {
		e := err.(openapi3.MultiError)
		e = append(e, depError...)
		fmt.Println(e)
		return e
	}
	return nil
}