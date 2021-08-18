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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/devtron-labs/deprecation-checker/pkg/log"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/tidwall/sjson"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/http"
	"sort"
	"strings"
)

const (
	urlTemplate       = `https://raw.githubusercontent.com/kubernetes/kubernetes/release-%s/api/openapi-spec/swagger.json`
	intOrStringPath   = "components.schemas.io\\.k8s\\.apimachinery\\.pkg\\.util\\.intstr\\.IntOrString"
	intOrStringType   = `{"oneOf":[{"type": "string"},{"type": "integer"}]}`
	intOrStringFormat = "definitions.io\\.k8s\\.apimachinery\\.pkg\\.util\\.intstr\\.IntOrString.format"
	alphaVersion      = 1
	betaVersion       = 2
	gaVersion         = 3
)

type KubeChecker interface {
	LoadFromUrl(releaseVersion string, force bool) error
	LoadFromPath(releaseVersion string, filePath string, force bool) error
	ValidateJson(spec string, releaseVersion string) (ValidationResult, error)
	ValidateYaml(spec string, releaseVersion string) (ValidationResult, error)
	ValidateObject(spec map[string]interface{}, releaseVersion string) (ValidationResult, error)
	GetKinds(releaseVersion string) ([]schema.GroupVersionKind, error)
}

type kubeCheckerImpl struct {
	versionMap map[string]*kubeSpec
}

func NewKubeCheckerImpl() *kubeCheckerImpl {
	return &kubeCheckerImpl{versionMap: map[string]*kubeSpec{}}
}

func (k *kubeCheckerImpl) hasReleaseVersion(releaseVersion string) bool {
	_, ok := k.versionMap[releaseVersion]
	return ok
}

func (k *kubeCheckerImpl) LoadFromPath(releaseVersion string, filePath string, force bool) error {
	if _, ok := k.versionMap[releaseVersion]; ok && !force {
		return nil
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return err
	}
	return k.load(data, releaseVersion)
}

func (k *kubeCheckerImpl) LoadFromUrl(releaseVersion string, force bool) error {
	if _, ok := k.versionMap[releaseVersion]; ok && !force {
		return nil
	}
	data, err := downloadFile(releaseVersion)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return err
	}
	return k.load(data, releaseVersion)
}

func (k *kubeCheckerImpl) load(data []byte, releaseVersion string) error {
	openapi, err := loadOpenApi2(data)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return err
	}
	k.versionMap[releaseVersion] = newKubeSpec(openapi)
	return nil
}

func downloadFile(releaseVersion string) ([]byte, error) {
	url := fmt.Sprintf(urlTemplate, releaseVersion)
	resp, err := http.Get(url)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return []byte{}, err
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, err = io.Copy(&out, resp.Body)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return []byte{}, err
	}
	return out.Bytes(), nil
}

func loadOpenApi2(data []byte) (*openapi3.T, error) {
	var err error
	stringData := string(data)
	stringData, err = sjson.Delete(stringData, intOrStringFormat)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	ctx := context.Background()
	api := openapi2.T{}
	err = (&api).UnmarshalJSON([]byte(stringData))
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	doc, err := openapi2conv.ToV3(&api)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	err = doc.Validate(ctx)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	doc3, err := doc.MarshalJSON()
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	stringData = string(doc3)
	stringData, err = sjson.SetRaw(stringData, intOrStringPath, intOrStringType)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	loader := &openapi3.Loader{Context: ctx}
	doc, err = loader.LoadFromData([]byte(stringData))
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}

	err = doc.Validate(ctx)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return nil, err
	}
	for _, v := range doc.Components.Schemas {
		v.Value.AdditionalPropertiesAllowed = openapi3.BoolPtr(false)
	}
	return doc, nil
}

func (k *kubeCheckerImpl) ValidateYaml(spec string, releaseVersion string) (ValidationResult, error) {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return ValidationResult{}, err
	}
	return k.versionMap[releaseVersion].ValidateYaml(spec)
}

func (k *kubeCheckerImpl) ValidateJson(spec string, releaseVersion string) (ValidationResult, error) {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return ValidationResult{}, err
	}
	return k.versionMap[releaseVersion].ValidateJson(spec)
}

func (k *kubeCheckerImpl) ValidateObject(spec map[string]interface{}, releaseVersion string) (ValidationResult, error) {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return ValidationResult{}, err
	}
	return k.versionMap[releaseVersion].ValidateObject(spec)
}

func (k *kubeCheckerImpl) GetKinds(releaseVersion string) ([]schema.GroupVersionKind, error) {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return make([]schema.GroupVersionKind, 0), err
	}
	return k.versionMap[releaseVersion].fetchLatestKinds(), nil
}

type kubeSpec struct {
	*openapi3.T
	kinds                   []schema.GroupVersionKind
	kindInfoMap             map[string][]*KindInfo
}

func newKubeSpec(openapi *openapi3.T) *kubeSpec {
	ks := &kubeSpec{T: openapi}
	ks.kindInfoMap = ks.buildKindInfoMap()
	ks.kinds = ks.buildResources()
	return ks
}

func (ks *kubeSpec) buildGVKRestPathMap() map[string]string {
	pathMap := map[string]string{}
	for path, value := range ks.T.Paths {
		var method *openapi3.Operation
		if value.Post != nil {
			method = value.Post
		} else if value.Put != nil {
			method = value.Put
		}
		if method != nil {
			if gvk, ok := method.Extensions["x-kubernetes-group-version-kind"]; ok {
				gvks, err := getKeyForGVK(gvk.(json.RawMessage))
				if err != nil {
					continue
				}
				pathMap[gvks] = path
			}
		}
	}
	return pathMap
}

func (ks *kubeSpec) buildKindInfoMap() map[string][]*KindInfo {
	kindMap := map[string][]*KindInfo{}
	restPath := ks.buildGVKRestPathMap()
	for component, value := range ks.T.Components.Schemas {
		if gvk, ok := value.Value.Extensions["x-kubernetes-group-version-kind"]; ok {
			gvks, err := parseGVK(gvk.(json.RawMessage))
			if err != nil {
				continue
			}
			kind := strings.ToLower(gvks["kind"])
			if _, ok := kindMap[kind]; !ok {
				kindMap[kind] = make([]*KindInfo, 0)
			}
			ki := KindInfo{
				Version:      gvks["version"],
				Group:        gvks["group"],
				RestPath:     "",
				ComponentKey: component,
				IsGA:         getVersionType(gvks["version"]) == gaVersion,
			}
			gvkKey, err := getKeyForGVK(gvk.(json.RawMessage))
			if err != nil {
				log.Error(err)
				continue
			}
			if p, ok := restPath[gvkKey]; ok {
				ki.RestPath = p
			}
			kindMap[kind] = append(kindMap[kind], &ki)
		}
	}
	for kind, gvs := range kindMap {
		sort.Slice(gvs, func(i, j int) bool {
			return compareVersion(gvs[i].Version, gvs[j].Version)
		})
		kindMap[kind] = gvs
	}
	return kindMap
}

func (ks *kubeSpec) buildResources() []schema.GroupVersionKind {
	gvkMap := make(map[string]bool, 0)
	var gvka []schema.GroupVersionKind
	for _, value := range ks.T.Paths {
		if value.Get == nil {
			continue
		}
		if gvks, ok := value.Get.Extensions["x-kubernetes-group-version-kind"]; ok {
			gvkm, err := parseGVK(gvks.(json.RawMessage))
			if err != nil {
				continue
			}
			gvk := schema.GroupVersionKind{
				Group:   gvkm["group"],
				Version: gvkm["version"],
				Kind:    gvkm["kind"],
			}
			if _, ok := gvkMap[gvk.String()]; ok {
				continue
			}
			gvkMap[gvk.String()] = true
			gvka = append(gvka, gvk)
		}
	}
	return gvka
}

func (ks *kubeSpec) fetchLatestKinds() []schema.GroupVersionKind {
	gvkMap := make(map[string]bool, 0)
	var gvka []schema.GroupVersionKind
	for kind, info := range ks.kindInfoMap {
		last := info[len(info) - 1]
		gvk := schema.GroupVersionKind{
			Group:   last.Group,
			Version: last.Version,
			Kind:    kind,
		}
		if _, ok := gvkMap[gvk.String()]; ok {
			continue
		}
		gvkMap[gvk.String()] = true
		gvka = append(gvka, gvk)
	}
	return gvka
}