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

package kubedd

import (
	"bytes"
	"fmt"
	"github.com/devtron-labs/deprecation-checker/pkg"
	kLog "github.com/devtron-labs/deprecation-checker/pkg/log"
	"github.com/prometheus/common/log"
	"os"
)

var yamlSeparator = []byte("\n---\n")

// Validate a Kubernetes YAML file, parsing out individual resources
// and validating them all according to the  relevant schemas
func Validate(input []byte, conf *pkg.Config) ([]pkg.ValidationResult, error) {
	kubeC := pkg.NewKubeCheckerImpl()
	if len(conf.SchemaLocation) > 0 {
		err := kubeC.LoadFromPath(conf.KubernetesVersion, conf.SchemaLocation, false)
		if err != nil {
			kLog.Error(err)
			os.Exit(1)
		}
	} else {
		err := kubeC.LoadFromUrl(conf.KubernetesVersion, false)
		if err != nil {
			kLog.Error(err)
			os.Exit(1)
		}
	}
	splits := bytes.Split(input, yamlSeparator)
	var validationResults []pkg.ValidationResult
	for _, split := range splits {
		validationResult, err := kubeC.ValidateYaml(string(split), conf.KubernetesVersion)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			continue
		}
		validationResults = append(validationResults, validationResult)
	}
	return validationResults, nil
}

func ValidateCluster(cluster *pkg.Cluster, conf *pkg.Config) ([]pkg.ValidationResult, error) {
	kubeC := pkg.NewKubeCheckerImpl()
	if len(conf.SchemaLocation) > 0 {
		err := kubeC.LoadFromPath(conf.KubernetesVersion, conf.SchemaLocation, false)
		if err != nil {
			kLog.Error(err)
			os.Exit(1)
		}
	} else {
		err := kubeC.LoadFromUrl(conf.KubernetesVersion, false)
		if err != nil {
			kLog.Error(err)
			os.Exit(1)
		}
	}
	serverVersion, err := cluster.ServerVersion()
	if err != nil {
		log.Debug("unable to parse server version for cluster %v", err)
		serverVersion = conf.KubernetesVersion
	}
	resources, err := kubeC.GetKinds(serverVersion)
	if err != nil {
		log.Debug("error fetching data for server version, defaulting to target kubernetes version, err %v", err)
		//return make([]pkg.ValidationResult, 0), nil
		resources, err = kubeC.GetKinds(conf.KubernetesVersion)
	}
	objects := cluster.FetchK8sObjects(resources, conf)
	var validationResults []pkg.ValidationResult
	for _, obj := range objects {
		annon := obj.GetAnnotations()
		k8sObj := ""
		if val, ok := annon["kubectl.kubernetes.io/last-applied-configuration"]; ok {
			k8sObj = val
		} else {
			bt, err := obj.MarshalJSON()
			if err != nil {
				continue
			}
			k8sObj = string(bt)
		}
		validationResult, err := kubeC.ValidateJson(k8sObj, conf.KubernetesVersion)
		if err != nil {
			fmt.Printf("err: %v\n", err)
			continue
		}
		validationResults = append(validationResults, validationResult)
	}
	return validationResults, nil
}
