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
	"os"
	"strings"
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
	for _, split := range splits {
		err := kubeC.ValidateYaml(string(split), conf.KubernetesVersion)
		if err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}
	return []pkg.ValidationResult{}, nil
}

func singleLineErrorFormat(es []error) string {
	messages := make([]string, len(es))
	for i, e := range es {
		messages[i] = e.Error()
	}
	return strings.Join(messages, "\n")
}
