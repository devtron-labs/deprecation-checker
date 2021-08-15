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

package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/devtron-labs/deprecation-checker/kubedd"
	"github.com/devtron-labs/deprecation-checker/pkg"
	log2 "github.com/devtron-labs/deprecation-checker/pkg/log"
	"github.com/prometheus/common/log"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version             = "dev"
	commit              = "none"
	date                = "unknown"
	directories         = []string{}
	ignoredPathPatterns = []string{}

	// forceColor tells kubedd to use colored output even if
	// stdout is not a TTY
	forceColor bool

	config = pkg.NewDefaultConfig()
)

// RootCmd represents the the command to run when kubedd is run
var RootCmd = &cobra.Command{
	Short:   "ValidateJson a Kubernetes YAML file against the relevant apiVersion and kind",
	Long:    `ValidateJson a Kubernetes YAML file against the relevant apiVersion and kind, in case the apiVersion for the kind is deprecated or removed then it validates against the latest available apiVersion`,
	Version: fmt.Sprintf("Version: %s\nCommit: %s\nDate: %s\n", version, commit, date),
	Run: func(cmd *cobra.Command, args []string) {
		if config.IgnoreMissingSchemas && !config.Quiet {
			log2.Warn("Set to ignore missing schemas")
		}

		// This is not particularly secure but we highlight that with the name of
		// the config item. It would be good to also support a configurable set of
		// trusted certificate authorities as in the `--certificate-authority`
		// kubectl option.
		if config.InsecureSkipTLSVerify {
			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}

		success := true
		outputManager := pkg.GetOutputManager(config.OutputFormat)

		// Assert that colors will definitely be used if requested
		if forceColor {
			color.NoColor = false
		}

		if len(args) < 1 && len(directories) < 1 {
			log.Error(errors.New("at least one file or one directory should be passed as argument"))
			os.Exit(1)
		}
		files, err := aggregateFiles(args)
		if err != nil {
			log.Error(err)
			success = false
		}

		var aggResults []pkg.ValidationResult
		for _, fileName := range files {
			filePath, _ := filepath.Abs(fileName)
			fileContents, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Error(fmt.Errorf("Could not open file %v", fileName))
				earlyExit()
				success = false
				continue
			}
			config.FileName = fileName
			results, err := kubedd.Validate(fileContents, config)
			if err != nil {
				log.Error(err)
				earlyExit()
				success = false
				continue
			}

			fmt.Println("")
			fmt.Println("###########################################")
			fmt.Printf("Results for file %s\n", fileName)
			fmt.Println("###########################################")
			var deleted []pkg.ValidationResult
			var deprecated []pkg.ValidationResult
			var newerVersion []pkg.ValidationResult
			var success []pkg.ValidationResult

			for _, result := range results {
				if result.Deleted {
					deleted = append(deleted, result)
				} else if result.Deprecated {
					deprecated = append(deprecated, result)
				} else if len(result.LatestAPIVersion) > 0 {
					newerVersion = append(newerVersion, result)
				} else {
					success = append(success, result)
				}
			}
			if len(deleted) > 0 {
				//fmt.Println("###########################################")
				fmt.Println("Removed API Version's")
				//fmt.Println("###########################################")
				fmt.Println("-------------------------------------------")
			}
			printResult(fileName, deleted, outputManager)
			if len(deprecated) > 0 {
				//fmt.Println("###########################################")
				fmt.Println("Deprecated API Version's")
				//fmt.Println("###########################################")
				fmt.Println("-------------------------------------------")
			}
			printResult(fileName, deprecated, outputManager)
			if len(newerVersion) > 0 {
				//fmt.Println("###########################################")
				fmt.Println("Newer API Version's")
				//fmt.Println("###########################################")
				fmt.Println("-------------------------------------------")
			}
			printResult(fileName, newerVersion, outputManager)
			if len(success) > 0 {
				//fmt.Println("###########################################")
				fmt.Println("No Changes in API Version's")
				//fmt.Println("###########################################")
				fmt.Println("-------------------------------------------")
			}
			printResult(fileName, success, outputManager)


			aggResults = append(aggResults, results...)
		}

		// only use result of hasErrors check if `success` is currently truthy
		success = success && !hasErrors(aggResults)

		// flush any final logs which may be sitting in the buffer
		err = outputManager.Flush()
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		if !success {
			os.Exit(1)
		}
	},
}

func printResult(fileName string, results []pkg.ValidationResult, outputManager pkg.OutputManager) {
	for _, r := range results {
		r.FileName = fileName
		err := outputManager.Put(r)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
	}
}

// hasErrors returns truthy if any of the provided results
// contain errors.
func hasErrors(res []pkg.ValidationResult) bool {
	for _, r := range res {
		if len(r.Errors) > 0 {
			return true
		}
	}
	return false
}

// isIgnored returns whether the specified filename should be ignored.
func isIgnored(path string) (bool, error) {
	for _, p := range ignoredPathPatterns {
		m, err := regexp.MatchString(p, path)
		if err != nil {
			return false, err
		}
		if m {
			return true, nil
		}
	}
	return false, nil
}

func aggregateFiles(args []string) ([]string, error) {
	files := make([]string, len(args))
	copy(files, args)

	var allErrors *multierror.Error
	for _, directory := range directories {
		err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			ignored, err := isIgnored(path)
			if err != nil {
				return err
			}
			if !info.IsDir() && (strings.HasSuffix(info.Name(), ".yaml") || strings.HasSuffix(info.Name(), ".yml")) && !ignored {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			allErrors = multierror.Append(allErrors, err)
		}
	}

	return files, allErrors.ErrorOrNil()
}

func earlyExit() {
	if config.ExitOnError {
		os.Exit(1)
	}
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}

func init() {
	rootCmdName := filepath.Base(os.Args[0])
	if strings.HasPrefix(rootCmdName, "kubectl-") {
		rootCmdName = strings.Replace(rootCmdName, "-", " ", 1)
	}
	RootCmd.Use = fmt.Sprintf("%s <file> [file...]", rootCmdName)
	pkg.AddKubeaddFlags(RootCmd, config)
	RootCmd.Flags().BoolVarP(&forceColor, "force-color", "", false, "Force colored output even if stdout is not a TTY")
	RootCmd.SetVersionTemplate(`{{.Version}}`)
	RootCmd.Flags().StringSliceVarP(&directories, "directories", "d", []string{}, "A comma-separated list of directories to recursively search for YAML documents")
	RootCmd.Flags().StringSliceVarP(&ignoredPathPatterns, "ignored-path-patterns", "i", []string{}, "A comma-separated list of regular expressions specifying paths to ignore")
	RootCmd.Flags().StringSliceVarP(&ignoredPathPatterns, "ignored-filename-patterns", "", []string{}, "An alias for ignored-path-patterns")

	viper.SetEnvPrefix("KUBEADD")
	viper.AutomaticEnv()
	viper.BindPFlag("schema_location", RootCmd.Flags().Lookup("schema-location"))
	viper.BindPFlag("filename", RootCmd.Flags().Lookup("filename"))
}

func main() {
	Execute()
}
