# transport

Lean publish/subscribe boundary for inventory change notifications inside the operator.

## Phase 1

- `Publisher` / `Subscriber` interfaces in `transport.go`
- `InProcessBus` — synchronous dispatch in-process (default wiring)

## Planned: NATS adapter (stub)

A future `nats.go` package will implement the same interfaces against
[NATS](https://nats.io/) or JetStream:

```go
// NATSAdapter — not implemented yet
type NATSAdapter struct {
    URL string
    // Publish: nats.Conn.Publish(subject, payload)
    // Subscribe: nats.Conn.Subscribe(subject, handler)
}
```

Use cases: debounced export triggers across reconcilers, hub CRD fan-out (ADR-0022), and
optional decoupling of collection from export workers. Until then, controllers call
`InProcessBus.Publish` directly after store updates.
