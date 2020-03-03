package restart

import (
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/spf13/cobra"
)

var Command = &cobra.Command{
	Use:   "restart",
	Short: "Restart FuncEasy Pods and Services in Kubernetes",
	Long: `Restart FuncEasy Pods and Services in Kubernetes`,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.Restart()
	},
}

func init() {
}
