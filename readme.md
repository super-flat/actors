# Actors

POC building an actor system that passivate. For now, actors are marked for deletion every 5 seconds, and the cache passivates them every 7 seconds.

```sh
# protogen
earthly +protogen

# vendor
go mod vendor

# run
go run ./sample run
```

## Features

- [x] Ask message pattern
- [x] Actor ref acting like erlang PID
- [x] Tracing using open-telemetry
- [x] Actors dispatcher
- [ ] Configuration (env vars, toml, etc...)
- [ ] Metrics
- [ ] Supervisor strategy
- [ ] Tell message pattern
- [ ] Forward message pattern
- [ ] Clustering
- [ ] Unit tests
- [ ] Benchmark tests

## Clustering principles

- actors belong to a specific shard and do not move between shards
- shards have an ID (number)
- shards can move between nodes
- one shard coordinator per node, which contains many shards
- v1 implementation will have a fixed number of shards and position actors in shards by taking hash modulo of the actor ID (like akka cluster or kafka partitions)
