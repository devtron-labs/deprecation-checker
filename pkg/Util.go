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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"math"
	"regexp"
	"strconv"
	"strings"
)

func buildResources(openpidoc *openapi3.T) []schema.GroupVersionKind {
	gvkMap := make(map[string]bool, 0)
	var gvka []schema.GroupVersionKind
	for _, value := range openpidoc.Components.Schemas {
		if gvks, ok := value.Value.Extensions["x-kubernetes-group-version-kind"]; ok {
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

func getKeyForGV(msg json.RawMessage) (string, error) {
	var arr []map[string]string
	err := json.Unmarshal(msg, &arr)
	if err == nil {
		if len(arr) > 1 {
			return "", fmt.Errorf("multiple x-kubernetes-group-version-kind hence skipping")
		}
		if len(arr) > 0 {
			return fmt.Sprintf(gvFormat, arr[0]["group"], arr[0]["version"]), nil
		}
	}
	var m map[string]string
	err = json.Unmarshal(msg, &m)
	if err == nil {
		return fmt.Sprintf(gvFormat, m["group"], m["version"]), nil
	}
	return "", nil
}

func getKeyForRawGVK(msg json.RawMessage) (string, error) {
	gvk, err := parseGVK(msg)
	if err != nil {
		return "", err
	}
	if g, ok := gvk["group"]; ok && len(g) > 0 {
		return strings.ToLower(fmt.Sprintf(gvkFormat, gvk["group"], gvk["version"], gvk["kind"])), nil
	}
	return strings.ToLower(fmt.Sprintf(gvFormat, gvk["version"], gvk["kind"])), nil
}

func parseGVK(msg json.RawMessage) (map[string]string, error) {
	var arr []map[string]string
	err := json.Unmarshal(msg, &arr)
	if err == nil {
		if len(arr) > 1 {
			//fmt.Printf("len >1 for %v\n", arr)
			return nil, fmt.Errorf("multiple x-kubernetes-group-version-kind hence skipping")
		}
		if len(arr) > 0 {
			return arr[0], nil
		}
	}
	var m map[string]string
	err = json.Unmarshal(msg, &m)
	if err == nil {
		return m, nil
	}
	return nil, fmt.Errorf("parsing error")
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
		//kLog.Debug(fmt.Sprintf("%v", err))
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
	if err != nil {
		log.Error(err)
	}
	lhsMatch := re.FindAllStringSubmatch(lhs, -1)
	rhsMatch := re.FindAllStringSubmatch(rhs, -1)
	lhsMajorVersion, err := getMajorVersion(lhsMatch)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return false, err
	}
	rhsMajorVersion, err := getMajorVersion(rhsMatch)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
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
		//kLog.Debug(fmt.Sprintf("%v", err))
		return false, err
	}
	rhsMinorVersion, err := getMinorVersion(rhsMatch)
	if err != nil {
		//kLog.Debug(fmt.Sprintf("%v", err))
		return false, err
	}

	if lhsMinorVersion <= rhsMinorVersion {
		return true, nil
	}
	if lhsMajorVersion > rhsMajorVersion {
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


func Contains(key string, patterns []string) bool {
	for _, ignoreKey := range patterns {
		if strings.EqualFold(ignoreKey, key) {
			return true
		}
		if RegexMatch(key, ignoreKey) {
			return true
		}
	}
	return false
}

func RegexMatch(s string, pattern string) bool {
	ls := strings.ToLower(s)
	lp := strings.ToLower(pattern)
	if !strings.Contains(lp, "*") {
		return ls == lp
	}
	if strings.Count(lp, "*") == 2 {
		np := strings.ReplaceAll(lp, "*", "")
		return strings.Contains(ls,np)
	}
	if strings.Index(lp, "*") == 0 {
		np := strings.ReplaceAll(lp, "*", "")
		i := strings.Index(ls, np)
		return len(ls) == i + len(np)
		return strings.Contains(ls,np)
	}
	np := strings.ReplaceAll(lp, "*", "")
	return strings.Index(ls, np) == 0
}
