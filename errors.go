package gdriver

import (
	"errors"
	"fmt"
)

// ErrNotImplemented is returned when this operation is not (yet) implemented
var ErrNotImplemented = errors.New("not implemented")

// ErrNotSupported is returned when this operations is not supported by Google Drive
var ErrNotSupported = errors.New("google drive doesn't support this operation")

// ErrInvalidSeek is returned when the seek operation is not doable
var ErrInvalidSeek = errors.New("invalid seek offset")

var ErrReadAndWriteNotSupported = errors.New("O_RDWR mode is not supported")

var ErrEmptyPath = errors.New("empty path")

var ErrForbiddenOnRoot = errors.New("forbidden root directory")

var InternalNilError = errors.New("internal nil error")

// FileNotExistError will be thrown if an File was not found
type FileNotExistError struct {
	Path string
}

func (e FileNotExistError) Error() string {
	return fmt.Sprintf("`%s' does not exist", e.Path)
}

// FileExistError will be thrown if an File exists
type FileExistError struct {
	Path string
}

func (e FileExistError) Error() string {
	return fmt.Sprintf("`%s' already exists", e.Path)
}

// IsNotExist returns true if the error is an FileNotExistError
func IsNotExist(e error) bool {
	_, ok := e.(FileNotExistError)
	return ok
}

// IsExist returns true if the error is an FileExistError
func IsExist(e error) bool {
	_, ok := e.(FileExistError)
	return ok
}

// FileIsDirectoryError will be thrown if a File is a directory
type FileIsDirectoryError struct {
	Path string
}

func (e FileIsDirectoryError) Error() string {
	return fmt.Sprintf("`%s' is a directory", e.Path)
}

// FileIsNotDirectoryError will be thrown if a File is not a directory
type FileIsNotDirectoryError struct {
	fi *FileInfo
}

func (e FileIsNotDirectoryError) Error() string {
	return "file is not a directory"
}

type FileHasMultipleEntriesError struct {
	Path string
}

func (e FileHasMultipleEntriesError) Error() string {
	return fmt.Sprintf("multiple entries found for `%s'", e.Path)
}
