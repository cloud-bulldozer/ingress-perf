package main

import (
	"time"

	"github.com/cloud-bulldozer/go-commons/indexers"
	"github.com/rsevilla87/ingress-perf/pkg/config"
	_ "github.com/rsevilla87/ingress-perf/pkg/log"
	"github.com/rsevilla87/ingress-perf/pkg/runner"
	uid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Short: "Benchmark OCP ingress stack",
}

func run() *cobra.Command {
	var cfg, uuid, esServer, esIndex, logLevel string
	var cleanup bool
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Run benchmark",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return err
			}
			logrus.SetLevel(lvl)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var indexer *indexers.Indexer
			var err error
			log.Infof("Running ingress performance %s", uuid)
			if err := config.Load(cfg); err != nil {
				return err
			}
			if esServer != "" {
				log.Infof("Creating %s indexer", indexers.ElasticIndexer)
				indexerCfg := indexers.IndexerConfig{
					Type:    indexers.ElasticIndexer,
					Servers: []string{esServer},
					Index:   esIndex,
				}
				indexer, err = indexers.NewIndexer(indexerCfg)
				if err != nil {
					return err
				}
			}
			if err = runner.Start(uuid, indexer); err != nil {
				return err
			}
			if cleanup {
				err = runner.Cleanup(10 * time.Minute)
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&cfg, "cfg", "c", "", "Configuration file")
	cmd.Flags().StringVar(&uuid, "uuid", uid.NewV4().String(), "Benchmark uuid")
	cmd.Flags().StringVar(&esServer, "es-server", "", "Elastic Search endpoint")
	cmd.Flags().StringVar(&esIndex, "es-index", "ingress-performance", "Elastic Search index")
	cmd.Flags().BoolVar(&cleanup, "cleanup", true, "Cleanup benchmark assets")
	cmd.Flags().StringVar(&logLevel, "loglevel", "info", "Log level. Allowed levels are error, info and debug")
	cmd.MarkFlagRequired("cfg")
	return cmd
}

func cleanup() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:          "cleanup",
		Short:        "Cleanup benchmark assets",
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
