package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderParse(t *testing.T) {
	// Test: Valid single header
	headers := NewHeaders()
	data := []byte("Host: localhost:42069\r\n\r\n")
	n, done, err := headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	host, ok := headers.Get("Host")
	assert.Equal(t, "localhost:42069", host)
	assert.Equal(t, true, ok)
	host, ok = headers.Get("miss")
	assert.Equal(t, "", host)
	assert.Equal(t, false, ok)
	assert.Equal(t, 23, n)
	assert.True(t, done)

	// Test: Valid multiple headers
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\na:   b\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	host, _ = headers.Get("Host")
	assert.Equal(t, "localhost:42069", host)
	host, _ = headers.Get("a")
	assert.Equal(t, "b", host)
	assert.Equal(t, 31, n)
	assert.True(t, done)

	// Test: Valid multiple values of a header
	headers = NewHeaders()
	data = []byte("Host: localhost:42069\r\nHost: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.NoError(t, err)
	require.NotNil(t, headers)
	host, ok = headers.Get("Host")
	assert.Equal(t, "localhost:42069,localhost:42069", host)
	assert.Equal(t, true, ok)
	assert.Equal(t, 46, n)
	assert.True(t, done)

	// Test: Invalid spacing header
	headers = NewHeaders()
	data = []byte("       Host: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	// Test: Invalid spacing header
	headers = NewHeaders()
	data = []byte("Host : localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)

	// Test: Invalid field name
	headers = NewHeaders()
	data = []byte("H©st: localhost:42069\r\n\r\n")
	n, done, err = headers.Parse(data)
	require.Error(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, done)
}
