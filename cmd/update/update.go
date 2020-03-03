package update

import (
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/funceasy/funceasy-cli/pkg/util/release"
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"github.com/spf13/cobra"
	"io/ioutil"
	"regexp"
)

var Command = &cobra.Command{
	Use:   "update <version>",
	Short: "update FuncEasy in kubernetes",
	Long: `update command allows user to update FuncEasy Resources 
to a available version`,
	Run: func(cmd *cobra.Command, args []string) {
		t := terminal.NewTerminalPrint()
		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		currentVersion := pkg.GetCurrentVersion()
		if currentVersion == "" {
			t.PrintWarnOneLine("Not Install")
			t.LineEnd()
			return
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
		err = pkg.UpdateFuncEasyResources(fileByte)
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
	},
}

func init() {
	Command.Flags().StringP("file", "f", "", "the yaml file path to update")
}

