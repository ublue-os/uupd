package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func ConfigDump(cmd *cobra.Command, args []string) {
	allSettings := viper.AllSettings()

	data, err := yaml.Marshal(allSettings)
	if err != nil {
		slog.Error("Error marshaling config", slog.Any("error", err))
		os.Exit(1)
	}

	fmt.Println(string(data))
}
