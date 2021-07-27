package iohelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

type TargetWriter struct {
	hash.Hash
}

func NewTargetWriter() *TargetWriter {
	return &TargetWriter{
		Hash: sha256.New(),
	}
}

func (w *TargetWriter) Close() error {
	return nil
}

func (w *TargetWriter) Write(p []byte) (int, error) {
	log.Printf("target.Write: %x\n", p)

	return w.Hash.Write(p)
}

func (w *TargetWriter) String() string {
	return hex.EncodeToString(w.Hash.Sum(nil))
}

type EmptyWriter struct {
	written int64
}

func (w *EmptyWriter) Write(b []byte) (int, error) {
	w.written += int64(len(b))

	return len(b), nil
}

func (w *EmptyWriter) Close() error {
	return nil
}

var inputs = []string{
	"abc",
	"def",
	"this is a source string",
	"it's something",
	".",
	"",
	"Hello !",
}

func testWriterBuf(t *testing.T, buf io.WriteCloser) {
	for _, i := range inputs {
		_, err := io.Copy(buf, bytes.NewReader([]byte(i)))
		require.NoError(t, err)
	}

	require.NoError(t, buf.Close())
}

func createBuffer(size int) []byte {
	buffer := make([]byte, size)
	for i := 0; i < size; i++ {
		buffer[i] = byte(i % 256)
	}

	return buffer
}

func benchWriterBuf(b *testing.B, dst io.WriteCloser, size int) {
	b.ReportAllocs()

	buf := createBuffer(size)

	b.SetBytes(int64(size) * int64(b.N))

	for n := 0; n < b.N; n++ {
		_, err := io.Copy(dst, bytes.NewReader(buf))
		require.NoError(b, err)
	}

	require.NoError(b, dst.Close())
}

func BenchmarkWriterBuf(b *testing.B) {
	buf := NewAsyncWriterBuffer(&EmptyWriter{}, 4096)
	benchWriterBuf(b, buf, 2048)
}

func BenchmarkWriterChan(b *testing.B) {
	buf := NewAsyncWriterChannel(&EmptyWriter{}, 4096)
	benchWriterBuf(b, buf, 2048)
}

func BenchmarkSimpleBuf(b *testing.B) {
	buf := NewBufferedWriteCloser(&EmptyWriter{}, 4096)
	benchWriterBuf(b, buf, 2048)
}

func TestWriterBuf(t *testing.T) {
	refHash := ""
	{
		dst := NewTargetWriter()
		testWriterBuf(t, dst)
		refHash = dst.String()
	}

	t.Run("writerBuf", func(t *testing.T) {
		dst := NewTargetWriter()
		buf := NewAsyncWriterBuffer(dst, 4)
		testWriterBuf(t, buf)
		require.Equal(t, refHash, dst.String())
	})

	t.Run("writerChan", func(t *testing.T) {
		dst := NewTargetWriter()
		buf := NewAsyncWriterChannel(dst, 4)
		testWriterBuf(t, buf)
		require.Equal(t, refHash, dst.String())
	})
}
