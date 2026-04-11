package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	GitCommit = "dev"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long: `显示版本信息。

示例:
  icpcli version
  icpcli version -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		output, _ := cmd.Flags().GetString("output")

		info := struct {
			Version   string `json:"version"`
			GitCommit string `json:"git_commit"`
			BuildDate string `json:"build_date"`
		}{
			Version:   Version,
			GitCommit: GitCommit,
			BuildDate: BuildDate,
		}

		if output == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		fmt.Printf("icpcli %s (commit: %s, built: %s)\n", info.Version, info.GitCommit, info.BuildDate)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().StringVarP(new(string), "output", "o", "", "输出格式: json")
}
