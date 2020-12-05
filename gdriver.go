// Package gdriver provides an afero Fs interface to Google Drive API
package gdriver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fclairamb/afero-gdrive/log"
	"github.com/spf13/afero"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// GDriver can be used to access google drive in a traditional File-folder-path pattern
type GDriver struct {
	srv                 *drive.Service
	rootNode            *FileInfo
	Logger              log.Logger
	LogReaderAndWriters bool
	TrashForDelete      bool
}

// HashMethod is the hashing method to use for GetFileHash
type HashMethod int

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
	mimeTypeFile   = "application/octet-stream"
)

var (
	fileInfoFields = []googleapi.Field{
		"createdTime",
		"id",
		"mimeType",
		"modifiedTime",
		"name",
		"size",
	}
	listFields     []googleapi.Field
	sharedInitOnce sync.Once
)

func sharedInit() {
	listFields = []googleapi.Field{
		googleapi.Field(fmt.Sprintf("files(%s)", googleapi.CombineFields(fileInfoFields))),
	}
}

// New creates a new Google Drive driver, client must me an authenticated instance for google drive
func New(client *http.Client, opts ...Option) (*GDriver, error) {
	sharedInitOnce.Do(sharedInit)

	driver := &GDriver{
		Logger: log.Nothing(),
	}

	var err error

	driver.srv, err = drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive client: %w", err)
	}

	if _, err = driver.SetRootDirectory(""); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if err = opt(driver); err != nil {
			return nil, err
		}
	}

	return driver, nil
}

// Name provides the name of this filesystem
func (d *GDriver) Name() string {
	return "gdrive"
}

// AsAfero provides a cast to afero interface for easier testing
func (d *GDriver) AsAfero() afero.Fs {
	return d
}

// SetRootDirectory changes the working root directory
// use this if you want to do certain operations in a special directory
// path should always be the absolute real path
func (d *GDriver) SetRootDirectory(path string) (*FileInfo, error) {
	rootNode, err := getRootNode(d.srv)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Drive root: %w", err)
	}

	file, err := d.getFileOnRootNode(rootNode, path, listFields...)
	if err != nil {
		return nil, err
	}

	if !file.IsDir() {
		return nil, FileIsNotDirectoryError{Fi: file}
	}

	d.rootNode = file

	return file, nil
}

// Stat gives a FileInfo for a File or directory
func (d *GDriver) Stat(path string) (os.FileInfo, error) {
	return d.getFile(path, listFields...)
}

func (d *GDriver) listDirectory(fi *FileInfo, count int) ([]os.FileInfo, error) {
	if !fi.IsDir() {
		return nil, FileIsNotDirectoryError{Fi: fi}
	}

	pageToken := ""
	files := make([]os.FileInfo, 0)

	for count < 0 || len(files) < count {
		call := d.srv.Files.List().
			Q(fmt.Sprintf("'%s' in parents and trashed = false", fi.file.Id)).
			Fields(append(listFields, "nextPageToken")...)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		descendants, err := call.Do()
		if err != nil {
			return files, err
		}

		if descendants == nil {
			return nil, &NoFileInformationError{Fi: fi}
		}

		for i := 0; i < len(descendants.Files); i++ {
			files = append(files, &FileInfo{
				file:       descendants.Files[i],
				parentPath: fi.Path(),
			})
		}

		if pageToken = descendants.NextPageToken; pageToken == "" {
			break
		}
	}

	return files, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (d *GDriver) Mkdir(path string, perm os.FileMode) error {
	return d.MkdirAll(path, perm)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (d *GDriver) MkdirAll(path string, _ os.FileMode) error {
	_, err := d.makeDirectoryByParts(strings.FieldsFunc(path, isPathSeperator))
	return err
}

func (d *GDriver) makeDirectoryByParts(pathParts []string) (*FileInfo, error) {
	parentNode := d.rootNode

	for i := 0; i < len(pathParts); i++ {
		query := fmt.Sprintf(
			"'%s' in parents and name='%s' and trashed = false",
			parentNode.file.Id,
			sanitizeName(pathParts[i]),
		)

		files, err := d.srv.Files.List().Q(query).Fields(listFields...).Do()
		if err != nil {
			return nil, err
		}

		if files == nil {
			return nil, &NoFileInformationError{Fi: parentNode, Path: path.Join(pathParts[:i+1]...)}
		}

		switch len(files.Files) {
		case 0:
			{
				// File not found => create directory
				if !parentNode.IsDir() {
					return nil, FileIsNotDirectoryError{
						Fi:   parentNode,
						Path: path.Join(pathParts[:i]...),
					}
				}
				var createdDir *drive.File

				createdDir, err = d.srv.Files.Create(&drive.File{
					Name:     sanitizeName(pathParts[i]),
					MimeType: mimeTypeFolder,
					Parents: []string{
						parentNode.file.Id,
					},
				}).Fields(fileInfoFields...).Do()
				if err != nil {
					return nil, err
				}

				parentNode = &FileInfo{
					file:       createdDir,
					parentPath: path.Join(pathParts[:i]...),
				}
			}
		case 1:
			{
				parentNode = &FileInfo{
					file:       files.Files[0],
					parentPath: path.Join(pathParts[:i]...),
				}
			}
		default:
			{
				return nil, &FileHasMultipleEntriesError{Path: path.Join(pathParts[:i+1]...)}
			}
		}
	}

	return parentNode, nil
}

// DeleteDirectory will delete a directory and its descendants
func (d *GDriver) DeleteDirectory(path string) error {
	file, err := d.getFile(path, "files(id,mimeType)")
	if err != nil {
		return err
	}

	if !file.IsDir() {
		return FileIsNotDirectoryError{Fi: file}
	}

	if file == d.rootNode {
		return ErrForbiddenOnRoot
	}

	return d.deleteFile(file)
}

func (d *GDriver) deleteFile(fi *FileInfo) error {
	var err error

	if d.TrashForDelete {
		_, err = d.srv.Files.Update(fi.file.Id, &drive.File{Trashed: true}).Do()
	} else {
		err = d.srv.Files.Delete(fi.file.Id).Do()
	}

	return err
}

// RemoveAll will delete a File or directory, if directory it will also delete its descendants
func (d *GDriver) RemoveAll(path string) error {
	file, err := d.getFile(path)
	if err != nil {
		return err
	}

	if file == d.rootNode {
		return ErrForbiddenOnRoot
	}

	return d.deleteFile(file)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (d *GDriver) Remove(path string) error {
	return d.RemoveAll(path)
}

func (d *GDriver) getFileReader(fi *FileInfo, offset int64) (io.ReadCloser, error) {
	if fi.IsDir() {
		return nil, FileIsDirectoryError{Path: fi.Path()}
	}

	request := d.srv.Files.Get(fi.file.Id)

	if offset > 0 {
		request.Header().Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	// The resulting stream will be closed by the reader of the file
	// nolint:bodyclose
	response, err := request.Download()
	if err != nil {
		return nil, err
	}

	return response.Body, nil
}

func (d *GDriver) getFileWriter(fi *FileInfo) (io.WriteCloser, chan error, error) {
	if fi == nil {
		return nil, nil, errInternalNil
	}
	// open a pipe and use the writer part for Write()
	reader, writer := io.Pipe()

	endErr := make(chan error)

	// the channel is used to notify the Close() or Write() function if something goes wrong
	go func() {
		if d.LogReaderAndWriters {
			d.Logger.Info("Starting the writer",
				"fileId", fi.file.Id,
				"fileName", fi.file.Name,
			)
		}

		_, err := d.srv.Files.Update(fi.file.Id, nil).Fields(fileInfoFields...).Media(reader).Do()

		endErr <- err

		if d.LogReaderAndWriters {
			d.Logger.Info("Writer stopped",
				"fileId", fi.file.Id,
				"fileName", fi.file.Name,
			)
		}
	}()

	return writer, endErr, nil
}

func (d *GDriver) getFileInfoFromPath(path string) (*FileInfo, error) {
	return d.getFile(path, listFields...)
}

// createFile creates a new file
func (d *GDriver) createFile(filePath string) (*FileInfo, error) {
	pathParts := strings.FieldsFunc(filePath, isPathSeperator)
	amountOfParts := len(pathParts)

	if amountOfParts <= 0 {
		return nil, ErrEmptyPath
	}

	// check if there is already a File
	existentFile, err := d.getFileByParts(d.rootNode, pathParts, listFields...)
	if err != nil {
		if !IsNotExist(err) {
			return nil, err
		}

		existentFile = nil
	}

	if existentFile == d.rootNode {
		return nil, ErrForbiddenOnRoot
	}

	// create a new File
	parentNode := d.rootNode

	if amountOfParts > 1 {
		dir, errMkDir := d.makeDirectoryByParts(pathParts[:amountOfParts-1])
		if errMkDir != nil {
			return nil, errMkDir
		}

		parentNode = dir
		if !parentNode.IsDir() {
			return nil, &FileIsNotDirectoryError{
				Fi:   parentNode,
				Path: path.Join(pathParts[:amountOfParts-1]...),
			}
		}
	}

	file, err := d.srv.Files.Create(
		&drive.File{
			Name:        sanitizeName(pathParts[amountOfParts-1]),
			MimeType:    mimeTypeFile,
			Description: "Created by https://github.com/fclairamb/afero-gdrive",
			Parents: []string{
				parentNode.file.Id,
			},
		},
	).Fields(fileInfoFields...).Media(bytes.NewReader([]byte{})).Do()
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		file:       file,
		parentPath: path.Join(pathParts[:amountOfParts-1]...),
	}, nil
}

// Rename moves a File or directory to a new path
func (d *GDriver) Rename(oldPath, newPath string) error {
	pathParts := strings.FieldsFunc(newPath, isPathSeperator)
	amountOfParts := len(pathParts)

	if amountOfParts <= 0 {
		return ErrEmptyPath
	}

	file, err := d.getFile(oldPath, "files(id,parents)")
	if err != nil {
		return err
	}

	if file == d.rootNode {
		return ErrForbiddenOnRoot
	}

	parentNode := d.rootNode

	if amountOfParts > 1 {
		dir, errMkDir := d.makeDirectoryByParts(pathParts[:amountOfParts-1])
		if errMkDir != nil {
			return errMkDir
		}

		parentNode = dir
		if !parentNode.IsDir() {
			// Was: return fmt.Errorf("unable to create File in `%s': `%s' is not a directory",
			// path.Join(pathParts[:amountOfParts-1]...), parentNode.Name())
			return &FileIsNotDirectoryError{Fi: parentNode}
		}
	}

	_, err = d.srv.Files.Update(file.file.Id, &drive.File{
		Name: sanitizeName(pathParts[amountOfParts-1]),
	}).
		AddParents(parentNode.file.Id).
		RemoveParents(path.Join(file.file.Parents...)).
		Fields(fileInfoFields...).Do()

	return err
}

// Trash trashes a File or directory
func (d *GDriver) trash(fi *FileInfo) error {
	_, err := d.srv.Files.Update(fi.file.Id, &drive.File{
		Trashed: true,
	}).Do()

	return err
}

func (d *GDriver) trashPath(path string) error {
	if path == "" {
		return ErrForbiddenOnRoot
	}

	fi, err := d.getFile(path, "files(id)")
	if err != nil {
		return err
	}

	return d.trash(fi)
}

// ListTrash lists the contents of the trash
// if you specify directories it will only list the trash contents of the specified directories
func (d *GDriver) ListTrash(filePath string, count int) ([]*FileInfo, error) {
	file, err := d.getFile(filePath, "files(id,name)")
	if err != nil {
		return nil, err
	}

	// no directories specified
	files, err := d.srv.Files.List().Q("trashed = true").Fields(
		googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))),
	).Do()
	if err != nil {
		return nil, err
	}

	var list []*FileInfo

	for i := 0; i < len(files.Files); i++ {
		// determinate the parent of this File
		inRoot, parentPath, err := isInRoot(d.srv, file.file.Id, files.Files[i], "")
		if err != nil {
			return nil, err
		}

		if inRoot {
			list = append(
				list,
				&FileInfo{
					file:       files.Files[i],
					parentPath: path.Join(file.Path(), parentPath),
				},
			)
		}
	}

	return list, nil
}

func getRootNode(srv *drive.Service) (*FileInfo, error) {
	root, err := srv.Files.Get("root").Fields(fileInfoFields...).Do()
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		file:       root,
		parentPath: "",
	}, nil
}

// isInRoot checks if a File is a descendant of root, if so it will return the parent path of the File
func isInRoot(srv *drive.Service, rootID string, file *drive.File, basePath string) (bool, string, error) {
	for _, parentID := range file.Parents {
		if parentID == rootID {
			return true, basePath, nil
		}

		parent, err := srv.Files.Get(parentID).Fields("id,name,parents").Do()
		if err != nil {
			return false, "", err
		}

		if inRoot, parentPath, err := isInRoot(srv, rootID, parent, path.Join(parent.Name, basePath)); err != nil || inRoot {
			return inRoot, parentPath, err
		}
	}

	return false, "", nil
}

func (d *GDriver) getFile(path string, fields ...googleapi.Field) (*FileInfo, error) {
	return d.getFileOnRootNode(d.rootNode, path, fields...)
}

func (d *GDriver) getFileOnRootNode(rootNode *FileInfo, path string, fields ...googleapi.Field) (*FileInfo, error) {
	spl := strings.FieldsFunc(path, isPathSeperator)
	return d.getFileByParts(rootNode, spl, fields...)
}

func (d *GDriver) getFileByParts(rootNode *FileInfo, pathParts []string, fields ...googleapi.Field) (*FileInfo, error) {
	amountOfParts := len(pathParts)

	if amountOfParts == 0 {
		// get root directory if we have no parts
		return rootNode, nil
	}

	lastID := rootNode.file.Id
	lastPart := amountOfParts - 1
	var lastFile *drive.File

	for i := 0; i < amountOfParts; i++ {
		query := fmt.Sprintf("'%s' in parents and name='%s' and trashed = false", lastID, sanitizeName(pathParts[i]))
		// Logger.Println("query:" + query)
		call := d.srv.Files.List().Q(query)

		// if we are not at the last part
		if i == lastPart {
			if len(fields) == 0 {
				call = call.Fields("files(id)")
			} else {
				call = call.Fields(fields...)
			}
		} else {
			call = call.Fields("files(id)")
		}

		files, err := call.Do()
		if err != nil {
			return nil, err
		}

		if files == nil || len(files.Files) == 0 {
			return nil, FileNotExistError{Path: path.Join(pathParts[:i+1]...)}
		}

		if len(files.Files) > 1 {
			return nil, &FileHasMultipleEntriesError{Path: path.Join(pathParts[:i+1]...)}
		}

		lastFile = files.Files[0]
		lastID = lastFile.Id
	}

	return &FileInfo{
		file:       lastFile,
		parentPath: path.Join(pathParts[:amountOfParts-1]...),
	}, nil
}

// Open a File for reading.
func (d *GDriver) Open(name string) (afero.File, error) {
	return d.OpenFile(name, os.O_RDONLY, 0)
}

// OpenFile opens a File in the traditional os.Open way
func (d *GDriver) OpenFile(path string, flag int, perm os.FileMode) (afero.File, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}

	if flag&os.O_RDWR != 0 {
		return nil, ErrReadAndWriteNotSupported
	}

	// determinate existent status
	file, err := d.getFileInfoFromPath(path)
	var fileExists bool

	switch {
	case err == nil:
		{
			fileExists = true

			if file.IsDir() {
				return &File{
					driver:   d,
					Path:     path,
					FileInfo: file,
				}, nil
			}
		}
	case IsNotExist(err):
		{
			fileExists = false
		}
	default:
		{
			return nil, err
		}
	}

	// We should try to create the file if we have the right to do so
	if !fileExists {
		if flag&os.O_CREATE != 0 && flag&os.O_WRONLY != 0 {
			file, err = d.createFile(path)
			if err != nil {
				return nil, err
			}

			fileExists = true
		} else {
			return nil, FileNotExistError{Path: path}
		}
	}

	// If we're in write mode
	if flag&os.O_WRONLY != 0 {
		if !fileExists {
			return nil, FileNotExistError{Path: path}
		}

		return d.openFileWrite(file, path)
	}

	return d.openFileRead(file)
}

func (d *GDriver) openFileRead(file *FileInfo) (afero.File, error) {
	reader, errReader := d.getFileReader(file, 0)

	if errReader != nil {
		return nil, errReader
	}

	return &File{
		driver:     d,
		FileInfo:   file,
		streamRead: reader,
	}, nil
}

func (d *GDriver) openFileWrite(file *FileInfo, path string) (afero.File, error) {
	writer, endErr, err := d.getFileWriter(file)
	if err != nil {
		return nil, err
	}

	return &File{
		driver:         d,
		Path:           path,
		FileInfo:       file,
		streamWrite:    writer,
		streamWriteEnd: endErr,
	}, nil
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens.
func (d *GDriver) Create(name string) (afero.File, error) {
	file, err := d.OpenFile(name, os.O_CREATE, 0777)
	if err != nil {
		return nil, err
	}

	if _, errWrite := file.Write([]byte{}); errWrite != nil {
		return nil, err
	}

	return file, nil
}

// Chmod changes the mode of the named file to mode.
func (d *GDriver) Chmod(path string, mode os.FileMode) error {
	fi, err := d.getFile(path)
	if err != nil {
		return err
	}

	_, err = d.srv.Files.Update(fi.file.Id, &drive.File{
		Properties: map[string]string{
			"ftp_file_mode": fmt.Sprintf("%d", mode),
		},
	}).Do()

	return err
}

// Chtimes changes the access and modification times of the named file
func (d *GDriver) Chtimes(path string, atime time.Time, mTime time.Time) error {
	fi, err := d.getFile(path)
	if err != nil {
		return err
	}

	_, err = d.srv.Files.Update(fi.file.Id, &drive.File{
		ViewedByMeTime: atime.Format(time.RFC3339),
		ModifiedTime:   mTime.Format(time.RFC3339),
		// ModifiedByMeTime: mTime.Format(time.RFC3339),
	}).Do()

	return err
}
