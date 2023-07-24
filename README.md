# ingress-perf - OCP Ingress Performance

OCP Ingress performance ultimate tool!

![diagram](doc-assets/diagram.png)

## Reference

Ingress-perf configuration is defined in a YAML file, holding an array of the following structure. [Examples directory](./config)

| Field Name       | Type             | Description                                                                                              | Default Value |
|------------------|------------------|----------------------------------------------------------------------------------------------------------|---------------|
| `termination`    | `string`         | Defines the type of benchmark termination. Allowed values are `http`, `edge`, `reencrypt` and `reencrypt`. | N/A           |
| `connections`    | `int`            | Defines the number of connections per client.                                                            | `0`           |
| `samples`        | `int`            | Defines the number of samples per scenario.                                                              | `0`           |
| `duration`       | `time.Duration`  | Defines the duration of each sample.                                                                     | `""`          |
| `path`           | `string`         | Defines the scenario endpoint path, for example: `/1024.html`, `/2048.html`.                              | `""`          |
| `concurrency`    | `int32`          | Defines the number of clients that will concurrently run the benchmark scenario.                        | `0`           |
| `tool`           | `string`         | Defines the tool to run the benchmark scenario.                                                         | `""`          |
| `serverReplicas` | `int32`          | Defines the number of server (nginx) replicas backed by the routes.                                      | `0`           |
| `tuningPatch`    | `string`         | Defines a JSON merge tuning patch for the default `IngressController` object.                            | `""`          |
| `delay`          | `time.Duration`  | Defines a delay between samples.                                                                         | `0s`          |
| `warmup`         | `bool`           | Enables warmup: indexing will be disabled in this scenario.                                              | `false`       |
| `requestTimeout` | `time.Duration`  | Request timeout                                                                                          | `1s`       |
| `procs         ` | `int`            | Number of processes to trigger in each of the client pods                                                | `1`       |

## Supported tools

- wrk: HTTP benchmarking tool. https://github.com/wg/wrk

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
