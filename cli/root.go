package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/divolgin/release2chart/pkg/helm"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "release2chart [release]",
		Short:        "Convert a Helm release to a Helm chart",
		Long:         `Convert a Helm release to a Helm chart`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			if len(args) == 0 {
				return errors.New("release name is required")
			}

			namespace := v.GetString("namespace")
			releaseName := args[0]
			revision := 0

			if v.GetString("revision") != "" {
				r, err := strconv.Atoi(v.GetString("revision"))
				if err != nil {
					return errors.Wrap(err, "parse revision")
				}
				revision = r
			} else {
				r, err := helm.FindLatestReleaseVersion(namespace, releaseName)
				if err != nil {
					return errors.Wrap(err, "find latest revision")
				}
				revision = r
			}

			chartFile, valuesFile, err := helm.ConvertReleaseVersion(namespace, releaseName, revision)
			if err != nil {
				return errors.Wrap(err, "convert release")
			}

			command := []string{"helm", "install", releaseName, chartFile}
			if valuesFile != "" {
				command = append(command, "--values", valuesFile)
			}
			command = append(command, "--namespace", namespace)

			fmt.Println("Chart has been saved to", chartFile)
			fmt.Println("To install the chart, run the following command:")
			fmt.Println("")
			fmt.Println(strings.Join(command, " "))
			fmt.Println("")

			return nil
		},
	}

	cobra.OnInitialize(func() {
		// viper.SetEnvPrefix("KOTS")
		// viper.AutomaticEnv()
	})
	helm.AddFlags(cmd.PersistentFlags())

	cmd.Flags().String("revision", "", "release revision to convert")

	viper.BindPFlags(cmd.Flags())
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}
