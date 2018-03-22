# agentmon Unmasked

Fundamentally, `agentmon` is a silly little server that proxies metric
reporting to a third party, namely, Heroku's Application Metrics endpoint
available over HTTPS. It's capable of receiving metrics via statsd, and
offers a simple Prometheus scraper as well. This document discusses the
way in which all of these things interact.

## Metrics

Those familiar with standard operational metric terminology won't be
surprised to learn that agentmon has a concept of counters and gauges.
These behave very similarly to counters and gauges from
the [statsd][statsd] "protocol". 

That is to say, for a time period defined by the flush interval, the
value of a named counter metric is the sum of all values reported for
that name.

For a gauge, in that same time period, the last value received for a
given name is the value that will be reported.

Outside of the statsd "protocol," agentmon is also capable of handling
monotonically increasing counters as well. Internally, these are called
"derived counters," and they are simply flushed as regular counters. 
Each time a measurement is observed, the value added to the counter is
equal to `{observed value} - {previously observed value}`.

In the [Etsy statsd][etsy-statsd] implementation, counters, and timers
can have an attached sample rate that is typically used to reduce
observations sent to statsd, accepting, possibly, a bit less accuracy
in reported observations. Statsd timers, however, are not handled in
agentmon at the time of this writing. 

### Why no timers?

As it turns out, timers just haven't been necessary for the types of
values we're collecting to date. However, there are also some
challenges, and incompatibilities with the rest of the Heroku metrics
infrastructure.

First, the incompatibilities. The Heroku Metrics infrastructure makes
use of a library called HDR Histogram, which can record the
distribution of a large number of values at a relatively low cost. The
cost is typically less than 1KiB in memory, but the serialization format
is not standardized making it difficult for other agents needing to 
report to the Heroku Application Metrics endpoints difficult.

An alternative strategy would involve reporting multiple, related
metrics, namely, sum of squares, sum, min, max, and count for each
timer metric.  This would allow all of the reporting dynos' values to
be fairly represented, and we could even derive standard deviation,
and your typical average latency. However, average latency, and even
standard deviation of latency isn't all that helpful. What's helpful
are the tail latencies (p95, p99, p99.9), and often times the median
latency. These _aren't_ derivable without knowing what the distribution
looks like, which as we've previously stated, is not representable in
an easily standardizable way.

## Reporting Metrics to Heroku

When started, the agentmon program expects a URL passed as an argument
which speaks the protocol outlined below.

While it'd certainly be possible to report metrics to other services,
and via other means, the only reporter shipping with agentmon is a
simple reporter for Heroku's Application Metrics service. This 
endpoint is POST'ed to over HTTPS, with a few optional headers,
and a simple JSON body.

The `Content-Type` header should be `application/json`. Optionally,
the header `Measurements-Time` can be set to an RFC3339 timestamp,
no more than 4 minutes in the past, representing the time the 
observations are being reported. The 4 minutes is an implementation
detail of the Heroku Metrics Aggregation system--metrics timestamped
older than that will not be aggregated.

Additionally, it's good practice to include the number of measurements
the payload encodes using the `Measurements-Count` header.

The JSON payload is quite simple:

```json
{
   "counters": { 
      COUNTER_NAME_1: COUNTER_VALUE_1,
      COUNTER_NAME_2: COUNTER_VALUE_2,
      ...
      COUNTER_NAME_N: COUNTER_VALUE_N
   },
   "gauges": { 
      GAUGE_NAME_1: GAUGE_VALUE_1,
      GAUGE_NAME_2: GAUGE_VALUE_2,
      ...
      GAUGE_NAME_N: GAUGE_VALUE_N
   }
}
```

The tricky part is that the names must follow statsd conventions,
which is to say alpha numeric tokens separated by `-`, `_`, and `.`.

An HTTP 400 Bad Request is returned for any request related issues,
such as invalid JSON body, stale metrics, or missing `Content-Type`.
A HTTP 200 OK, with no body is returned on success.

## Receiving Metrics via statsd over UDP

When the program is started with `-statsd-addr IPV4:PORT`, the program
creates a UDP listener to receive UDP packets, with statsd formatted
measurements contained with them. Statsd style counts and gauges will
be handled as described above. Timers and histograms are silently
ignored.

## Scraping Metrics via Prometheus.

When the program is started with `-prom-url URL`, and `-prom-interval
INT`, a [Prometheus][prometheus] scraper scrapes the endpoint every
`INT` seconds, and stores the metrics locally pending the flush
interval.

There are a few quirky items to discuss in this process. Gauges in
Prometheus are directly compatible with our interpretation. Counters
are treated as derived counters (see above). We drop Histograms due to
the less than meaningful data it would provide in our context, but we
_do_ capture and report at least the non-quantile values from the
Summary type. These values are reported as, again, derived counters
with special names: `{name of metric}_sum + rest` and `{name of
metric}_count + rest`. `rest`, in this case is a statsd encoding of
the label pairs (with name and value separated by `_`) in the parse
order of the metric.


[statsd]: https://github.com/b/statsd_spec
[etsy-statsd]: https://github.com/etsy/statsd
[prometheus]: https://prometheus.io
