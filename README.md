# HTTP From TCP

A working HTTP/1.1 server written in Go that speaks the protocol directly over a TCP socket. Nothing in the request path uses Go's `net/http` server. Requests are read off the connection as raw bytes and parsed by hand, and responses are written out as raw status lines, header blocks and body bytes.

You give it a port and a handler function. It hands your handler a parsed request and a writer, and you decide what goes on the wire.

There is a demo server in `cmd/httpserver` that uses the library. It serves HTML pages, streams a video file, and proxies a streaming endpoint from httpbin.org back to the client using chunked transfer encoding with trailers.

## What it does

**Incremental request parsing.** The server never assumes a whole request arrived in one read. It feeds whatever bytes it has into a parser that consumes as much as it can and then asks for more.

**Header modelling that matches the spec.** Field names are case insensitive, repeated fields fold together into one value, and names are validated against the character set the spec actually allows.

**Concurrent connections.** Each accepted connection is handled on its own goroutine, with a clear open and close point.

**Response control in the handler's hands.** Handlers write the status line, headers and body themselves, in that order, which is what makes streaming responses possible.

**Chunked transfer encoding with trailers.** The demo proxies a remote stream and forwards it chunk by chunk without buffering the whole thing first, then sends checksum fields after the body has finished.

## Running it

Needs Go 1.25 or newer.

```
go run ./cmd/httpserver
```

The server listens on port 42069.

| Route | What happens |
| --- | --- |
| `/httpbin/stream/N` | Proxies `https://httpbin.org/stream/N` and re-encodes it as chunked with trailers |
| `/video` | Serves `assets/vim.mp4` as `video/mp4` |
| `/yourproblem` | Returns 400 with an HTML error page |
| `/myproblem` | Returns 500 with an HTML error page |
| anything else | Returns 200 with an HTML page |

The `/video` route reads `assets/vim.mp4` relative to the working directory. That file is not in the repo, so put any mp4 there to try it. The port is a constant at the top of `cmd/httpserver/main.go`.

Ctrl-C shuts the server down cleanly. It waits on SIGINT and SIGTERM rather than dying where it stands.

Two smaller programs came out of building this and are still here:

| Command | What it is |
| --- | --- |
| `go run ./cmd/tcplistener` | Accepts a connection, parses it with the same parser, and prints the request line, headers and body |
| `go run ./cmd/udpsender` | Reads lines from stdin and sends each one as a UDP datagram to localhost:42069 |

Run the tests with `go test ./...`.

## How it works

### Reading a request when you do not know how long it is

TCP gives you a stream of bytes, not messages. A single read can return three bytes, or half a header line, or two requests at once. The parser cannot just wait for the whole request to show up, because nothing tells it where the request ends until it has already parsed the headers.

The approach is a state machine that the caller drives. `Request.parse` takes whatever bytes are available right now and returns how many of them it actually consumed. It moves through four states: the request line, then the header block, then the body, then done. If it runs out of data half way through a line it stops and holds its position, so the next call picks up exactly where it left off.

`RequestFromReader` in `internal/request/request.go` drives that loop. It keeps a fixed 1 KB buffer, reads new bytes into the free space at the end, hands the filled portion to the parser, then shifts whatever was not consumed back to the front so the buffer never has to grow.

The request line is validated as it is parsed. The version must be 1.1, the target must start with a slash, and the method has to be one the server supports. A failure puts the parser into an error state that it does not leave.

What this buys is that the server behaves the same no matter how the writes from the client happen to be split, and parsing costs a constant amount of memory whatever the request size. The tests lean on this directly: `request_test.go` uses a reader that deliberately hands over one to three bytes per call, so a parser that quietly assumed it would get whole lines would fail on the first case.

### Deciding when the body ends

Once the blank line closing the headers is consumed, the parser has to work out whether there is a body coming at all. It looks for `content-length`. If there is no such header it moves straight to done, which matters more than it sounds: without that shortcut the next read would sit there blocking, or return EOF, on a request that was already complete. If the header is present the parser reads exactly that many bytes and not one more, which is what keeps the connection in a state you can reason about.

### Headers as a data model, not a map

Header names are case insensitive, the same name is allowed to appear more than once, and only certain characters are legal in a name. A plain `map[string]string` gets all three of those wrong.

`internal/headers` wraps a map and lowercases every key on the way in and on the way out, so `Content-Length` and `content-length` are the same field. `Add` appends to an existing value with a comma rather than overwriting it, which is how repeated fields are supposed to combine. `Set` replaces outright, for the cases where you want exactly one value. Names are checked against the allowed token characters before being accepted, and a name with whitespace sitting before the colon is rejected instead of quietly trimmed, because that shape has a long history of being used to slip requests past proxies.

`Parse` follows the same contract as the request parser. It returns the number of bytes consumed and whether it reached the terminating blank line, so it can be called over and over as data arrives.

### Connection lifecycle

`server.Serve` binds the port and returns immediately, leaving the accept loop running on its own goroutine. Every connection it accepts is then handled on another goroutine, so one slow client cannot hold up anyone else.

A connection is closed when its handler returns, through a deferred close, so there is exactly one place where that happens. The default response headers include `Connection: close`, which makes the model one request per connection and tells the client so explicitly.

If parsing fails, the handler is never called at all. The server writes a 400 itself and drops the connection. A handler should not have to defend itself against malformed input, so it simply never sees any.

### Writing responses in the right order

HTTP is order sensitive. Status line, then headers, then body, and once bytes have gone out you cannot take them back. If the writer were one big `Write(status, headers, body)` call, streaming would be impossible, because you would need the entire body in hand before sending anything at all.

`response.Writer` splits this into `WriteStatusLine`, `WriteHeaders` and `WriteBody`, and wraps any `io.Writer`. A handler that knows its content up front sets `content-length` and writes once. A handler that is streaming writes its headers first and then keeps calling `WriteBody` as data shows up.

`GetDefaultHeaders` gives back a reasonable starting set that the handler edits before sending. The proxy route, for instance, deletes `content-length` and sets `transfer-encoding: chunked`, since those two cannot both be there.

### Chunked transfer encoding and trailers

The proxy route is the reason the writer is built this way. It fetches a stream from httpbin.org and forwards it on to the client, but it has no idea how many bytes are coming. With no `content-length` there is nothing to tell the client where the body stops.

Chunked encoding solves that by framing the body itself. Each chunk goes out as its length in hexadecimal, a CRLF, the bytes, and another CRLF, and a zero length chunk marks the end. The handler reads 32 bytes at a time from the upstream response and writes each read out as its own chunk, so data reaches the client while it is still arriving rather than after the upstream has finished.

Trailers are what make this properly useful. Because the terminating chunk comes after the body, you can send header fields that could not possibly have been computed earlier. The handler advertises them up front with a `Trailer` header, and then after the last chunk it sends `X-Content-SHA256` and `X-Content-Length`, a checksum and a byte count covering everything it just streamed. The client can verify the body it received, and the server never had to hold that body in memory to say anything about it.

## Project layout

| Path | What lives there |
| --- | --- |
| `internal/request` | The incremental request parser and its state machine |
| `internal/headers` | Header storage, parsing and field name validation |
| `internal/response` | Status line, header and body writing |
| `internal/server` | Listener, accept loop and per connection handling |
| `cmd/httpserver` | The demo server and its route handlers |
| `cmd/tcplistener`, `cmd/udpsender` | Small tools from earlier stages of the build |

## Reference

Built by working through the [Learn HTTP Protocol](https://www.boot.dev/courses/learn-http-protocol-golang) course on Boot.dev, following along with [ThePrimeagen's walkthrough of it](https://www.youtube.com/watch?v=FknTw9bJsXM).

The behaviour here follows the HTTP/1.1 specifications rather than restating them:

- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html), HTTP semantics: methods, status codes, field name rules, trailers
- [RFC 9112](https://www.rfc-editor.org/rfc/rfc9112.html), HTTP/1.1 message syntax: the request line, the header block, chunked transfer coding
