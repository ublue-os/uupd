package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ConfigDump(cmd *cobra.Command, args []string) error {
	settings := viper.AllSettings()
	ret, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", ret)
	return nil
}
