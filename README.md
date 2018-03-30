# agentmon: Secret Dyno Metrics Agent Mon

![agent mon](https://i.imgur.com/0qtodUm.png)

## Overview

The `agentmon` application reads language metrics from
a [Prometheus][prometheus] or a [StatsD][statsd] source, then
translates and aggregates the metrics data to a format expected by the
language metrics sink.  It posts the data to the language metrics sink
located at `https://app.metrics.heroku.com`.  A typical use case would
be to add `agentmon` to a buildpack and set an environment variable
`HEROKU_METRICS_URL` to `https://app.metrics.heroku.com/<dyno
id>`. `agentmon` will read the language metrics sink URL from the
environment variable when it starts and use it to post language
metrics.

For more about what agentmon actually does, and its modes of
operation, see [design][design].

## Usage

```bash
usage: agentmon [flags] sink-URL 
  -backlog int
        Size of pending measurement buffer (default 1000)
  -debug
        debug mode is more verbose
  -interval int
        Sink flush interval in seconds (default 20)
  -prom-interval int
        Prometheus poll interval in seconds (default 5)
  -prom-url string
        Prometheus URL
  -statsd-addr string
        UDP address for statsd listener
  -version
        print version string
```

## Developing

### Run Tests

```bash
make test 
``` 

### Install

```bash
make install 
```

### Build

```bash
make build 
```

### Create Release Package 

```bash
make release 
```

## Copyright

(c) 2017, Heroku, Inc. See [LICENSE](./LICENSE) for details.


[prometheus]: https://github.com/prometheus/prometheus
[statsd]: https://github.com/b/statsd_spec
[design]: https://github.com/heroku/agentmon/tree/master/doc/design.md
