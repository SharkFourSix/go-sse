# go-sse

[![Go Reference](https://pkg.go.dev/badge/github.com/tmaxmax/go-sse.svg)](https://pkg.go.dev/github.com/tmaxmax/go-sse)
![CI](https://github.com/tmaxmax/go-sse/actions/workflows/go.yml/badge.svg)
![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-95%25-brightgreen.svg?longCache=true&style=flat)</a>
[![Go Report Card](https://goreportcard.com/badge/github.com/tmaxmax/go-sse)](https://goreportcard.com/report/github.com/tmaxmax/go-sse)

Lightweight, fully spec-compliant HTML5 server-sent events library.

## Table of contents

- [go-sse](#go-sse)
  - [Table of contents](#table-of-contents)
  - [Installation and usage](#installation-and-usage)
  - [Implementing a server](#implementing-a-server)
    - [Creating the server instance](#creating-the-server-instance)
    - [Meet Joe](#meet-joe)
    - [Publish your first event](#publish-your-first-event)
    - [The server-side "Hello world"](#the-server-side-hello-world)
  - [Using the client](#using-the-client)
    - [Creating a client](#creating-a-client)
    - [Initiating a connection](#initiating-a-connection)
    - [Subscribing to events](#subscribing-to-events)
    - [Receiving events](#receiving-events)
    - [Establishing the connection](#establishing-the-connection)
    - [Connection lost?](#connection-lost)
    - [The "Hello world!" server's client](#the-hello-world-servers-client)
  - [License](#license)
  - [Contributing](#contributing)

## Installation and usage

Install the package using `go get`:

```sh
go get -u github.com/tmaxmax/go-sse
```

The library is split into two subpackages, named suggestively: `server` and `client`. The implementations are completely decoupled and unopinionated: you can connect to a server created using `go-sse` from the browser and you can connect to any server that emits events using the client!

If you are not familiar with the protocol or not sure how it works, read [MDN's guide for using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events). [The spec](https://html.spec.whatwg.org/multipage/server-sent-events.html) is also useful read!

## Implementing a server

### Creating the server instance

First, a server instance has to be created:

```go
import "github.com/tmaxmax/go-sse/server"

sse := server.New()
```

The `server.Server` type also implements the `http.Handler` interface, but a server is framework-agnostic: See the [`ServeHTTP` implementation](https://github.com/tmaxmax/go-sse/blob/master/server/server.go#L118) to learn how to implement your own custom logic.

The `New` constructor actually takes in an optional parameter, a `Provider`:

```go
package server

func New(provider ...Provider) *Server
```

A provider is an implmenetation of the publish-subscribe messaging pattern:

```go
type Provider interface {
    Publish(msg Message) error
    Subscribe(ctx context.Context, sub Subscription) error
    Stop() error
}
```

It can be anything: a pure Go implementation or an adapter for an external service or tool, such as Redis or RabbitMQ, you name it!

Read about the Provider interface in the [docs][2].

### Meet Joe

The server still works by default, without a provider. `go-sse` brings you Joe: the trusty, pure Go pub-sub pattern, who handles all your events by default! Befriend Joe as following:

```go
import "github.com/tmaxmax/go-sse/server"

joe := server.NewJoe()
```

and he'll dispatch events all day! By default, he has no memory of what events he has received, but you can help him remember and replay older messages to new clients using a `ReplayProvider`:

```go
type ReplayProvider interface {
    Put(msg *Message)
    Replay(sub Subscription)
    GC() error
}
```

`go-sse` provides two replay providers by default, which both hold the events in-memory: the `ValidReplayProvider` and `FiniteReplayProvider`. The first replays events that are valid, not expired, the second replays a finite number of the most recent events. For example:

```go
server.NewJoe(server.JoeConfig{
    ReplayProvider: server.NewValidReplayProvider(),
    ReplayGCInterval: time.Minute,
})
```

will tell Joe to replay all valid events and clean up the expired ones each minute! Replay providers can do so much more (for example, add IDs to events automatically): read the [docs][3] on how to use the existing ones and how to implement yours.

### Publish your first event

The `server` package has a nimble subpackage named `event`. Let's create an event:

```go
import "github.com/tmaxmax/go-sse/server/event"

e := event.Event{}
e.AppendText("Hello world!", "Nice\nto see you.")
```

Now let's send it to our clients:

```go
var sse *server.Server

sse.Publish(&e)
```

This is how clients will receive our event:

```txt
data: Hello world!
data: Nice
data: to see you.
```

If we use a replay provider, such as `ValidReplayProvider`, this event will expire immediately and it also doesn't have an ID. Let's solve this:

```go
e.SetID(event.MustID("unique"))
e.SetTTL(5 * time.Minute)
```

Now the event will look like this:

```txt
id: unique
data: Hello world!
data: Nice
data: to see you.
```

And the ValidReplayProvider will stop replaying it after 5 minutes!

The `event` package also exposes an `ID` type, which is a special type that denotes an event's ID. An ID must not have newlines, so we use a special function that validates the ID beforehand. `MustID` panics, but there's also `NewID`, which returns a boolean flag indicating whether the ID is valid or not:

```go
id, ok := event.NewID("invalid\nID")
```

Here, `ok` will be `false` and `id` will be an invalid value: nothing will be sent to clients if you set an event's ID using that value!

Either way, IDs and expiry times can also be retrieved, so replay providers can use them to determine which IDs to replay and which are still valid:

```go
fmt.Println(e.ID(), e.ExpiresAt())
```

Note that the `Event` type used on the server-side is different from the one used by the client - we'll present it later. Read the [docs][4] to find out more about events and how to use them!

### The server-side "Hello world"

Now, let's put everything that we've learned together! We'll create a server that sends a "Hello world!" message every second to all its clients, with Joe's help:

```go
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/tmaxmax/go-sse/server"
    "github.com/tmaxmax/go-sse/server/event"
)

func main() {
    sse := server.New()

    go func() {
        ev := &event.Event{}
        ev.AppendText("Hello world")

        for range time.Tick(time.Second) {
            _ = sse.Publish(ev)
        }
    }()

    if err := http.ListenAndServe(":8000", sse); err != nil {
        log.Fatalln(err)
    }
}
```

Joe is our default provider here, as no provider is given to the server constructor. The server is already an `http.Handler` so we can use it directly with `http.ListenAndServe`.

[Also see a more complex example!](cmd/complex/main.go)

This is by far a complete presentation, make sure to read the docs in order to use `go-sse` to its full potential!

## Using the client

### Creating a client

Under the `client` package, we find the `Client` type:

```go
type Client struct {
    HTTPClient              *http.Client
    OnRetry                 backoff.Notify
    ResponseValidator       ResponseValidator
    MaxRetries              int
    DefaultReconnectionTime time.Duration
}
```

As you can see, it uses a `net/http` client. It also uses the [cenkalti/backoff][1] library for implementing auto-reconnect when a connection to a server is lost. Read the [client docs][5] and the Backoff library's docs to find out how to configure the client. We'll use the default client the package provides for further examples.

### Initiating a connection

We must first create an `http.Request` - yup, a fully customizable request:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, "host", nil)
```

Any kind of request is valid as long as your server handler supports it: you can do a GET, a POST, send a body; do whatever! The context is used as always for cancellation - to stop receiving events you will have to cancel the context.
Let's initiate a connection with this request:

```go
import "github.com/tmaxmax/go-sse/client"

conn := client.DefaultClient.NewConnection(req)
// you can also do client.NewConnection(req)
// it is an utility function that calls the
// NewConnection method on the default client
```

### Subscribing to events

Great! Let's imagine the event stream looks as following:

```txt
data: some unnamed event

event: I have a name
data: some data

event: Another name
data: some data
```

To receive the unnamed events, we subscribe to them as following:

```go
unnamedEvents := make(chan client.Event)
conn.SubscribeMessages(unnamedEvents)
```

To receive the events named "I have a name":

```go
namedEvents := make(chan client.Event)
conn.SubscribeEvent("I have a name", namedEvents)
```

We can susbcribe to multiple event types using the same channel:

```go
all := make(chan client.Event)
conn.SubscribeMessages(all)
conn.SubscribeEvent("I have a name", all)
conn.SubscribeEvent("Another name", all)
```

The code above will subscribe the channel to all events. But there's a shorthand for this, which is useful especially when you don't know all event names:

```go
conn.SubscribeToAll(all)
```

### Receiving events

Before we establish the connection, we must setup some goroutines to receive the events from the channels.

Let's start with the client's `Event` type:

```go
type Event struct {
    LastEventID string
    Name        string
    Data        []byte
}

func (e Event) String() { return string(e.Data) }
```

Pretty self-explanatory, but make sure to read the [docs][6]!

Let's start a goroutine that receives from the `unnamedEvents` channel created above:

```go
go func() {
    for e := range unnamedEvents {
        fmt.Printf("Received an unnamed event: %s": e)
    }
}()
```

This will print the data from each unnamed event to `os.Stdout`. Don't forget to syncronize access to shared resources that are not thread-safe, as `os.Stdout` is!

### Establishing the connection

Great, we are subscribed now! Let's start receiving events:

```go
err := conn.Connect()
```

By calling `Connect`, the request created above will be sent to the server, and if successful, the subscribed channels will start receiving new events.

Let's say we want to stop receiving events named "Another name", we can unsubscribe:

```go
conn.UnsubscribeEvent("Another name", ch)
```

If `ch` is a channel that's subscribed using `SubscribeToAll` or is not subscribed to "Another name" events nothing will happen. Make sure to call `Unsubscribe` methods from a different goroutine than the one that receives from the channel, as it might result in a deadlock! If you don't know to what events a channel is subscribed to, but want to unsubscribe from all of them, use `UnsubscribeFromAll`.

### Connection lost?

Either way, after receiving so many events, something went wrong and the server is temporarily down. Oh no! As a last hope, it has sent us the following event:

```text
retry: 60000
: that's a minute in milliseconds and this
: is a comment which is ignored by the client
```

Not a sweat, though! The connection will automatically be reattempted after a minute, when we'll hope the server's back up again. Canceling the request's context will cancel any reconnection attempt, too.

If the server doesn't set a retry time, the client's `DefaultReconnectionTime` is used.

### The "Hello world!" server's client

Let's use what we know to create a client for the prevoius server example:

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/tmaxmax/go-sse/client"
)

func main() {
    r, _ := http.NewRequest(http.MethodGet, "http://localhost:8000", nil)
    conn := client.NewConnection(r)
    ch := make(chan client.Event)

    conn.SubscribeMessages(ch)

    go func() {
        for ev := range ch {
            fmt.Printf("%s\n\n", ev.Data)
        }
    }()

    if err := conn.Connect(); err != nil {
        log.Println(err)
    }
}
```

Yup, this is it! We are using the default client to receive all the unnamed events from the server. The output will look like this, when both programs are run in parallel:

```txt
Hello world!

Hello world!

Hello world!

Hello world!

...
```

[See the complex example's client too!](cmd/complex_client/main.go)

## License

This project is licensed under the [MIT license](LICENSE).

## Contributing

The library's in its early stages, so contributions are vital - I'm so glad you wish to improve `go-sse`! Maybe start by opening an issue first, to describe the intendended modifications and further discuss how to integrate them. Open PRs to the `master` branch and wait for CI to complete. If all is clear, your changes will soon be merged! Also, make sure your changes come with an extensive set of tests and the code is formatted.

Thank you for contributing!

[1]: https://github.com/cenkalti/backoff
[2]: https://pkg.go.dev/github.com/tmaxmax/go-sse/server#Provider
[3]: https://pkg.go.dev/github.com/tmaxmax/go-sse/server#ReplayProvider
[4]: https://pkg.go.dev/github.com/tmaxmax/go-sse/server/event
[5]: https://pkg.go.dev/github.com/tmaxmax/go-sse/client
[6]: https://pkg.go.dev/github.com/tmaxmax/go-sse/client#Event