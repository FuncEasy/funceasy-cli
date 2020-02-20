package generate

import (
	"encoding/pem"
	"github.com/funceasy/funceasy-cli/pkg"
	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
	"os"
	"path"
)

var tokenCmd = &cobra.Command{
	Use:                        "token <token_name> <token_out_path> FLAG",
	Short:                      "generate a signed token for client",
	Long:                       `generate a signed token with RS256 private key for 
verification in data source service and gateway service using public key`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		outpath := args[1]
		bits, err := cmd.Flags().GetInt32("bits")
		if err != nil {
			logrus.Fatal(err)
		}
		privateKeyPemBlock, publicKeyPemBlock , err := pkg.GenerateRSAKeys(int(bits))
		if err != nil {
			logrus.Fatal(err)
		}
		file, err := os.Create(path.Join(outpath, name + ".rsa_private.key"))
		defer file.Close()
		if err != nil {
			logrus.Fatal(err)
		}

		err = pem.Encode(file, privateKeyPemBlock)
		if err != nil {
			logrus.Fatal(err)
		}

		file, err = os.Create(path.Join(outpath, name + ".public.key"))
		if err != nil {
			logrus.Fatal(err)
		}

		err = pem.Encode(file, publicKeyPemBlock)
		if err != nil {
			logrus.Fatal(err)
		}

		privateByte := pem.EncodeToMemory(privateKeyPemBlock)
		tokenStr, err := pkg.SignedToken(name, privateByte)
		if err != nil {
			logrus.Fatal(err)
		}

		file, err = os.Create(path.Join(outpath, name + ".token"))
		if err != nil {
			logrus.Fatal(err)
		}

		_, err = file.WriteString(tokenStr)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	tokenCmd.Flags().Int32P("bits", "b", 1024, "the bits of private key")
}
