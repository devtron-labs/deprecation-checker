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
	"encoding/json"
	"fmt"
	kLog "github.com/devtron-labs/deprecation-checker/pkg/log"
	"github.com/getkin/kin-openapi/openapi3"
	"strings"

	//"github.com/olekukonko/tablewriter"
	"github.com/tomlazar/table"
	"log"
	"os"
)

// OutputManager controls how results of the `kubedd` evaluation will be recorded
// and reported to the end user.
// This interface is kept private to ensure all implementations are closed within
// this package.
type OutputManager interface {
	PutBulk(r []ValidationResult) error
	Put(r ValidationResult) error
	Flush() error
}

const (
	outputSTD  = "stdout"
	outputJSON = "json"
	outputTAP  = "tap"
)

func validOutputs() []string {
	return []string{
		outputSTD,
		outputJSON,
		outputTAP,
	}
}

func GetOutputManager(outFmt string) OutputManager {
	switch outFmt {
	case outputSTD:
		return newSTDOutputManager()
	case outputJSON:
		return newDefaultJSONOutputManager()
	case outputTAP:
		return newDefaultTAPOutputManager()
	default:
		return newSTDOutputManager()
	}
}

// STDOutputManager reports `kubedd` results to stdout.
type STDOutputManager struct {
}

// newSTDOutputManager instantiates a new instance of STDOutputManager.
func newSTDOutputManager() *STDOutputManager {
	return &STDOutputManager{}
}

func (s *STDOutputManager) PutBulk(results []ValidationResult) error {
	if len(results) == 0 {
		return nil
	}
	var deleted []ValidationResult
	var deprecated []ValidationResult
	var newerVersion []ValidationResult
	var unchanged []ValidationResult

	for _, result := range results {
		if len(result.Kind) == 0 {
			continue
		}else if result.Deleted {
			deleted = append(deleted, result)
		} else if result.Deprecated && len(result.LatestAPIVersion) > 0 {
			deprecated = append(deprecated, result)
		} else if result.Deprecated {
			//Skip
		} else if len(result.LatestAPIVersion) > 0 {
			newerVersion = append(newerVersion, result)
		} else {
			unchanged = append(unchanged, result)
		}
	}
	if len(deleted) > 0 {
		kLog.Error(fmt.Errorf("Removed API Version's"))
		s.SummaryTableBodyOutput(deleted)
		fmt.Println("")
		s.DeprecationTableBodyOutput(results, false)
		s.ValidationErrorTableBodyOutput(results, true)
	}
	if len(deprecated) > 0 {
		kLog.Warn("Deprecated API Version's")
		s.SummaryTableBodyOutput(deprecated)
		fmt.Println("")
		//s.DeprecationTableBodyOutput(results, true)
		s.ValidationErrorTableBodyOutput(results, true)
		s.DeprecationTableBodyOutput(results, false)
		s.ValidationErrorTableBodyOutput(results, false)
	}
	if len(newerVersion) > 0 {
		kLog.Warn("Newer Versions available")
		s.SummaryTableBodyOutput(newerVersion)
		fmt.Println("")
		s.DeprecationTableBodyOutput(results, true)
		s.ValidationErrorTableBodyOutput(results, true)
		s.DeprecationTableBodyOutput(results, false)
		s.ValidationErrorTableBodyOutput(results, false)
	}
	if len(unchanged) > 0 {
		kLog.Warn("Unchanged API Version's")
		//s.SummaryTableBodyOutput(unchanged)
		s.DeprecationTableBodyOutput(results, true)
		s.ValidationErrorTableBodyOutput(results, true)
	}

	return nil
}

func (s *STDOutputManager) SummaryTableBodyOutput(results []ValidationResult) {
	t := table.Table{Headers: []string{"Namespace", "Name", "Kind", "API Version", "Replace With API Version"}}
	c := table.DefaultConfig()
	c.ShowIndex = false
	for _, result := range results {
		t.Rows = append(t.Rows, []string{result.ResourceNamespace, result.ResourceName, result.Kind, result.APIVersion, result.LatestAPIVersion})
	}
	t.WriteTable(os.Stdout, c)
}

func (s *STDOutputManager) DeprecationTableBodyOutput(results []ValidationResult, currentVersion bool) {
	hasData := false
	for _, result := range results {
		errors := result.DeprecationForLatest
		if currentVersion {
			errors = result.DeprecationForOriginal
		}
		if len(errors) > 0 {
			for _, e := range errors {
				if len(e.JSONPointer()) > 0 {
					hasData = true
					break
				}
			}
		}
	}
	if !hasData {
		return
	}
	t := table.Table{Headers: []string{"Namespace", "Name", "Kind", "API Version", "Field", "Reason"}}
	c := table.DefaultConfig()
	c.ShowIndex = false
	for _, result := range results {
		errors := result.DeprecationForLatest
		apiVersion := result.LatestAPIVersion
		if currentVersion {
			apiVersion = result.APIVersion
			errors = result.DeprecationForOriginal
		}
		for _, e := range errors {
			t.Rows = append(t.Rows, []string{result.ResourceNamespace, result.ResourceName, result.Kind, apiVersion, strings.Join(e.JSONPointer(), "/"), e.Reason})
		}
	}
	t.WriteTable(os.Stdout, c)
}

func (s *STDOutputManager) ValidationErrorTableBodyOutput(results []ValidationResult, currentVersion bool) {
	hasData := false
	for _, result := range results {
		errors := result.ErrorsForLatest
		if currentVersion {
			errors = result.ErrorsForOriginal
		}
		if len(errors) > 0 {
			for _, e := range errors {
				if len(e.JSONPointer()) > 0 {
					hasData = true
					break
				}
			}
		}
	}
	if !hasData {
		return
	}
	t := table.Table{Headers: []string{"Namespace", "Name", "Kind", "API Version", "Field", "Reason"}}
	c := table.DefaultConfig()
	c.ShowIndex = false
	for _, result := range results {
		errors := result.ErrorsForLatest
		apiVersion := result.LatestAPIVersion
		if currentVersion {
			apiVersion = result.APIVersion
			errors = result.ErrorsForOriginal
		}
		for _, e := range errors {
			if len(e.JSONPointer()) > 0 {
				t.Rows = append(t.Rows, []string{result.ResourceNamespace, result.ResourceName, result.Kind, apiVersion, strings.Join(e.JSONPointer(), "/"), e.Reason})
			}
		}
	}
	t.WriteTable(os.Stdout, c)
}

func (s *STDOutputManager) Put(result ValidationResult) error {
	openapi3.SchemaErrorDetailsDisabled = true
	//s.TableOutput(result)
	//if result.Kind == "" {
	//	log2.Success(result.FileName, "contains an empty YAML document")
	//} else if !result.ValidatedAgainstSchema {
	//	log2.Warn(result.FileName, "containing a", result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), "was not validated against a schema")
	//} else if !result.Deleted && len(result.ErrorsForOriginal) == 0 {
	//	log2.Success(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()))
	//}
	//
	//if len(result.LatestAPIVersion) > 0 {
	//	if result.Deleted {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), result.APIVersion, result.LatestAPIVersion)
	//	} else if result.Deprecated {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), result.APIVersion, result.LatestAPIVersion)
	//	} else {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), result.APIVersion, result.LatestAPIVersion)
	//	}
	//}
	//if len(result.DeprecationForOriginal) > 0 {
	//	fmt.Printf("Deprecations for %s %s %s\n", result.APIVersion, result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()))
	//	for _, desc := range result.DeprecationForOriginal {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), strings.Join(desc.JSONPointer(), "/"), desc.Reason)
	//	}
	//}
	//if len(result.ErrorsForOriginal) > 0 {
	//	fmt.Printf("Validation error for %s %s %s\n", result.APIVersion, result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()))
	//	for _, desc := range result.ErrorsForOriginal {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), strings.Join(desc.JSONPointer(), "/"), desc.Reason)
	//	}
	//}
	//if len(result.DeprecationForLatest) > 0 {
	//	fmt.Printf("Deprecations for %s %s %s\n", result.LatestAPIVersion, result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()))
	//	for _, desc := range result.DeprecationForLatest {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), strings.Join(desc.JSONPointer(), "/"), desc.Reason)
	//	}
	//}
	//if len(result.ErrorsForLatest) > 0 {
	//	fmt.Printf("Validation error for %s %s %s\n", result.LatestAPIVersion, result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()))
	//	for _, desc := range result.ErrorsForLatest {
	//		log2.Warn(result.Kind, fmt.Sprintf("(%s)", result.QualifiedName()), strings.Join(desc.JSONPointer(), "/"), desc.Reason)
	//	}
	//}

	return nil
}

func (s *STDOutputManager) Flush() error {
	// no op
	return nil
}

type status string

const (
	statusInvalid = "invalid"
	statusValid   = "valid"
	statusSkipped = "skipped"
)

type dataEvalResult struct {
	Filename string   `json:"filename"`
	Kind     string   `json:"kind"`
	Status   status   `json:"status"`
	Errors   []string `json:"errors"`
}

// jsonOutputManager reports `ccheck` results to `stdout` as a json array..
type jsonOutputManager struct {
	logger *log.Logger

	data []dataEvalResult
}

func newDefaultJSONOutputManager() *jsonOutputManager {
	return newJSONOutputManager(log.New(os.Stdout, "", 0))
}

func newJSONOutputManager(l *log.Logger) *jsonOutputManager {
	return &jsonOutputManager{
		logger: l,
	}
}

func getStatus(r ValidationResult) status {
	if r.Kind == "" {
		return statusSkipped
	}

	if !r.ValidatedAgainstSchema {
		return statusSkipped
	}

	if len(r.Errors) > 0 {
		return statusInvalid
	}

	return statusValid
}

func (j *jsonOutputManager) PutBulk(r []ValidationResult) error {
	return nil
}

func (j *jsonOutputManager) Put(r ValidationResult) error {
	// stringify gojsonschema errors
	// use a pre-allocated slice to ensure the json will have an
	// empty array in the "zero" case
	errs := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		errs = append(errs, e.String())
	}

	j.data = append(j.data, dataEvalResult{
		Filename: r.FileName,
		Kind:     r.Kind,
		Status:   getStatus(r),
		Errors:   errs,
	})

	return nil
}

func (j *jsonOutputManager) Flush() error {
	b, err := json.Marshal(j.data)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	err = json.Indent(&out, b, "", "\t")
	if err != nil {
		return err
	}

	j.logger.Print(out.String())
	return nil
}

// tapOutputManager reports `conftest` results to stdout.
type tapOutputManager struct {
	logger *log.Logger

	data []dataEvalResult
}

// newDefaultTapOutManager instantiates a new instance of tapOutputManager
// using the default logger.
func newDefaultTAPOutputManager() *tapOutputManager {
	return newTAPOutputManager(log.New(os.Stdout, "", 0))
}

// newTapOutputManager constructs an instance of tapOutputManager given a
// logger instance.
func newTAPOutputManager(l *log.Logger) *tapOutputManager {
	return &tapOutputManager{
		logger: l,
	}
}

func (j *tapOutputManager) PutBulk(r []ValidationResult) error {
	return nil
}

func (j *tapOutputManager) Put(r ValidationResult) error {
	errs := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		errs = append(errs, e.String())
	}

	j.data = append(j.data, dataEvalResult{
		Filename: r.FileName,
		Kind:     r.Kind,
		Status:   getStatus(r),
		Errors:   errs,
	})

	return nil
}

func (j *tapOutputManager) Flush() error {
	issues := len(j.data)
	if issues > 0 {
		total := 0
		for _, r := range j.data {
			if len(r.Errors) > 0 {
				total = total + len(r.Errors)
			} else {
				total = total + 1
			}
		}
		j.logger.Print(fmt.Sprintf("1..%d", total))
		count := 0
		for _, r := range j.data {
			count = count + 1
			var kindMarker string
			if r.Kind == "" {
				kindMarker = ""
			} else {
				kindMarker = fmt.Sprintf(" (%s)", r.Kind)
			}
			if r.Status == "valid" {
				j.logger.Print("ok ", count, " - ", r.Filename, kindMarker)
			} else if r.Status == "skipped" {
				j.logger.Print("ok ", count, " - ", r.Filename, kindMarker, " # SKIP")
			} else if r.Status == "invalid" {
				for i, e := range r.Errors {
					j.logger.Print("not ok ", count, " - ", r.Filename, kindMarker, " - ", e)

					// We have to skip adding 1 if it's the last error
					if len(r.Errors) != i+1 {
						count = count + 1
					}
				}
			}
		}
	}
	return nil
}
