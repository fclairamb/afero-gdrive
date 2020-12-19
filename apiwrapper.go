package gdrive // nolint: golint

import (
	"bytes"
	"fmt"
	"sync/atomic"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/fclairamb/afero-gdrive/cache"
	"github.com/fclairamb/afero-gdrive/log"
)

// APIWrapper allows to wrap some GDrive API calls to perform some caching
type APIWrapper struct {
	UseCache bool
	srv      *drive.Service
	cache    *cache.Cache
	logger   log.Logger
	calls    map[string]*int32
}

// NewAPIWrapper instantiates a new APIWrapper
func NewAPIWrapper(srv *drive.Service, logger log.Logger) *APIWrapper {
	return &APIWrapper{
		srv:    srv,
		cache:  cache.NewCache(),
		logger: logger,
		calls: map[string]*int32{
			"Files.Create": new(int32),
			"Files.Update": new(int32),
			"Files.Delete": new(int32),
			"Files.List":   new(int32),
		},
		UseCache: true,
	}
}

func (a *APIWrapper) called(apiName string) {
	atomic.AddInt32(a.calls[apiName], 1)
}

// createFile wraps a call to the Files.Create
func (a *APIWrapper) createFile(
	folderID string,
	fileName string,
	mimeType string,
	fields ...googleapi.Field,
) (*drive.File, error) {
	defer a.called("Files.Create")

	call := a.srv.Files.Create(&drive.File{
		Name:        sanitizeName(fileName),
		MimeType:    mimeType,
		Description: "Created by https://github.com/fclairamb/afero-gdrive",
		Parents: []string{
			folderID,
		},
	}).Fields(fields...)

	if mimeType != mimeTypeFolder {
		call.Media(bytes.NewReader([]byte{}))
	}

	file, err := call.Do()

	if err == nil {
		a.cache.CleanupByPrefix(fmt.Sprintf("%s-", folderID))
	}

	return file, err
}

// deleteFile wraps a call to Files.Update or Files.Delete
// To keep it simple and yet true, when a folder is deleted the entire cache is trashed
func (a *APIWrapper) deleteFile(file *drive.File, trash bool) error {
	var err error

	if trash {
		defer a.called("Files.Update")
		_, err = a.srv.Files.Update(file.Id, &drive.File{Trashed: true}).Do()
	} else {
		defer a.called("Files.Delete")
		err = a.srv.Files.Delete(file.Id).Do()
	}

	if err == nil {
		if file.MimeType == mimeTypeFolder {
			a.cache.CleanupEverything()
		} else {
			for _, p := range file.Parents {
				a.cache.CleanupByPrefix(p)
			}
		}
	}

	return err
}

func (a *APIWrapper) getFileByFolderAndName(
	folderID string,
	fileName string,
	fields ...googleapi.Field,
) (*drive.FileList, error) {
	queryFields := googleapi.CombineFields(fields)
	if queryFields == "" {
		queryFields = "files(id,mimeType)"
	}

	cacheKey := fmt.Sprintf("%s-getFileByFolderAndName-%s-%s", folderID, fileName, queryFields)
	value, ok := a.cache.Get(cacheKey)

	if ok {
		return value.(*drive.FileList), nil
	}

	fileList, err := a._getFileByFolderAndName(folderID, fileName, googleapi.Field(queryFields))

	if err == nil && a.UseCache {
		a.cache.Set(cacheKey, fileList)
	}

	return fileList, err
}

func (a *APIWrapper) _getFileByFolderAndName(
	folderID string,
	fileName string,
	fields googleapi.Field,
) (*drive.FileList, error) {
	defer a.called("Files.List")

	query := fmt.Sprintf("'%s' in parents and name='%s' and trashed = false", folderID, sanitizeName(fileName))
	call := a.srv.Files.List().Q(query).Fields(fields)

	return call.Do()
}
