package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/flipslidersand/otel-lens/internal/correlator"
	"github.com/flipslidersand/otel-lens/internal/receiver"
	"github.com/flipslidersand/otel-lens/internal/store"
)

func main() {
	root := &cobra.Command{
		Use:   "otellens",
		Short: "OpenTelemetry signal correlation engine",
	}
	root.AddCommand(serveCmd(), statsCmd(), correlateCmd())
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

func statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show trace+metrics correlation summary for a time window",
		RunE: func(cmd *cobra.Command, args []string) error {
			chAddr, _ := cmd.Flags().GetString("clickhouse")
			fromStr, _ := cmd.Flags().GetString("from")
			toStr, _ := cmd.Flags().GetString("to")
			window, _ := cmd.Flags().GetDuration("window")

			var from, to time.Time
			if fromStr != "" {
				var err error
				from, err = time.Parse(time.RFC3339, fromStr)
				if err != nil {
					return fmt.Errorf("--from: %w", err)
				}
			} else {
				from = time.Now().UTC().Add(-window)
			}
			if toStr != "" {
				var err error
				to, err = time.Parse(time.RFC3339, toStr)
				if err != nil {
					return fmt.Errorf("--to: %w", err)
				}
			} else {
				to = time.Now().UTC()
			}

			st, err := store.New(chAddr)
			if err != nil {
				return fmt.Errorf("clickhouse connect: %w", err)
			}
			defer st.Close() //nolint:errcheck

			ctx := context.Background()
			win, err := correlator.Join(ctx, st, from, to)
			if err != nil {
				return err
			}

			summaries := win.Summarize()
			if len(summaries) == 0 {
				fmt.Printf("no signals in [%s, %s)\n", from.Format(time.RFC3339), to.Format(time.RFC3339))
				return nil
			}

			fmt.Printf("Window: %s → %s\n", from.Format(time.RFC3339), to.Format(time.RFC3339))
			fmt.Printf("Spans: %d  Metrics: %d\n\n", len(win.Spans), len(win.Metrics))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SERVICE\tSPANS\tERRORS\tAVG_MS")
			for _, s := range summaries {
				fmt.Fprintf(w, "%s\t%d\t%d\t%.2f\n",
					s.ServiceName, s.SpanCount, s.ErrorCount, s.AvgDurationMs)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().String("clickhouse", "localhost:9000", "ClickHouse address")
	cmd.Flags().String("from", "", "start time (RFC3339, default: now-window)")
	cmd.Flags().String("to", "", "end time (RFC3339, default: now)")
	cmd.Flags().Duration("window", 5*time.Minute, "look-back window when --from is omitted")
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
