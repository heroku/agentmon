# agentmon: Secret Dyno Metrics Agent Mon

![agent mon](https://i.imgur.com/0qtodUm.png)

## Overview

Agentmon reads language metrics from Prometheus or a StatsD source and posts them to the language metrics sink, for 
example `https://app.metrics.heroku.com/<dyno id>`.
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
        UDP port for statsd listener
  -version
        print version string
```
Language metrics are supported for Go, JVM, and Ruby. 

#### Go Metrics 
* Counters
     - go.gc.collections
     - go.gc.pause.ns
* Gauges
    - go.memory.heap.bytes
    - go.memory.stack.bytes
    - go.memory.heap.objects
    - go.gc.goal
    - go.routines
#### JVM Metrics 
* Counters
    - jvm_gc_collection_seconds_count.gc_PS_Scavenge
    - jvm_gc_collection_seconds_sum.gc_PS_Scavenge
* Gauges
    - jvm_memory_bytes_used.area_heap
    - jvm_memory_bytes_used.area_nonheap
    - jvm_buffer_pool_bytes_used.name_direct
    - jvm_buffer_pool_bytes_used.name_mapped
#### Ruby Metrics
* Gauges
    - Rack.Server.All.GC.heap_free_slots
    - Rack.Server.All.GC.total_allocated_objects
    - Rack.Server.All.GC.total_freed_objects
    



## Copyright

(c) 2017, Heroku, Inc. See [LICENSE](./LICENSE) for details.
