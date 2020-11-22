package gdriver

import (
	"errors"
	"github.com/spf13/afero"
	"io"
	"os"
	"sync"
)

/*
type File interface {
	Info() *FileInfo
	Write([]byte) (int, error)
	Read([]byte) (int, error)
	Close() error
}
*/

func (f *File) AsAfero() afero.File {
	return f
}

type File struct {
	Driver *GDriver
	*FileInfo
	write    bool
	reader   io.ReadCloser
	once     sync.Once
	Path     string
	writer   *io.PipeWriter
	mu       sync.Mutex
	doneChan chan struct{}
	putError error
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	panic("implement me")
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if _, err := f.Seek(off, 0); err != nil {
		return 0, err
	}
	return f.Read(p)
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	return f.Driver.ListDirectory(f.Path, count)
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
	return errors.New("not implemented")
}

func (f *File) Info() *FileInfo {
	return f.FileInfo
}

func (f *File) getReader() error {
	var lastErr error
	f.once.Do(func() {
		response, err := f.Driver.srv.Files.Get(f.file.Id).Download()
		if err != nil {
			lastErr = err
			return
		}
		f.reader = response.Body
	})
	return lastErr
}

func (f *File) Read(p []byte) (int, error) {
	if f.write {
		return 0, errors.New("open the File with O_RONLY for writing")
	}
	if err := f.getReader(); err != nil {
		return 0, err
	}
	return f.reader.Read(p)
}

func (f *File) getWriter() error {
	f.mu.Lock()
	if f.doneChan == nil {
		var reader io.Reader
		// open a pipe and use the writer part for Write()
		reader, f.writer = io.Pipe()
		// the channel is used to notify the Close() or Write() function if something goes wrong
		f.doneChan = make(chan struct{})
		go func() {
			if f.FileInfo == nil {
				f.FileInfo, f.putError = f.Driver.PutFile(f.Path, reader)
			} else {
				f.putError = f.Driver.updateFileContents(f.FileInfo.file.Id, reader)
			}
			f.doneChan <- struct{}{}
		}()
	}
	err := f.putError
	f.mu.Unlock()
	return err
}

func (f *File) Write(p []byte) (int, error) {
	if !f.write {
		return 0, errors.New("open the File with O_WRONLY for writing")
	}
	if err := f.getWriter(); err != nil {
		return 0, err
	}
	return f.writer.Write(p)
}

func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	panic("implement me")
}

func (f *File) WriteString(s string) (ret int, err error) {
	return f.Write([]byte(s))
}

func (f *File) Close() error {
	if f.write {
		closeErr := f.writer.Close()
		if f.doneChan != nil {
			<-f.doneChan
			if err := f.putError; err != nil {
				return err
			}
		}
		return closeErr
	} else {
		if err := f.getReader(); err != nil {
			return err
		}
		return f.reader.Close()
	}
}

func (f *File) Stat() (os.FileInfo, error) {
	return f.FileInfo, nil
}

func (f *File) Sync() error {
	return nil
}
