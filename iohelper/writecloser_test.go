package iohelper

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type bogusFile struct {
	os.File
}

var errCouldNotWrite = errors.New("could not write")

func (f *bogusFile) Write(_ []byte) (n int, err error) {
	return 0, errCouldNotWrite
}

func TestWriteCloser(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("simple", func(t *testing.T) {
		wc := NewBufferedWriteCloser(file, 1024*8)
		_, err := wc.WriteString("hello world !")
		require.NoError(t, err)
		require.NoError(t, wc.Close())
	})

	t.Run("flush fail", func(t *testing.T) {
		wc := NewBufferedWriteCloser(&bogusFile{File: *file}, 1024*8)
		_, err := wc.WriteString("hello world !")
		require.NoError(t, err)
		require.EqualError(t, wc.Close(), "couldn't flush underlying stream: could not write")
	})
}
