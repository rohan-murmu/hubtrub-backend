# hubtrub — backend

Realtime backend for a multiplayer world. A Go WebSocket hub that accepts connections and
broadcasts player motion to everyone else in it, with a Godot client on the other end.

## Shape

```
              ┌──────────────── Hub ────────────────┐
  client ───▶ │ RegisterC    Clients map            │
  client ───▶ │ BroadcastC   ──▶ fan out to all     │ ──▶ client
  client ───▶ │ UnregisterC                         │ ──▶ client
              └─────────────────────────────────────┘
                     one goroutine, one select loop
```

A `Hub` owns a set of clients and three channels — register, unregister, broadcast — and
runs a single `select` loop over them. Nothing else touches the client map, so there are no
locks around it: registration, disconnects and fan-out are all serialised through the same
goroutine by construction.

Each client is a connection plus its own send buffer, so one slow reader cannot stall the
broadcast to everybody else.

## Hub per concern

`NewMotionHub()` and `NewSubscriptionHub()` both return a fresh `Hub`. That is deliberate,
and it is the whole design: position updates are high-frequency and disposable, subscription
traffic is low-frequency and must not be dropped. Giving them separate hubs means separate
channels and separate goroutines, so a burst of one does not queue behind the other.

Right now **only the motion hub is wired** — `/ws/motion`. The subscription hub is built and
commented out in `cmd/server/main.go`, waiting for the message types to settle.

## Run it

```bash
go run ./cmd/server        # listens on :8080
```

Connect a client to `ws://localhost:8080/ws/motion`. Anything one client sends is
broadcast to every other client on that hub.

## Layout

```
cmd/server/main.go       wiring: build hubs, run them, route /ws/*
internal/hub/hub.go      the hub: client set, channels, select loop
internal/hub/motion_hub.go
internal/hub/sub_hub.go
internal/client/client.go read/write pumps for one connection
internal/room/room.go    placeholder — rooms are not implemented yet
shared/types.go          message types shared with the client
```

## Status

Early. Motion broadcast works end to end against the Godot client. Rooms are a stub, the
subscription hub is not wired, and there is no auth or persistence — it broadcasts to every
connected client rather than to a room.

The companion repositories are [hubtrub-frontend-game](https://github.com/rohan-murmu/hubtrub-frontend-game)
(Godot) and [hubtrub-frontend-client](https://github.com/rohan-murmu/hubtrub-frontend-client).
