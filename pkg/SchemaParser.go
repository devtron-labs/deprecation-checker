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
	"fmt"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/tidwall/sjson"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	urlTemplate       = `https://raw.githubusercontent.com/kubernetes/kubernetes/release-%s/api/openapi-spec/swagger.json`
	intOrStringPath   = "components.schemas.io\\.k8s\\.apimachinery\\.pkg\\.util\\.intstr\\.IntOrString"
	intOrStringType   = `{"oneOf":[{"type": "string"},{"type": "integer"}]}`
	intOrStringFormat = "definitions.io\\.k8s\\.apimachinery\\.pkg\\.util\\.intstr\\.IntOrString.format"
	alphaVersion = 1
	betaVersion = 2
	gaVersion = 3
)

type kubernetesSpec struct {
	*openapi3.T
	kindMap map[string]string
}

type KubeChecker interface {
	LoadFromUrl(releaseVersion string, force bool) error
	LoadFromPath(releaseVersion string, filePath string, force bool) error
	ValidateJson(spec string, releaseVersion string) error
	ValidateYaml(spec string, releaseVersion string) error
}

type kubeCheckerImpl struct {
	versionMap map[string]*kubernetesSpec
}

func NewKubeCheckerImpl() *kubeCheckerImpl {
	return &kubeCheckerImpl{versionMap: map[string]*kubernetesSpec{}}
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
		return err
	}
	openapi, err := loadOpenApi2(data)
	if err != nil {
		return err
	}
	kindMap := buildKindMap(openapi)
	k.versionMap[releaseVersion] = &kubernetesSpec{T: openapi, kindMap: kindMap}
	return nil
}

func (k *kubeCheckerImpl) LoadFromUrl(releaseVersion string, force bool) error {
	if _, ok := k.versionMap[releaseVersion]; ok && !force {
		return nil
	}
	data, err := downloadFile(releaseVersion)
	if err != nil {
		return err
	}
	openapi, err := loadOpenApi2(data)
	if err != nil {
		return err
	}
	kindMap := buildKindMap(openapi)
	k.versionMap[releaseVersion] = &kubernetesSpec{T: openapi, kindMap: kindMap}
	return nil
}

func downloadFile(releaseVersion string) ([]byte, error) {
	url := fmt.Sprintf(urlTemplate, releaseVersion)
	resp, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	_, err = io.Copy(&out, resp.Body)
	if err != nil {
		return []byte{}, err
	}
	return out.Bytes(), nil
}

func loadOpenApi2(data []byte) (*openapi3.T, error) {
	var err error
	stringData := string(data)
	stringData, err = sjson.Delete(stringData, intOrStringFormat)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	ctx := context.Background()
	api := openapi2.T{}
	err = (&api).UnmarshalJSON([]byte(stringData))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	doc, err := openapi2conv.ToV3(&api)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	err = doc.Validate(ctx)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	doc3, err := doc.MarshalJSON()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	stringData = string(doc3)
	stringData, err = sjson.SetRaw(stringData, intOrStringPath, intOrStringType)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	loader := &openapi3.Loader{Context: ctx}
	doc, err = loader.LoadFromData([]byte(stringData))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	err = doc.Validate(ctx)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	for _, v := range doc.Components.Schemas {
		v.Value.AdditionalPropertiesAllowed = openapi3.BoolPtr(false)
	}
	return doc, nil
}

func buildKindMap(openapidoc *openapi3.T) map[string]string {
	kindMap := map[string]string{}
	for component, _ := range openapidoc.Components.Schemas {
		componentParts := strings.Split(component, ".")
		if len(componentParts) > 0 {
			kind := strings.ToLower(componentParts[len(componentParts)-1])
			if _, ok := kindMap[kind]; !ok {
				kindMap[kind] = component
			}
			version, err := compareVersion(kindMap[kind], component)
			if err != nil {
				continue
			}
			kindMap[kind] = version
		}
	}
	return kindMap
}

func compareVersion(lhs, rhs string) (string, error) {
	if lhs == rhs {
		return lhs, nil
	}
	if !isExtension(lhs) && isExtension(rhs) {
		return lhs, nil
	}
	if isExtension(lhs) && !isExtension(rhs) {
		return rhs, nil
	}

	firstApiVersion := getApiVersion(lhs)
	secondApiVersion := getApiVersion(rhs)

	isSmaller, err := isSmallerVersion(firstApiVersion, secondApiVersion)
	if err != nil {
		return "", err
	}
	if isSmaller {
		return rhs, nil
	} else {
		return lhs, nil
	}
}

func getApiVersion(first string) string {
	components := strings.Split(strings.ToLower(first), ".")[3:]
	apiVersion := components[len(components)-2]
	return apiVersion
}

func isExtension(second string) bool {
	return strings.Index(second, "extensions") >= 0
}

func isSmallerVersion(lhs, rhs string) (bool, error) {
	var re *regexp.Regexp
	var err error
	re, err = regexp.Compile("v(\\d*)([^0-9]*)(\\d*)")
	lhsMatch := re.FindAllStringSubmatch(lhs, -1)
	rhsMatch := re.FindAllStringSubmatch(rhs, -1)
	lhsMajorVersion, err := getMajorVersion(lhsMatch)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	rhsMajorVersion,  err := getMajorVersion(rhsMatch)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	if lhsMajorVersion < rhsMajorVersion {
		return true, nil
	}
	if lhsMajorVersion > rhsMajorVersion {
		return false, nil
	}

	lhsVersionType := getVersionType(lhs)
	rhsVersionType := getVersionType(rhs)
	if lhsVersionType < rhsVersionType {
		return true, nil
	}
	if lhsVersionType > rhsVersionType {
		return false, nil
	}

	lhsMinorVersion, err := getMinorVersion(lhsMatch)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	rhsMinorVersion, err := getMinorVersion(rhsMatch)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	if lhsMinorVersion <= rhsMinorVersion {
		return true, nil
	}
	if  lhsMajorVersion > rhsMajorVersion {
		return false, nil
	}
	return false, nil
}

func getMajorVersion(apiVersion [][]string) (int, error) {
	majorVersion := apiVersion[0][1]
	return strconv.Atoi(majorVersion)
}

func getMinorVersion(apiVersion [][]string) (int, error) {
	minorVersion := apiVersion[0][3]
	if len(minorVersion) == 0 {
		return math.MaxInt32, nil
	}
	return strconv.Atoi(minorVersion)
}

func getVersionType(apiVersion string) int {
	if strings.Index(apiVersion, "alpha") > 0 {
		return alphaVersion
	} else if strings.Index(apiVersion, "beta") > 0 {
		return betaVersion
	}
	return gaVersion
}

func (k *kubeCheckerImpl) ValidateYaml(spec string, releaseVersion string) error {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return err
	}
	return k.versionMap[releaseVersion].ValidateYaml(spec)
}

func (k *kubeCheckerImpl) ValidateJson(spec string, releaseVersion string) error {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return err
	}
	return k.versionMap[releaseVersion].ValidateJson(spec)
}

func (k *kubeCheckerImpl) ValidateObject(spec map[string]interface{}, releaseVersion string) error {
	err := k.LoadFromUrl(releaseVersion, false)
	if err != nil {
		return err
	}
	return k.versionMap[releaseVersion].ValidateObject(spec)
}
