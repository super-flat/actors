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
