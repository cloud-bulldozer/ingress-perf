package main

import (
	"time"

	"github.com/rsevilla87/ingress-perf/pkg/config"
	_ "github.com/rsevilla87/ingress-perf/pkg/log"
	"github.com/rsevilla87/ingress-perf/pkg/runner"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Short: "Benchmark OCP ingress stack",
}

func run() *cobra.Command {
	var cfg string
	cmd := &cobra.Command{
		Use:          "run",
		Short:        "Run benchmark",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Load(cfg); err != nil {
				return err
			}
			err := runner.Start()
			return err
		},
	}
	cmd.Flags().StringVarP(&cfg, "cfg", "c", "", "Configuration file")
	cmd.MarkFlagRequired("cfg")
	return cmd
}

func cleanup() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:          "cleanup",
		Short:        "Cleanup benchmark resources",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runner.Cleanup(timeout)
			return err
		},
	}
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", time.Minute, "Cleanup timeout")
	return cmd
}

func main() {
	cmd.AddCommand(run())
	cmd.AddCommand(cleanup())
	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
