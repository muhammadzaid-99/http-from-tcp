package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"http-from-tcp/internal/headers"
	"http-from-tcp/internal/request"
	"http-from-tcp/internal/response"
	"http-from-tcp/internal/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const port = 42069

func getBody(sc response.StatusCode) []byte {
	var body string
	switch sc {
	case response.StatusBadRequest:
		body = `<html>
		<head>
		<title>400 Bad Request</title>
		</head>
		<body>
		<h1>Bad Request</h1>
		<p>Your request honestly kinda sucked.</p>
		</body>
		</html>`
	case response.StatusInternalServerError:
		body = `<html>
		<head>
		<title>500 Internal Server Error</title>
		</head>
		<body>
		<h1>Internal Server Error</h1>
		<p>Okay, you know what? This one is on me.</p>
		</body>
		</html>`
	case response.StatusOK:
		body = `<html>
		<head>
		<title>200 OK</title>
		</head>
		<body>
		<h1>Success!</h1>
		<p>Your request was an absolute banger.</p>
		</body>
		</html>`
	}
	return []byte(body)
}

func main() {
	s, err := server.Serve(port, func(w *response.Writer, req *request.Request) {
		h := response.GetDefaultHeaders(0)
		var body bytes.Buffer
		var status response.StatusCode
		switch {
		case req.RequestLine.RequestTarget == "/yourproblem":
			status = response.StatusBadRequest
			body.Write(getBody(status))
		case req.RequestLine.RequestTarget == "/myproblem":
			status = response.StatusInternalServerError
			body.Write(getBody(status))
		case req.RequestLine.RequestTarget == "/video":
			f, err := os.ReadFile("assets/vim.mp4")
			if err != nil {
				status = response.StatusInternalServerError
				break
			}
			h.Set("content-type", "video/mp4")
			h.Set("content-length", fmt.Sprint(len(f)))
			w.WriteStatusLine(response.StatusOK)
			w.WriteHeaders(h)
			w.WriteBody(f)

			return
		case strings.HasPrefix(req.RequestLine.RequestTarget, "/httpbin/stream"):
			target := req.RequestLine.RequestTarget
			countPath := strings.TrimPrefix(target, "/httpbin")
			res, err := http.Get("https://httpbin.org" + countPath)
			if err != nil {
				status = response.StatusInternalServerError
				// body = getBody(status)
				break
			}
			w.WriteStatusLine(response.StatusOK)
			h.Delete("content-length")
			h.Set("transfer-encoding", "chunked")
			h.Set("content-type", "text/plain")
			h.Add("trailer", "X-Content-SHA256")
			h.Add("trailer", "X-Content-Length")
			w.WriteHeaders(h)

			for {
				buf := make([]byte, 32)
				n, err := res.Body.Read(buf)
				if err != nil {
					break
				}

				body.Write(buf[:n])
				w.WriteBody(fmt.Appendf(nil, "%x\r\n", n))
				w.WriteBody(buf[:n])
				w.WriteBody([]byte("\r\n"))
			}
			// w.WriteBody([]byte("0\r\n\r\n")) // without trailer
			w.WriteBody([]byte("0\r\n"))
			t := headers.NewHeaders()
			hash := sha256.Sum256(body.Bytes())
			t.Set("X-Content-SHA256", hex.EncodeToString(hash[:]))
			t.Set("X-Content-Length", fmt.Sprint(body.Len()))
			w.WriteHeaders(*t)
			return

		default:
			status = response.StatusOK
		}

		w.WriteStatusLine(status)
		body.Write(getBody(status))
		h.Set("content-length", fmt.Sprint(body.Len()))
		h.Set("content-type", "text/html")
		w.WriteHeaders(h)
		w.WriteBody(body.Bytes())
	})
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer s.Close()
	log.Println("Server started on port", port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}
