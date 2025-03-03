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

	"github.com/cloud-bulldozer/go-commons/version"
	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
	_ "github.com/cloud-bulldozer/ingress-perf/pkg/log"
	"github.com/cloud-bulldozer/ingress-perf/pkg/runner"
	uid "github.com/satori/go.uuid"
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
	var cfg, uuid, esServer, esIndex, logLevel, outputDir, igNamespace string
	var cleanup, esInsecureSkipVerify, podMetrics, serviceMesh, gatewayAPI bool
	cmd := &cobra.Command{
		Use:           "run",
		Short:         "Run benchmark",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			lvl, err := log.ParseLevel(logLevel)
			if err != nil {
				return err
			}
			log.SetLevel(lvl)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Infof("Running ingress-perf (%s@%s) with uuid %s", version.Version, version.GitCommit, uuid)
			if err := config.Load(cfg); err != nil {
				return err
			}
			r := runner.New(
				uuid, cleanup,
				runner.WithIndexer(esServer, esIndex, outputDir, podMetrics, esInsecureSkipVerify),
				runner.WithServiceMesh(serviceMesh, igNamespace),
				runner.WithGatewayAPI(gatewayAPI),
			)
			return r.Start()
		},
	}
	cmd.Flags().StringVarP(&cfg, "cfg", "c", "", "Configuration file")
	cmd.Flags().StringVar(&uuid, "uuid", uid.NewV4().String(), "Benchmark uuid")
	cmd.Flags().StringVar(&esServer, "es-server", "", "Elastic Search endpoint")
	cmd.Flags().BoolVar(&esInsecureSkipVerify, "es-insecure-skip-verify", true, "Elastic Search insecure skip verify")
	cmd.Flags().StringVar(&esIndex, "es-index", "ingress-performance", "Elasticsearch index")
	cmd.Flags().StringVar(&outputDir, "output-dir", "output", "Store collected metrics in this directory")
	cmd.Flags().BoolVar(&cleanup, "cleanup", true, "Cleanup benchmark assets")
	cmd.Flags().BoolVar(&podMetrics, "pod-metrics", false, "Index per pod metrics")
	cmd.Flags().StringVar(&logLevel, "loglevel", "info", "Log level. Allowed levels are error, info and debug")
	cmd.Flags().StringVar(&igNamespace, "gw-ns", "istio-system", "Ingress gateway namespace")
	cmd.Flags().BoolVar(&serviceMesh, "service-mesh", false, "Enable service mesh mode")
	cmd.Flags().BoolVar(&gatewayAPI, "gateway-api", false, "Enable gateway api mode")
	cmd.MarkFlagRequired("cfg")
	return cmd
}

func main() {
	cmd.AddCommand(run(), versionCmd)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}
