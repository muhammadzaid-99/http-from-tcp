package request

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type parserState int

const (
	StateInit parserState = iota
	StateDone
	StateError
)

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

type Request struct {
	RequestLine RequestLine
	state       parserState
}

func newRequest() *Request {
	return &Request{
		state: StateInit,
	}
}

var (
	ErrInvalidHTTPVersion   = errors.New("invalid HTTP version")
	ErrInvalidMethod        = errors.New("invalid method")
	ErrInvalidReqTarget     = errors.New("invalid request target")
	ErrMalformedRequestLine = fmt.Errorf("malformed request line")
	ErrIncompleteStartLine  = fmt.Errorf("incomplete start line")
	ErrReqInErrorState      = fmt.Errorf("request in error state")
)

const crlf = "\r\n"
const bufferSize = 1024

func (rl *RequestLine) validHTTPVersion() error {
	if rl.HttpVersion != "1.1" {
		return ErrInvalidHTTPVersion
	}
	return nil
}

func (rl *RequestLine) validMethod() error {
	switch rl.Method {
	case "GET", "POST":
		return nil
	}
	return ErrInvalidMethod
}

func (rl *RequestLine) validReqTarget() error {
	if !strings.HasPrefix(rl.RequestTarget, "/") {
		return ErrInvalidReqTarget
	}
	return nil
}

func parseRequestLine(b string) (*RequestLine, int, error) {
	startLine, _, ok := strings.Cut(b, crlf)
	if !ok {
		return nil, 0, nil
		// return nil, 0, ErrIncompleteStartLine
	}
	parts := strings.Split(startLine, " ")
	read := len(startLine) + len(crlf)

	// method, target and http version separated by  single space according to RFC
	if len(parts) != 3 {
		return nil, 0, ErrMalformedRequestLine
	}

	httpParts := strings.Split(parts[2], "/")
	if len(httpParts) != 2 || httpParts[0] != "HTTP" {
		return nil, 0, ErrMalformedRequestLine
	}

	rl := &RequestLine{
		Method:        parts[0],
		RequestTarget: parts[1],
		HttpVersion:   httpParts[1],
	}

	for _, check := range []func() error{
		rl.validHTTPVersion,
		rl.validMethod,
		rl.validReqTarget,
	} {
		if err := check(); err != nil {
			return nil, 0, err
		}
	}

	return rl, read, nil
}

func (r *Request) parse(data []byte) (int, error) {
	read := 0
outer:
	for {
		switch r.state {
		case StateError:
			return 0, ErrReqInErrorState
		case StateInit:
			rl, n, err := parseRequestLine(string(data[read:]))
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break outer
			}
			r.RequestLine = *rl
			read += n
			r.state = StateDone

		case StateDone:
			break outer
		}
	}
	return read, nil
}

func (r *Request) done() bool {
	return r.state == StateDone
}

func (r *Request) error() bool {
	return r.state == StateError
}

func RequestFromReader(reader io.Reader) (*Request, error) {
	request := newRequest()

	buffer := make([]byte, bufferSize) // TODO: take care of overflow
	bufferLength := 0
	for !request.done() && !request.error() {
		n, err := reader.Read(buffer[bufferLength:])
		if err != nil {
			return nil, err
		}
		bufferLength += n
		pn, err := request.parse(buffer[:bufferLength])
		if err != nil {
			return nil, err
		}

		copy(buffer, buffer[pn:bufferLength])
		bufferLength -= pn
	}

	// data, err := io.ReadAll(reader)
	// if err != nil {
	// 	return nil, errors.Join(fmt.Errorf("unable to io.ReadAll"), err)
	// }
	// str := string(data)
	// rl, _, err := parseRequestLine(str)
	// if err != nil {
	// 	return nil, err
	// }

	// return &Request{
	// 	RequestLine: *rl,
	// }, err

	return request, nil
}
