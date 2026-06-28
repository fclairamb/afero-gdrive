package gdrive

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

// ErrReadAndWriteNotSupported is returned when the O_RDWR flag is passed
var ErrReadAndWriteNotSupported = errors.New("option O_RDWR is not supported")

// ErrReadOnly means a write operation was performed on a file opened in read-only
var ErrReadOnly = errors.New("we're in a read-only mode")

// ErrWriteOnly means a write operation was performed on a file opened in write-only
var ErrWriteOnly = errors.New("we're in write-only mode")

// ErrUnknownBufferType is returned when a un unknown buffer is specified
var ErrUnknownBufferType = errors.New("unknown buffer type")

// ErrEmptyPath is returned when an empty path is sent
var ErrEmptyPath = errors.New("path cannot be empty")

// ErrForbiddenOnRoot is returned when an operation is performed on the root node
var ErrForbiddenOnRoot = errors.New("forbidden for root directory")

// errInternalNil is an internal error and it should never be reported
var errInternalNil = errors.New("internal nil error")

// FileNotExistError will be thrown if a File was not found
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
	return fmt.Sprintf("\"%s\" already exists", e.Path)
}

// IsNotExist returns true if the error is an FileNotExistError
func IsNotExist(e error) bool {
	var fileNotExistError *FileNotExistError

	return errors.As(e, &fileNotExistError)
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
	Fi   *FileInfo
	Path string
}

func (e FileIsNotDirectoryError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("file %s is not a directory", e.Fi.file.Name)
	}

	return fmt.Sprintf("file %s is not a directory", e.Path)
}

// FileHasMultipleEntriesError will be returned when the same file name is present multiple times
// in the same directory.
type FileHasMultipleEntriesError struct {
	Path string
}

func (e FileHasMultipleEntriesError) Error() string {
	return fmt.Sprintf("multiple entries found for `%s'", e.Path)
}

// NoFileInformationError is returned when a given directory didn't provide any file info.
// This error is bit confusing and needs reviewing.
type NoFileInformationError struct {
	Fi   *FileInfo
	Path string
}

func (e NoFileInformationError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("no file information present in %s : \"%s\"", e.Fi.file.Id, e.Fi.file.Name)
	}

	return fmt.Sprintf("no file information present in path \"%s\"", e.Path)
}

// DriveAPICallError wraps an error that was returned by the Google Drive API
type DriveAPICallError struct {
	Err error
}

func (e *DriveAPICallError) Error() string {
	return fmt.Sprintf("problem calling the drive API: %v", e.Err)
}

// Unwrap exposes the Google Drive API returned error
func (e *DriveAPICallError) Unwrap() error {
	return e.Err
}

// DriveStreamError wraps an error that happened while using a stream opened from the Google Drive API
type DriveStreamError struct {
	Err error
}

func (e *DriveStreamError) Error() string {
	return fmt.Sprintf("problem with drive stream: %v", e.Err)
}

// Unwrap exposes the underlying stream error
func (e *DriveStreamError) Unwrap() error {
	return e.Err
}
