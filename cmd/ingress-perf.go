// Copyright 2023 The ingress-perf Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"

	"github.com/cloud-bulldozer/go-commons/indexers"
	"github.com/cloud-bulldozer/go-commons/version"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	_ "github.com/cloud-bulldozer/ingress-perf/pkg/log"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner"
	uid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Short: "Benchmark OCP ingress stack",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "😎 Print the version number of ingress-perf",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Version:", version.Version)
		fmt.Println("Git Commit:", version.GitCommit)
		fmt.Println("Build Date:", version.BuildDate)
		fmt.Println("Go Version:", version.GoVersion)
		fmt.Println("OS/Arch:", version.OsArch)
	},
}

func run() *cobra.Command {
	var cfg, uuid, baseUUID, esServer, esIndex, logLevel, baseIndex string
	var cleanup bool
	var tolerancy int
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
			if baseUUID != "" && (tolerancy > 100 || tolerancy < 1) {
				return fmt.Errorf("tolerancy is an integer between 1 and 100")
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
			return runner.Start(uuid, baseUUID, baseIndex, tolerancy, indexer, cleanup)
		},
	}
	cmd.Flags().StringVarP(&cfg, "cfg", "c", "", "Configuration file")
	cmd.Flags().StringVar(&uuid, "uuid", uid.NewV4().String(), "Benchmark uuid")
	cmd.Flags().StringVar(&baseUUID, "baseline-uuid", "", "Baseline uuid to compare the results with")
	cmd.Flags().StringVar(&baseIndex, "baseline-index", "ingress-performance", "Baseline Elasticsearch index")
	cmd.Flags().IntVar(&tolerancy, "tolerancy", 20, "Comparison tolerancy, must be an integer between 1 and 100")
	cmd.Flags().StringVar(&esServer, "es-server", "", "Elastic Search endpoint")
	cmd.Flags().StringVar(&esIndex, "es-index", "ingress-performance", "Elasticsearch index")
	cmd.Flags().BoolVar(&cleanup, "cleanup", true, "Cleanup benchmark assets")
	cmd.Flags().StringVar(&logLevel, "loglevel", "info", "Log level. Allowed levels are error, info and debug")
	cmd.MarkFlagRequired("cfg")
	return cmd
}

func main() {
	cmd.AddCommand(run(), versionCmd)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
