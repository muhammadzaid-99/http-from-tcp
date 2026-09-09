package headers

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

type Headers struct {
	headers map[string]string
}

var (
	crlf         = []byte("\r\n")
	fieldLineSep = []byte(":")
)

var (
	ErrParseHeader      = errors.New("could not parse malformed header")
	ErrInvalidFieldName = errors.New("field name contains invalid characters")
)

func NewHeaders() *Headers {
	return &Headers{
		headers: make(map[string]string),
	}
}

func (h *Headers) Get(name string) (string, bool) {
	val, ok := h.headers[strings.ToLower(name)]
	return val, ok
}

func (h *Headers) Count() int {
	return len(h.headers)
}

func (h *Headers) Set(name, value string) {
	h.headers[strings.ToLower(name)] = value
}

func (h *Headers) Add(name, value string) {
	name = strings.ToLower(name)
	if prev, ok := h.headers[name]; ok {
		h.headers[name] = fmt.Sprintf("%s,%s", prev, value)
	} else {
		h.headers[name] = value
	}
}
func (h *Headers) Delete(name string) {
	delete(h.headers, strings.ToLower(name))
}

func (h *Headers) ForEach(callback func(key, value string)) {
	for k, v := range h.headers {
		callback(k, v)
	}
}

func (h *Headers) Parse(data []byte) (n int, done bool, err error) {
	read := 0
	for {
		index := bytes.Index(data[read:], crlf)
		if index == -1 {
			break
		}

		if index == 0 {
			done = true
			break
		}

		name, val, err := parseHeader(data[read : read+index])
		if err != nil {
			return 0, false, err
		}
		read += index + len(crlf)
		if !isToken(name) {
			return 0, false, ErrInvalidFieldName
		}
		h.Add(name, val)
	}

	return read, done, nil
}

func parseHeader(fieldLine []byte) (string, string, error) {
	name, val, found := bytes.Cut(fieldLine, fieldLineSep) // cut ensures first instance of sep
	if !found {
		return "", "", ErrParseHeader
	}

	// name = bytes.TrimLeft(name, " ")
	val = bytes.TrimSpace(val)

	if bytes.HasSuffix(name, []byte(" ")) || bytes.HasPrefix(name, []byte(" ")) {
		return "", "", ErrParseHeader
	}

	return string(name), string(val), nil
}

func isToken(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			continue
		}
		switch ch {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		}
		return false
	}
	return true
}
