package gdriver

import (
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"log"
	"os"
)

func (f *File) AsAfero() afero.File {
	return f
}

type File struct {
	Driver *GDriver
	*FileInfo
	Path           string
	streamRead     io.ReadCloser
	streamWrite    io.WriteCloser
	streamWriteEnd chan error
	streamOffset   int64
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	// Write seek is not supported, I'm not 100% sure it can be implemented
	if f.streamWrite != nil {
		return 0, ErrNotImplemented
	}

	// Read seek has its own implementation
	if f.streamRead != nil {
		return f.seekRead(offset, whence)
	}

	// Not having a stream
	return 0, afero.ErrFileClosed
}

func (f *File) seekRead(offset int64, whence int) (int64, error) {
	startByte := int64(0)

	switch whence {
	case io.SeekStart:
		startByte = offset
	case io.SeekCurrent:
		startByte = f.streamOffset + offset
	case io.SeekEnd:
		startByte = f.FileInfo.Size() - offset
	}

	if err := f.streamRead.Close(); err != nil {
		return 0, fmt.Errorf("couldn't close previous stream: %v", err)
	}
	f.streamRead = nil

	if startByte < 0 {
		return startByte, ErrInvalidSeek
	}

	var err error

	f.streamRead, err = f.Driver.getFileReader(f.FileInfo, startByte)

	return startByte, err
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if _, err := f.Seek(off, 0); err != nil {
		return 0, err
	}
	return f.Read(p)
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	return f.Driver.listDirectory(f.FileInfo, count)
}

func (f *File) Readdirnames(n int) ([]string, error) {
	names := make([]string, n, 0)
	dirs, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		names = append(names, d.Name())
	}
	return names, nil
}

func (f *File) Truncate(int64) error {
	return ErrNotSupported
}

func (f *File) Read(p []byte) (int, error) {
	if f.streamWrite != nil {
		return 0, errors.New("open the File with O_RONLY for writing")
	}
	n, err := f.streamRead.Read(p)
	f.streamOffset += int64(n)
	return n, err
}

func (f *File) Write(p []byte) (int, error) {
	if f.streamRead != nil {
		return 0, errors.New("open the File with O_WRONLY for writing")
	}
	n, err := f.streamWrite.Write(p)
	f.streamOffset += int64(n)
	return n, err
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	if _, err := f.Seek(off, 0); err != nil {
		return 0, err
	}
	return f.Write(p)
}

func (f *File) WriteString(s string) (ret int, err error) {
	return f.Write([]byte(s))
}

func (f *File) Close() error {
	if f.streamWrite != nil {
		err := f.streamWrite.Close()
		if err != nil {
			log.Println("Closing issue: ", err)
		}
		closeErr := <-f.streamWriteEnd
		f.streamWrite = nil
		f.streamWriteEnd = nil
		return closeErr
	} else if f.streamRead != nil {
		err := f.streamRead.Close()
		f.streamRead = nil
		return err
	}
	return nil
}

func (f *File) Stat() (os.FileInfo, error) {
	return f.FileInfo, nil
}

func (f *File) Sync() error {
	return nil
}
