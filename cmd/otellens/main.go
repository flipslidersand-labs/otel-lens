package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "otellens",
		Short: "OpenTelemetry signal correlation engine",
	}
	root.AddCommand(
		&cobra.Command{Use: "serve", Short: "Start OTLP receiver", RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("serve — not yet implemented")
			return nil
		}},
		&cobra.Command{Use: "correlate", Short: "Find correlation candidates", RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("correlate — not yet implemented")
			return nil
		}},
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
