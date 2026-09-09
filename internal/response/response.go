package response

import (
	"errors"
	"fmt"
	"http-from-tcp/internal/headers"
	"io"
)

type Response struct {
}

type StatusCode int

const (
	StatusOK                  StatusCode = 200
	StatusBadRequest          StatusCode = 400
	StatusInternalServerError StatusCode = 500
)

func GetDefaultHeaders(contentLen int) headers.Headers {
	h := headers.NewHeaders()
	h.Set("Content-Length", fmt.Sprint(contentLen))
	h.Set("Connection", "close")
	h.Set("Content-Type", "text/plain")

	return *h
}

func WriteHeaders(w io.Writer, h headers.Headers) error {
	var b []byte
	h.ForEach(func(key, value string) {
		b = fmt.Appendf(b, "%s: %s\r\n", key, value)
	})
	b = fmt.Append(b, "\r\n")
	_, err := w.Write(b)
	return err
}

func WriteStatusLine(w io.Writer, statusCode StatusCode) error {
	var line []byte
	switch statusCode {
	case StatusOK:
		line = []byte("HTTP/1.1 200 OK\r\n\r\n")
	case StatusBadRequest:
		line = []byte("HTTP/1.1 400 Bad Request\r\n\r\n")
	case StatusInternalServerError:
		line = []byte("HTTP/1.1 500 Internal Server Error\r\n\r\n")
	default:
		return errors.New("unknown status code")
	}
	_, err := w.Write(line)
	return err
}

type Writer struct {
	io.Writer
}

func NewWriter(writer io.Writer) *Writer {
	return &Writer{Writer: writer}
}

func (w *Writer) WriteStatusLine(statusCode StatusCode) error {
	var line []byte
	switch statusCode {
	case StatusOK:
		line = []byte("HTTP/1.1 200 OK \r\n")
	case StatusBadRequest:
		line = []byte("HTTP/1.1 400 Bad Request \r\n")
	case StatusInternalServerError:
		line = []byte("HTTP/1.1 500 Internal Server Error \r\n")
	default:
		return errors.New("unknown status code")
	}
	_, err := w.Write(line)
	return err
}
func (w *Writer) WriteHeaders(headers headers.Headers) error {
	var b []byte
	headers.ForEach(func(key, value string) {
		b = fmt.Appendf(b, "%s: %s\r\n", key, value)
	})
	b = fmt.Append(b, "\r\n")
	_, err := w.Write(b)
	return err
}

func (w *Writer) WriteBody(p []byte) (int, error) {
	n, err := w.Write(p)
	return n, err
}
