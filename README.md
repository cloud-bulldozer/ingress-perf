# ingress-perf - OCP Ingress Performance

[![Release Go project](https://github.com/cloud-bulldozer/ingress-perf/actions/workflows/release.yml/badge.svg)](https://github.com/cloud-bulldozer/ingress-perf/actions/workflows/release.yml)

OCP Ingress performance ultimate tool!

![diagram](doc-assets/diagram.png)

## Reference

Ingress-perf configuration is defined in a YAML file, holding an array of the following structure. [Examples directory](./examples)

| Field Name       | Type             | Description                                                                                 | Default Value | Tools |
|------------------|------------------|---------------------------------------------------------------------------------------------|---------------|------------------|
| `termination`    | `string`         | Benchmark termination. Allowed values are `http`, `edge`, `reencrypt` and `reencrypt`.      | N/A           | `wrk`,`hloader` |
| `connections`    | `int`            | Number of connections per client.                                                           | `0`           | `wrk`,`hloader` |
| `samples`        | `int`            | Number of samples per scenario.                                                             | `0`           | `wrk`,`hloader` |
| `duration`       | `time.Duration`  | Duration of each sample.                                                                    | `""`          | `wrk`,`hloader` |
| `path`           | `string`         | Scenario endpoint path, for example: `/1024.html`, `/2048.html`.                            | `""`          | `wrk`,`hloader` |
| `concurrency`    | `int32`          | Number of clients that will concurrently run the benchmark scenario.                        | `0`           | `wrk`,`hloader` |
| `tool`           | `string`         | Tool to run the benchmark scenario.                                                         | `""`          | `wrk`,`hloader` |
| `serverReplicas` | `int32`          | Number of server (nginx) replicas backed by the routes.                                     | `0`           | `wrk`,`hloader` |
| `tuningPatch`    | `string`         | Defines a JSON merge tuning patch for the default `IngressController` object.               | `""`          | `wrk`,`hloader` |
| `delay`          | `time.Duration`  | Delay between samples.                                                                      | `0s`          | `wrk`,`hloader` |
| `warmup`         | `bool`           | Enables warmup: indexing will be disabled in this scenario.                                 | `false`       | `wrk`,`hloader` |
| `requestTimeout` | `time.Duration`  | Request timeout                                                                             | `1s`          | `wrk`,`hloader` |
| `procs`          | `int`            | Number of processes to trigger in each of the client pods                                   | `1`           | `wrk`,`hloader` |
| `keepalive`      | `bool`           | Use HTTP keepalived connections                                                             | `true`        | `hloader`       |
| `requestRate`    | `int`            | Number of requests per second                                                               | `0` (unlimited) | `hloader`     |
| `http2`          | `bool`           | Use HTTP2 requests, when possible                                                           | `false`         | `hloader`     |

## Supported tools

- wrk: HTTP benchmarking tool. https://github.com/wg/wrk. amd64 and arm64
- hloader: https://github.com/rsevilla87/hloader

## Running

Running ingress-perf is trivial:

```console
$ ./bin/ingress-perf -h
Benchmark OCP ingress stack

Usage:
   [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  run         Run benchmark
  help        Print the version

Flags:
  -h, --help   help for this command

Use " [command] --help" for more information about a command.
```

Use the `run` subcommand to trigger a new benchmark. For example:

```console
$ ./bin/ingress-perf run --cfg cfg.yaml --es-server=https://elasticsearch-instance.com
time="2023-05-10 13:24:37" level=info msg="Running ingress performance 7eba7c57-d875-4b99-a490-be1752b62782" file="ingress-perf.go:39"
time="2023-05-10 13:24:37" level=info msg="Creating elastic indexer" file="ingress-perf.go:44"
time="2023-05-10 13:24:39" level=info msg="Starting ingress-perf" file="runner.go:36"
time="2023-05-10 13:24:40" level=info msg="Deploying benchmark assets" file="runner.go:112"
time="2023-05-10 13:24:41" level=info msg="Running test 1/9: http" file="runner.go:62"
```

Check out the `run` subcommand help for more info about the allowed flags.

## Compile

Go 1.19 is required

```console
$ make build
```
