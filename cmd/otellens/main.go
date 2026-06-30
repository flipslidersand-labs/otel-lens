package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/flipslidersand/otel-lens/internal/receiver"
	"github.com/flipslidersand/otel-lens/internal/store"
)

func main() {
	root := &cobra.Command{
		Use:   "otellens",
		Short: "OpenTelemetry signal correlation engine",
	}
	root.AddCommand(serveCmd(), correlateCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start OTLP gRPC receiver",
		RunE: func(cmd *cobra.Command, args []string) error {
			chAddr, _ := cmd.Flags().GetString("clickhouse")
			grpcAddr, _ := cmd.Flags().GetString("addr")

			logger, err := zap.NewProduction()
			if err != nil {
				return err
			}
			defer logger.Sync() //nolint:errcheck

			st, err := store.New(chAddr)
			if err != nil {
				return fmt.Errorf("clickhouse connect: %w", err)
			}
			defer st.Close() //nolint:errcheck

			ctx := context.Background()
			if err := st.Ping(ctx); err != nil {
				return fmt.Errorf("clickhouse ping: %w", err)
			}
			if err := st.CreateSchema(ctx); err != nil {
				return fmt.Errorf("create schema: %w", err)
			}

			return receiver.Serve(grpcAddr, st, logger)
		},
	}
	cmd.Flags().String("clickhouse", "localhost:9000", "ClickHouse address")
	cmd.Flags().String("addr", ":4317", "OTLP gRPC listen address")
	return cmd
}

func correlateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "correlate",
		Short: "Find correlation candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("correlate — not yet implemented (Phase 5)")
			return nil
		},
	}
}
