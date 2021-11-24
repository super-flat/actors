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