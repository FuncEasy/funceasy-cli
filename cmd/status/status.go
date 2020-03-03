package status

import (
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var Command = &cobra.Command{
	Use:   "status",
	Short: "Show FuncEasy Pods Status in Kubernetes",
	Long: `Show FuncEasy Pods Status in Kubernetes`,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.GetResourceStatus()
	},
}

func init() {
}
