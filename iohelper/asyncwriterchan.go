package iohelper // nolint: golint

import (
	"io"
	"sync"
)

const maxBuffersOnChannel = 2000

// AsyncWriterChannel is an asynchronous writer that will push writes to a channel and then write them in a separate
// goroutine.
type AsyncWriterChannel struct {
	dstWriter      io.WriteCloser // final output
	writeChan      chan []byte    // channel used to store buffers that will be transmitted
	writeErr       chan error     // channel used to store write errors
	writeEnd       chan bool      // channel to wait for the end of the last write
	maxSize        int64          // approximate max size of this buffer
	bufferSize     int64          // current size of the data being stored
	bufferSizeMu   sync.Mutex
	bufferSizeHigh *sync.Cond
	closed         bool
}

// NewAsyncWriterChannel creates an asynchronous buffered writer based on a channel
func NewAsyncWriterChannel(writer io.WriteCloser, bufferSize int) *AsyncWriterChannel {
	aw := &AsyncWriterChannel{
		dstWriter: writer,
		writeChan: make(chan []byte, maxBuffersOnChannel),
		writeErr:  make(chan error, 1),
		writeEnd:  make(chan bool),
		maxSize:   int64(bufferSize),
	}

	aw.bufferSizeHigh = sync.NewCond(&aw.bufferSizeMu)

	go aw.run()

	return aw
}

func (aw *AsyncWriterChannel) addToChan(buf []byte) {
	aw.bufferSizeMu.Lock()
	defer aw.bufferSizeMu.Unlock()

	for !aw.closed && aw.bufferSize > aw.maxSize {
		aw.bufferSizeHigh.Wait()
	}

	aw.bufferSize += int64(len(buf))
	aw.writeChan <- buf
}

func (aw *AsyncWriterChannel) Write(src []byte) (int, error) {
	if len(aw.writeErr) > 0 {
		return 0, <-aw.writeErr
	}

	dst := make([]byte, len(src))
	copy(dst, src)

	aw.addToChan(dst)

	return len(src), nil
}

func (aw *AsyncWriterChannel) run() {
	defer func() {
		aw.writeEnd <- true
	}()

	for buf := range aw.writeChan {
		var n int
		var err error

		aw.bufferSizeMu.Lock()
		aw.bufferSize -= int64(len(buf))
		aw.bufferSizeHigh.Signal()
		aw.bufferSizeMu.Unlock()

		for {
			n, err = aw.dstWriter.Write(buf)

			if err != nil {
				aw.writeErr <- err
				return
			}

			if n < len(buf) {
				buf = buf[n:]
			} else {
				break
			}
		}
	}
}

// Close flushes the buffer and closes the underlying writer
func (aw *AsyncWriterChannel) Close() error {
	close(aw.writeChan)

	<-aw.writeEnd

	return aw.dstWriter.Close()
}
