package gdrive // nolint: golint

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/afero"
)

// AsAfero provides a cast to afero interface for easier testing
func (f *File) AsAfero() afero.File {
	return f
}

// File represents the managed file structure
type File struct {
	*FileInfo                     // FileInfo contains the core fileInfo
	Path           string         // Path is the complete path of hte file
	driver         *GDriver       // driver is a reference to the parent driver
	streamRead     io.ReadCloser  // streamRead is the underlying reading stream
	streamWrite    io.WriteCloser // streamWrite is the underlying writing stream
	streamWriteEnd chan error     // streamWriteEnd is a channel returning the error of the underlying write stream
	streamOffset   int64          // streamOffset is the position of the stream
	dirListToken   string         // dirListToken contains the token used to list files
}

// Seek sets the offset for the next Read or Write to offset
func (f *File) Seek(offset int64, whence int) (int64, error) {
	// Write seek is not supported by the google drive API.
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
		return 0, fmt.Errorf("couldn't close previous stream: %w", err)
	}

	f.streamRead = nil

	if startByte < 0 {
		return startByte, ErrInvalidSeek
	}

	var err error

	f.streamRead, err = f.driver.getFileReader(f.FileInfo, startByte)

	return startByte, err
}

// ReadAt reads a file at a specific offset
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if _, err := f.Seek(off, 0); err != nil {
		return 0, err
	}

	return f.Read(p)
}

// Readdir provides a list of file information
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	return f.driver.listDirectory(f, count)
}

// Readdirnames provides a list of directory names
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

// Truncate should truncate a file to a specific size. But this method is not supported by
// the google drive API.
func (f *File) Truncate(int64) error {
	return ErrNotSupported
}

func (f *File) Read(p []byte) (int, error) {
	if f.streamWrite != nil {
		return 0, ErrWriteOnly
	}

	n, err := f.streamRead.Read(p)
	f.streamOffset += int64(n)

	if err != nil && !errors.Is(err, io.EOF) {
		err = &DriveStreamError{Err: err}
	}

	return n, err
}

func (f *File) Write(p []byte) (int, error) {
	if f.streamRead != nil {
		return 0, ErrReadOnly
	}

	n, err := f.streamWrite.Write(p)
	f.streamOffset += int64(n)

	if err != nil && !errors.Is(err, io.EOF) {
		err = &DriveStreamError{Err: err}
	}

	return n, err
}

// WriteAt writes some bytes at a specified offset
func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	if _, err := f.Seek(off, 0); err != nil {
		return 0, err
	}

	return f.Write(p)
}

// WriteString writes a string
func (f *File) WriteString(s string) (ret int, err error) {
	return io.WriteString(f, s) //nolint: gocritic
}

// Close closes the file
// This marks the end of the file write.
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
		if err != nil && !errors.Is(err, io.EOF) {
			err = &DriveStreamError{Err: err}
		}

		return err
	}

	return nil
}

// Stat provides stat file information
func (f *File) Stat() (os.FileInfo, error) {
	return f.FileInfo, nil
}

// Sync forces a file synchronization. This has no effect here.
func (f *File) Sync() error {
	return nil
}
