package server

import (
	"fmt"
	"http-from-tcp/internal/request"
	"http-from-tcp/internal/response"
	"io"
	"net"
)

type HandlerError struct {
	StatusCode response.StatusCode
	Message    string
}

type Handler func(w *response.Writer, req *request.Request)

type Server struct {
	handler Handler
	closed  bool
}

func (s *Server) handle(conn io.ReadWriteCloser) {
	defer conn.Close()

	// headers := response.GetDefaultHeaders(0)
	// writer := bytes.NewBuffer(nil)
	writer := response.NewWriter(conn)
	req, err := request.RequestFromReader(conn)
	if err != nil {
		writer.WriteStatusLine(response.StatusBadRequest)
		writer.WriteHeaders(response.GetDefaultHeaders(0))
		return
	}

	// body := []byte(nil)
	// status := response.StatusOK

	s.handler(writer, req)
	// if handlerErr != nil {
	// 	status = handlerErr.StatusCode
	// 	body = []byte(handlerErr.Message)
	// } else {
	// 	body = writer.Bytes()
	// }

	// headers.Set("content-length", fmt.Sprint(len(body)))
	// response.WriteStatusLine(conn, status)
	// response.WriteHeaders(conn, headers)
	// conn.Write(body)
}

func (s *Server) listen(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if s.closed {
			return
		}
		if err != nil {
			return
		}

		go s.handle(conn)
	}
}

func Serve(port uint16, handler Handler) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	server := &Server{
		closed:  false,
		handler: handler,
	}
	go server.listen(listener)
	return server, nil
}

func (s *Server) Close() error {
	s.closed = true
	return nil
}
