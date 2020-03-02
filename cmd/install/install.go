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
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

// generateCmd represents the generate command
var Command = &cobra.Command{
	Use:   "install <version>",
	Short: "install FuncEasy in kubernetes",
	Long: `install command allows user to install FuncEasy CRD 
and other module working for FuncEasy`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			logrus.Fatal(err)
		}
		local, err := cmd.Flags().GetString("local")
		if err != nil {
			logrus.Fatal(err)
		}
		sc, err := cmd.Flags().GetString("storage-class")
		if err != nil {
			logrus.Fatal(err)
		}
		if filePath == "" && len(args) != 1 {
			logrus.Fatal("Need exactly one argument - version")
		}
		var fileByte []byte
		if filePath != "" {
			fileByte, err = ioutil.ReadFile(filePath)
			if err != nil {
				logrus.Fatal("Read Yaml File Error: ", err)
			}
		}
		if local != "" && sc == "" {
			err = pkg.DeployFuncEasyResources(fileByte, "Local", local)
		} else if local == "" && sc != "" {
			err = pkg.DeployFuncEasyResources(fileByte, "StorageClass", sc)
		} else {
			logrus.Fatal("Only one type: Local or StorageClass")
		}
		if err != nil {
			logrus.Fatal(err)
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
