/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package install

import (
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/funceasy/funceasy-cli/pkg/util/release"
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"regexp"
)

// generateCmd represents the generate command
var Command = &cobra.Command{
	Use:   "install <version> FLAG",
	Short: "install FuncEasy in kubernetes",
	Long: `install command allows user to install FuncEasy CRD 
and other module working for FuncEasy`,
	Run: func(cmd *cobra.Command, args []string) {
		t := terminal.NewTerminalPrint()
		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		local, err := cmd.Flags().GetString("local")
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		sc, err := cmd.Flags().GetString("storage-class")
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		if filePath == "" && len(args) != 1 {
			t.PrintErrorOneLineWithExit("Need exactly one argument - version")
		}
		var fileByte []byte
		if filePath != "" && len(args) == 0 {
			fileByte, err = ioutil.ReadFile(filePath)
			if err != nil {
				t.PrintErrorOneLineWithExit("Read Yaml File Error: ", err)
			}
		} else if filePath == "" && len(args) == 1 {
			version := args[0]
			downloadUrl := ""
			r, _ := regexp.Compile("^(.+).yaml$")
			if version == "latest" {
				release := release.GetLatestRelease()
				for _, asset := range release.Assets {
					if r.MatchString(asset.Name) {
						downloadUrl = asset.Download
					}
				}
			} else {
				list := release.GetRelease()
				for _, item := range list {
					if item.Name == version {
						for _, asset := range item.Assets {
							if r.MatchString(asset.Name) {
								downloadUrl = asset.Download
							}
						}
					}
				}
			}
			if downloadUrl == "" {
				t.PrintErrorOneLineWithExit("Version Not Found: ", version)
			}
			fileByte = release.Download(downloadUrl)
		} else {
			t.PrintErrorOneLineWithExit("Use arg <version> or flags [--file] ")
		}
		if local != "" && sc == "" {
			err = pkg.DeployFuncEasyResources(fileByte, "Local", local)
		} else if local == "" && sc != "" {
			err = pkg.DeployFuncEasyResources(fileByte, "StorageClass", sc)
		} else {
			t.PrintErrorOneLineWithExit("Only one type: Local or StorageClass")
		}
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
	},
}

func init() {
	var mountPath string
	if home := homedir.HomeDir(); home != "" {
		mountPath = filepath.Join(home, "mnt", "funceasy-data")
	} else {
		mountPath = "/mnt/funceasy-data"
	}
	Command.Flags().StringP("file", "f", "", "the yaml file path to install")
	Command.Flags().StringP("local", "l", mountPath, "the local mount path")
	Command.Flags().StringP("storage-class", "s", "", "the PVC StorageClass name")
}
