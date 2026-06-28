package gdrive

import (
	"bytes"
	"fmt"
	"sync/atomic"

	log "github.com/fclairamb/go-log"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/fclairamb/afero-gdrive/cache"
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

func (a *APIWrapper) calling(apiName string) {
	atomic.AddInt32(a.calls[apiName], 1)
}

// TotalNbCalls returns the total number of calls performed to the API
func (a *APIWrapper) TotalNbCalls() int {
	nb := int32(0)
	for _, c := range a.calls {
		nb += *c
	}

	return int(nb)
}

// createFile wraps a call to the Files.Create
func (a *APIWrapper) createFile(
	folderID string,
	fileName string,
	mimeType string,
	fields ...googleapi.Field,
) (*drive.File, error) {
	a.calling("Files.Create")

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
	} else {
		err = &DriveAPICallError{Err: err}
	}

	return file, err
}

//nolint:unused
func (a *APIWrapper) renameFile(file *drive.File, targetFolder *drive.File, targetName string) error {
	a.calling("Files.Update")

	call := a.srv.Files.Update(
		file.Id,
		&drive.File{
			Name: sanitizeName(targetName),
		},
	)

	if file.Parents[0] != targetFolder.Id {
		call = call.
			RemoveParents(file.Parents[0]).
			AddParents(targetFolder.Id)
	}

	_, err := call.Do()
	if err != nil {
		return &DriveAPICallError{Err: err}
	}

	// Removing cache of source and target folders
	a.cache.CleanupByPrefix(fmt.Sprintf("%s-", file.Parents[0]))
	a.cache.CleanupByPrefix(fmt.Sprintf("%s-", targetFolder.Id))

	return nil
}

// deleteFile wraps a call to Files.Update or Files.Delete
// To keep it simple and yet true, when a folder is deleted the entire cache is trashed
func (a *APIWrapper) deleteFile(file *drive.File, trash bool) error {
	var err error

	if trash {
		a.calling("Files.Update")
		_, err = a.srv.Files.Update(file.Id, &drive.File{Trashed: true}).Do()
	} else {
		a.calling("Files.Delete")
		err = a.srv.Files.Delete(file.Id).Do()
	}

	if err != nil {
		return &DriveAPICallError{Err: err}
	}

	if file.MimeType == mimeTypeFolder {
		a.cache.CleanupEverything()
	} else {
		for _, p := range file.Parents {
			a.cache.CleanupByPrefix(p)
		}
	}

	return nil
}

func (a *APIWrapper) getFileByFolderAndName(
	folderID string,
	fileName string,
	fields ...googleapi.Field,
) (*drive.FileList, error) {
	queryFields := googleapi.CombineFields(fields)
	if queryFields == "" {
		queryFields = "files(id,mimeType,parents)"
	}

	cacheKey := fmt.Sprintf("%s-getFileByFolderAndName-%s-%s", folderID, fileName, queryFields)
	value, ok := a.cache.Get(cacheKey)

	if ok {
		fileList, _ := value.(*drive.FileList)

		return fileList, nil
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
	a.calling("Files.List")

	query := fmt.Sprintf("'%s' in parents and name='%s' and trashed = false", folderID, sanitizeName(fileName))
	call := a.srv.Files.List().Q(query).Fields(fields)

	return call.Do()
}
