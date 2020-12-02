package gdriver

import (
	"os"
	"path"
	"time"

	drive "google.golang.org/api/drive/v3"
)

const mimeFolder = "application/vnd.google-apps.folder"

// FileInfo represents File information for a File or directory
type FileInfo struct {
	file       *drive.File
	parentPath string
}

// Mode returns the file mode bits
func (i *FileInfo) Mode() os.FileMode {
	mode := os.FileMode(0666)
	if i.file.MimeType == mimeFolder {
		mode |= os.ModeDir
	}

	return mode
}

// ModTime returns the modification time
func (i *FileInfo) ModTime() time.Time {
	modifiedTime, _ := time.Parse(time.RFC3339, i.file.ModifiedTime)
	return modifiedTime
}

// CreateTime returns the time when this File was created
func (i *FileInfo) CreateTime() time.Time {
	t, _ := time.Parse(time.RFC3339, i.file.CreatedTime)
	return t
}

// Sys provides underlying data source
func (i *FileInfo) Sys() interface{} {
	return i.file
}

// Name returns the name of the File or directory
func (i *FileInfo) Name() string {
	return path.Join(i.parentPath, sanitizeName(i.file.Name))
}

// ParentPath returns the parent path of the File or directory
func (i *FileInfo) ParentPath() string {
	return i.parentPath
}

// Path returns the full path to this File or directory
func (i *FileInfo) Path() string {
	return path.Join(i.parentPath, sanitizeName(i.file.Name))
}

// Size returns the bytes for this File
func (i *FileInfo) Size() int64 {
	return i.file.Size
}

// IsDir returns true if this File is a directory
func (i *FileInfo) IsDir() bool {
	return i.file.MimeType == mimeTypeFolder
}

// DriveFile returns the underlaying drive.File
func (i *FileInfo) DriveFile() *drive.File {
	return i.file
}

func sanitizeName(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if isPathSeperator(r) || r == '\'' {
			runes[i] = '-'
		}
	}

	return string(runes)
}

func isPathSeperator(r rune) bool {
	return r == '/' || r == '\\'
}
