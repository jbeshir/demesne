package proxycommon

import (
	"bytes"
	"io"
)

// JSONInterceptor buffers a non-streaming JSON response body, forwards
// it to the caller as-is, and invokes onClose with the full byte slice
// when the body closes. Vendor-specific JSON parsing lives in onClose.
type JSONInterceptor struct {
	upstream io.ReadCloser
	onClose  func([]byte)
	tee      bytes.Buffer
}

// NewJSONInterceptor constructs a JSONInterceptor. onClose is called once at
// Close with the full buffered body; it is skipped if the body was empty.
func NewJSONInterceptor(upstream io.ReadCloser, onClose func([]byte)) *JSONInterceptor {
	return &JSONInterceptor{upstream: upstream, onClose: onClose}
}

func (j *JSONInterceptor) Read(p []byte) (int, error) {
	n, err := j.upstream.Read(p)
	if n > 0 {
		j.tee.Write(p[:n])
	}
	return n, err
}

func (j *JSONInterceptor) Close() error {
	defer func() { _ = j.upstream.Close() }()
	if j.tee.Len() > 0 {
		j.onClose(j.tee.Bytes())
	}
	return nil
}

// SSEInterceptor buffers SSE bytes, scans line-by-line via ScanSSELines,
// hands each complete line to onLine, and invokes onClose once at body close.
// The vendor's handleLine and flush callbacks are passed in at construction.
type SSEInterceptor struct {
	upstream io.ReadCloser
	onLine   func(string)
	onClose  func()
	buf      bytes.Buffer
	flushed  bool
}

// NewSSEInterceptor constructs an SSEInterceptor. onLine is called for each
// complete SSE line; onClose is called once when Close is called.
func NewSSEInterceptor(upstream io.ReadCloser, onLine func(string), onClose func()) *SSEInterceptor {
	return &SSEInterceptor{upstream: upstream, onLine: onLine, onClose: onClose}
}

func (s *SSEInterceptor) Read(p []byte) (int, error) {
	n, err := s.upstream.Read(p)
	if n > 0 {
		s.buf.Write(p[:n])
		s.scan(false)
	}
	if err == io.EOF {
		s.scan(true)
	}
	return n, err
}

func (s *SSEInterceptor) Close() error {
	if !s.flushed {
		s.flushed = true
		s.onClose()
	}
	return s.upstream.Close()
}

func (s *SSEInterceptor) scan(eof bool) {
	ScanSSELines(&s.buf, eof, s.onLine)
}
