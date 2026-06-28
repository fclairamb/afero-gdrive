// Package iohelper should brings tools to help manage IOs
package iohelper

import (
	"bufio"
	"fmt"
	"io"
)

// BufferedWriteCloser brings a io.Closer to the bufio.Writer
type BufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}

// NewBufferedWriteCloser will create a buffered WriteCloser instance from a WriteCloser
func NewBufferedWriteCloser(dst io.WriteCloser, size int) *BufferedWriteCloser {
	wc := &BufferedWriteCloser{
		Writer: bufio.NewWriterSize(dst, size),
		Closer: dst,
	}

	return wc
}

// Close will close the underlying stream
func (wc *BufferedWriteCloser) Close() error {
	if err := wc.Flush(); err != nil {
		return fmt.Errorf("couldn't flush underlying stream: %w", err)
	}

	return wc.Closer.Close()
}
