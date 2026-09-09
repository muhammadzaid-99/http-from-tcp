package request

import (
	"errors"
	"http-from-tcp/internal/headers"
	"io"
	"strconv"
	"strings"
)

type parserState int

const (
	StateInit parserState = iota
	StateHeaders
	StateDone
	StateBody
	StateError
)

type RequestLine struct {
	HttpVersion   string
	RequestTarget string
	Method        string
}

type Request struct {
	RequestLine RequestLine
	Headers     *headers.Headers
	Body        string
	state       parserState
}

func newRequest() *Request {
	return &Request{
		Headers: headers.NewHeaders(),
		Body:    "",
		state:   StateInit,
	}
}

var (
	ErrInvalidHTTPVersion   = errors.New("invalid HTTP version")
	ErrInvalidMethod        = errors.New("invalid method")
	ErrInvalidReqTarget     = errors.New("invalid request target")
	ErrMalformedRequestLine = errors.New("malformed request line")
	ErrIncompleteStartLine  = errors.New("incomplete start line")
	ErrReqInErrorState      = errors.New("request in error state")
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
loop:
	for {
		buf := data[read:]
		if len(buf) == 0 {
			break loop
		}
		switch r.state {
		case StateError:
			return 0, ErrReqInErrorState
		case StateInit:
			rl, n, err := parseRequestLine(string(buf))
			if err != nil {
				r.state = StateError
				return 0, err
			}
			if n == 0 {
				break loop
			}
			r.RequestLine = *rl
			read += n
			r.state = StateHeaders

		case StateHeaders:
			n, done, err := r.Headers.Parse(buf)
			if err != nil {
				r.state = StateError
				return 0, err
			}

			read += n
			if done {
				// since does not contain last crlf, beacuse of test cases given
				read += len(crlf)
				if _, exists := r.Headers.Get("content-length"); exists {
					r.state = StateBody
				} else {
					// so eof does not come on requests that have no body on next read
					// we gracefully move to done
					r.state = StateDone
				}
			}

			if n == 0 {
				break loop
			}

		case StateBody:
			conLenStr, exists := r.Headers.Get("content-length")
			if !exists {
				r.state = StateDone
				break
			}

			conLen, err := strconv.Atoi(conLenStr)
			if err != nil {
				r.state = StateDone // if content length is not convertible, consider 0
				break
			}
			if conLen == 0 {
				r.state = StateDone
				break
			}

			n := min(conLen-len(r.Body), len(buf))
			if n == 0 {
				break loop
			}

			r.Body += string(buf[:n])
			read += n

			if len(r.Body) == conLen {
				r.state = StateDone
			}

		case StateDone:
			break loop
		default:
			panic("nice job")
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
		// if err == io.EOF {
		// 	break
		// }
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
	return request, nil
}
