package iohelper // nolint: golint

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

const writeBufferSize = 1024 * 32 // writeBufferSize defines the buffer used for sending write data

// ErrClosed is returned if the buffer was closed
var ErrClosed = errors.New("closed")

// AsyncWriterBuffer is an asynchronous writer that will push writes to a buffer and then writes them in separate
// goroutine.
type AsyncWriterBuffer struct {
	dstWriter     io.WriteCloser // dstWriter is the target writer
	buffer        *bytes.Buffer  // buffer is the shared buffer
	bufferMaxSize int            // bufferMaxSize is the buffer max size
	bufferMu      sync.RWMutex   // bufferMu is the buffer mutex
	bufferRead    *sync.Cond     // bufferRead allows to block until a read is made
	bufferWrite   *sync.Cond     // bufferWrite allows to block until a write is made
	closed        bool           // closed is set if the current stream has been closed
	writeErr      chan error     // writeErr is set when a write fails
	closeErr      chan error     // closeErr is used for the final / closed status
}

// NewAsyncWriterBuffer creates a new asynchronous buffered writer based on a standard buffer
func NewAsyncWriterBuffer(writer io.WriteCloser, maxSize int) *AsyncWriterBuffer {
	aw := &AsyncWriterBuffer{
		dstWriter:     writer,
		buffer:        bytes.NewBuffer(make([]byte, 0, maxSize)),
		writeErr:      make(chan error, 1),
		closeErr:      make(chan error),
		bufferMaxSize: maxSize,
	}
	aw.bufferRead = sync.NewCond(&aw.bufferMu)
	aw.bufferWrite = sync.NewCond(&aw.bufferMu)

	go aw.run()

	return aw
}

func (aw *AsyncWriterBuffer) Write(data []byte) (int, error) {
	aw.bufferMu.Lock()
	defer aw.bufferMu.Unlock()

	if aw.closed {
		return 0, ErrClosed
	}

	// If an error was queued, we'll return it. That means the write returns an error that is not linked
	// to what was just written.
	if len(aw.writeErr) > 0 {
		return 0, <-aw.writeErr
	}

	written := 0

	for !aw.closed && written < len(data) {
		available := aw.bufferMaxSize - aw.buffer.Len()

		// If there's no space available, it means the buffer must be read first
		if available == 0 {
			aw.bufferRead.Wait()
			continue
		}

		writedata := data[written:]

		if len(writedata) > available {
			writedata = writedata[:available]
		}

		n, err := aw.buffer.Write(writedata)
		aw.bufferWrite.Signal()
		written += n

		if err != nil {
			return written, err
		}
	}

	return written, nil
}

func (aw *AsyncWriterBuffer) nextRead(buffer []byte) (int, error) {
	aw.bufferMu.Lock()
	defer aw.bufferMu.Unlock()

	for !aw.closed && aw.buffer.Len() == 0 {
		aw.bufferWrite.Wait()
	}

	defer aw.bufferRead.Signal()
	return aw.buffer.Read(buffer)
}

func (aw *AsyncWriterBuffer) run() {
	buffer := make([]byte, writeBufferSize)

	for {
		n, err := aw.nextRead(buffer)
		b := buffer[:n]

		if err != nil {
			break
		}

		for len(b) > 0 {
			// log.Printf("dst.Write: %x", b)
			n, err := aw.dstWriter.Write(b[0:n])

			if err != nil && len(aw.writeErr) == 0 {
				aw.writeErr <- err
			}

			b = b[n:]
		}
	}

	aw.closeErr <- aw.dstWriter.Close()
}

func (aw *AsyncWriterBuffer) closeAsync() error {
	aw.bufferMu.Lock()
	defer aw.bufferMu.Unlock()

	if aw.closed {
		return ErrClosed
	}

	aw.closed = true
	aw.bufferWrite.Broadcast()
	aw.bufferRead.Broadcast()

	return nil
}

// Close flushes the buffer and closes the underlying writer
func (aw *AsyncWriterBuffer) Close() error {
	if err := aw.closeAsync(); err != nil {
		return err
	}

	<-aw.closeErr
	return nil
}
