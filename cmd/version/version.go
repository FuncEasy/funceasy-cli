package version

import (
	"fmt"
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/funceasy/funceasy-cli/pkg/util/release"
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var Command = &cobra.Command{
	Use:   "version",
	Short: "current version",
	Long: `Show current version. Use inspect to get all available version`,
	Run: func(cmd *cobra.Command, args []string) {
		t := terminal.NewTerminalPrint()
		inspect, err := cmd.Flags().GetBool("inspect")
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		currentVersion := pkg.GetCurrentVersion()
		if len(args) == 0 && !inspect {
			if currentVersion != "" {
				t.PrintInfoOneLine("Current Version: %s", currentVersion)
				t.LineEnd()
			} else {
				t.PrintWarnOneLine("Not Install")
				t.LineEnd()
			}
		} else if inspect {
			releases := release.GetRelease()
			for _, item := range releases {
				if item.Name == currentVersion {
					t.PrintInfoOneLine("%s [%s@%s]", item.Name, item.TagName, item.TargetCommitish)
					t.LineEnd()
				} else {
					fmt.Printf("  %s [%s@%s]\n", item.Name, item.TagName, item.TargetCommitish)
				}
			}
		} else {
			_ = cmd.Help()
		}
	},
}

func init() {
	Command.Flags().BoolP("inspect", "i", false, "inspect the available versions")
}
