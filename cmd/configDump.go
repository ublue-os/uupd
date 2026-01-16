package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"

	"github.com/ublue-os/uupd/pkg/config"
)

func ConfigDump(cmd *cobra.Command, args []string) error {
	ret, err := json.MarshalIndent(config.Conf, "", "    ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", ret)
	return nil
}
