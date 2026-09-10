# HTTP From TCP

An HTTP/1.1 server written in Go that speaks the protocol directly over a TCP socket. No `net/http` in the request path. Requests are read off the connection as raw bytes and parsed by hand, and responses go out as raw status lines, header blocks and body bytes.

You give it a port and a handler. It gives your handler a parsed request and a writer, and you decide what goes on the wire.

`cmd/httpserver` is a demo built on top of it: HTML pages, a video stream, and a proxy that re-encodes an httpbin.org stream as chunked with trailers.

## Features

- Incremental request parsing that never assumes a whole request arrived in one read
- Header handling per spec: case insensitive names, repeated fields folded, names validated
- One goroutine per connection
- Handlers write the status line, headers and body themselves, so responses can stream
- Chunked transfer encoding with trailers

## Running it

Needs Go 1.25 or newer.

```
go run ./cmd/httpserver
```

Listens on port 42069. Ctrl-C shuts it down cleanly (waits on SIGINT and SIGTERM).

| Route | Response |
| --- | --- |
| `/httpbin/stream/N` | Proxies `https://httpbin.org/stream/N`, re-encoded as chunked with trailers |
| `/video` | `assets/vim.mp4` as `video/mp4` |
| `/yourproblem` | 400 with an HTML error page |
| `/myproblem` | 500 with an HTML error page |
| anything else | 200 with an HTML page |

`/video` reads `assets/vim.mp4` relative to the working directory. That file is not in the repo, so drop any mp4 there to try it. The port is a constant at the top of `cmd/httpserver/main.go`.

Two smaller tools came out of building this:

| Command | What it does |
| --- | --- |
| `go run ./cmd/tcplistener` | Accepts a connection, parses it with the same parser, prints request line, headers and body |
| `go run ./cmd/udpsender` | Sends each stdin line as a UDP datagram to localhost:42069 |

Tests: `go test ./...`

## How it works

### Incremental parsing

TCP delivers a byte stream, not messages. One read can return three bytes, half a header line, or two requests at once.

- `Request.parse` takes whatever bytes are available and returns how many it consumed.
- Four states: request line, headers, body, done. Running out of data mid line stops the parser without losing its position.
- `RequestFromReader` keeps a fixed 1 KB buffer. Read into the free tail, parse, shift the unconsumed remainder to the front. The buffer never grows.
- The request line is validated as it is parsed: version 1.1, target starts with a slash, method is one the server supports. Failure moves the parser to a terminal error state.

Parsing memory stays constant whatever the request size, and it does not matter how the client split its writes. `request_test.go` uses a reader that hands over one to three bytes per call.

### Knowing where the body ends

After the blank line that closes the headers, the parser checks for `content-length`.

- Absent: go straight to done. Without this the next read would block or hit EOF on a request that was already complete.
- Present: read exactly that many bytes and no more, which leaves the connection in a known state.

### Headers

Names are case insensitive, can repeat, and only allow certain characters. A `map[string]string` gets all three wrong.

- Keys are lowercased on the way in and on the way out.
- `Add` folds a repeat into the existing value with a comma. `Set` replaces outright.
- Names are checked against the allowed token characters.
- A space before the colon is rejected rather than trimmed, since that shape has been used to slip requests past proxies.
- `Parse` is incremental like the request parser: returns bytes consumed plus whether the blank line was reached.

### Connection lifecycle

- `Serve` binds the port and returns. The accept loop runs on its own goroutine.
- Each connection gets its own goroutine, so a slow client blocks nobody.
- A deferred close means exactly one place closes the connection.
- Default headers carry `Connection: close`, so it is one request per connection and the client is told.
- A parse failure never reaches the handler. The server writes the 400 itself, so handlers never see malformed input.

### Response writing

HTTP is order sensitive and bytes already sent cannot be recalled. A single `Write(status, headers, body)` call would rule out streaming, since you would need the whole body before sending anything.

`response.Writer` wraps any `io.Writer` and splits the job into `WriteStatusLine`, `WriteHeaders` and `WriteBody`.

- Known content: set `content-length` and write once.
- Streaming: write headers first, then keep calling `WriteBody` as data arrives.
- `GetDefaultHeaders` returns a starting set the handler edits. The proxy deletes `content-length` and sets `transfer-encoding: chunked`, since those two cannot both be there.

### Chunked encoding and trailers

The proxy forwards a stream without knowing how long it is, so there is no `content-length` to mark where the body stops.

- Chunked encoding frames the body itself: hex length, CRLF, the bytes, CRLF, and a zero length chunk to finish.
- The handler reads 32 bytes at a time from upstream and sends each read as its own chunk, so data reaches the client while it is still arriving.
- Trailers come after the body, so they can carry values that could not be computed earlier. The handler advertises them with a `Trailer` header, then sends `X-Content-SHA256` and `X-Content-Length` covering everything it streamed.
- The client can verify the body it received, and the server never had to buffer it.

## Project layout

| Path | What lives there |
| --- | --- |
| `internal/request` | Incremental request parser and its state machine |
| `internal/headers` | Header storage, parsing and field name validation |
| `internal/response` | Status line, header and body writing |
| `internal/server` | Listener, accept loop and per connection handling |
| `cmd/httpserver` | Demo server and its route handlers |
| `cmd/tcplistener`, `cmd/udpsender` | Small tools from earlier stages of the build |

## Reference

Built by working through the [Learn HTTP Protocol](https://www.boot.dev/courses/learn-http-protocol-golang) course on Boot.dev, following along with [ThePrimeagen's walkthrough of it](https://www.youtube.com/watch?v=FknTw9bJsXM).

Behaviour follows the HTTP/1.1 specifications rather than restating them:

- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html), HTTP semantics: methods, status codes, field name rules, trailers
- [RFC 9112](https://www.rfc-editor.org/rfc/rfc9112.html), HTTP/1.1 message syntax: request line, header block, chunked transfer coding
