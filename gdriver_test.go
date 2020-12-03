// nolint: funlen
package gdriver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fclairamb/afero-gdrive/log/gokit"
	"github.com/fclairamb/afero-gdrive/oauthhelper"
	"github.com/hjson/hjson-go"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

var (
	prefix   string
	initOnce sync.Once
)

func varInit() {
	prefix = time.Now().UTC().Format("20060102_150405.000000")
}

func setup(t *testing.T) *GDriver {
	initOnce.Do(varInit)

	env, err := ioutil.ReadFile(".env.json")
	if err != nil {
		if !os.IsNotExist(err) {
			require.NoError(t, err)
		}
	}

	if len(env) > 0 {
		var environmentVariables map[string]interface{}

		require.NoError(t, hjson.Unmarshal(env, &environmentVariables))

		for key, val := range environmentVariables {
			if s, ok := val.(string); ok {
				require.NoError(t, os.Setenv(key, s))
			} else {
				require.FailNow(t, "unable to set environment", "Key `%s' is not a string was a %T", key, val)
			}
		}
	}

	helper := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Authenticate: func(url string) (string, error) {
			return "", ErrNotSupported
		},
	}
	var client *http.Client
	var driver *GDriver
	var token []byte

	token, err = base64.StdEncoding.DecodeString(os.Getenv("GOOGLE_TOKEN"))
	require.NoError(t, err)

	helper.Token = new(oauth2.Token)

	require.NoError(t, json.Unmarshal(token, helper.Token))

	client, err = helper.NewHTTPClient(context.Background())
	require.NoError(t, err)

	driver, err = New(client)
	require.NoError(t, err)

	driver.Logger = gokit.NewGKLoggerStdout()

	fullPath := sanitizeName(fmt.Sprintf("GDriveTest-%s-%s", t.Name(), prefix))

	err = driver.MkdirAll(fullPath, os.FileMode(700))
	require.NoError(t, err)

	_, err = driver.SetRootDirectory(fullPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err = driver.SetRootDirectory("")
		require.NoError(t, err)
		require.NoError(t, driver.DeleteDirectory(fullPath))
	})

	return driver
}

func TestMakeDirectory(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		driver := setup(t).AsAfero()

		err := driver.MkdirAll("Folder1", os.FileMode(700))
		require.NoError(t, err)

		// Folder1 created?
		fi, err := driver.Stat("Folder1")
		require.NoError(t, err)
		require.Equal(t, "Folder1", fi.Name())
	})

	t.Run("in existing directory", func(t *testing.T) {
		driver := setup(t).AsAfero()

		require.NoError(t, driver.MkdirAll("Folder1", os.FileMode(700)))

		err := driver.MkdirAll("Folder1/Folder2", os.FileMode(700))
		require.NoError(t, err)

		// Folder1/Folder2 created?
		_, err = driver.Stat("Folder1/Folder2")
		require.NoError(t, err)
	})

	t.Run("in non existing directory", func(t *testing.T) {
		driver := setup(t).AsAfero()

		require.NoError(t, driver.MkdirAll("Folder1/Folder2/Folder3", os.FileMode(0)))
		fi, err := driver.Stat("Folder1/Folder2/Folder3")
		require.NoError(t, err)
		require.Equal(t, "Folder1/Folder2/Folder3", fi.Name())

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))

		// Folder1/Folder2/Folder3 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2/Folder3")))
	})

	t.Run("creation of existing directory", func(t *testing.T) {
		driver := setup(t).AsAfero()

		err := driver.MkdirAll("Folder1/Folder2", os.FileMode(0))
		require.NoError(t, err)

		err = driver.MkdirAll("Folder1/Folder2", os.FileMode(0))
		require.NoError(t, err)
	})

	t.Run("create folder as a descendant of a File", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")

		require.EqualError(
			t,
			driver.MkdirAll("Folder1/File1/Folder2", os.FileMode(0)),
			"file Folder1/File1 is not a directory",
		)
	})

	t.Run("make root", func(t *testing.T) {
		driver := setup(t).AsAfero()

		require.NoError(t, driver.Mkdir("", os.FileMode(0)))
	})
}

func TestFileFolderMixup(t *testing.T) {
	driver := setup(t).AsAfero()

	// create File
	require.NoError(t, writeFile(driver, "Folder1/File1", bytes.NewBufferString("Hello World")))

	err := writeFile(driver, "Folder1/File1/File2", bytes.NewBufferString("Hello World"))
	require.EqualError(t, err, "file Folder1/File1 is not a directory")
}

func TestCreateFile(t *testing.T) {
	t.Run("in root folder", func(t *testing.T) {
		driver := setup(t).AsAfero()

		mustWriteFile(t, driver, "File1", "Hello World")

		fi, err := driver.Stat("File1")
		require.NoError(t, err)

		require.Equal(t, "File1", fi.Name())

		// File created?
		fi, err = driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi.Name())

		// Compare File contents
		r, err := driver.Open("File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("in non existing folder", func(t *testing.T) {
		driver := setup(t).AsAfero()

		// create File
		mustWriteFile(t, driver, "Folder1/File1", "Hello World")

		// Folder created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// File created?
		fi, err := driver.Stat("Folder1/File1")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File1", fi.Name())

		// Compare File contents
		r, err := driver.Open("Folder1/File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("as descendant of File", func(t *testing.T) {
		driver := setup(t).AsAfero()

		// create File
		require.NoError(t, writeFile(driver, "Folder1/File1", bytes.NewBufferString("Hello World")))

		err := writeFile(driver, "Folder1/File1/File2", bytes.NewBufferString("Hello World"))
		require.EqualError(t, err, "file Folder1/File1 is not a directory")
	})

	t.Run("empty target", func(t *testing.T) {
		driver := setup(t).AsAfero()

		// create File
		require.EqualError(t, writeFile(driver, "", bytes.NewBufferString("Hello World")), "path cannot be empty")
	})

	t.Run("overwrite File", func(t *testing.T) {
		driver := setup(t).AsAfero()

		// create File
		mustWriteFile(t, driver, "File1", "Hello World")

		// File created?
		fi1, err := driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi1.Name())

		// Compare File contents
		r, err := driver.Open("File1")
		require.NoError(t, err)
		defer func() { require.NoError(t, r.Close()) }()
		received, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello World", string(received))

		// create File
		mustWriteFile(t, driver, "File1", "Hello Universe")

		// File created?
		fi2, err := driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi2.Name())

		// Compare File contents
		r, err = driver.Open("File1")
		defer func() { require.NoError(t, r.Close()) }()
		require.NoError(t, err)
		received, err = ioutil.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "Hello Universe", string(received))
	})
}

func TestGetFile(t *testing.T) {
	driver := setup(t).AsAfero()

	newFile(t, driver, "Folder1/File1")

	// Compare File content
	fi, err := driver.Open("Folder1/File1")
	require.NoError(t, err)
	received, err := ioutil.ReadAll(fi)
	require.NoError(t, err)
	require.Equal(t, "Hello World", string(received))
	require.Equal(t, "Folder1/File1", fi.Name())

	// Get File contents of an Folder
	file, err := driver.Open("Folder1")
	require.NoError(t, err)
	fileInfo, err := file.Stat()
	require.NoError(t, err)
	require.True(t, fileInfo.IsDir())
}

func TestDelete(t *testing.T) {
	t.Run("delete File", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "File1")

		// delete File
		require.NoError(t, driver.Remove("File1"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("File1")), "`File1' does not exist")
	})

	t.Run("delete directory", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.Remove("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestDeleteDirectory(t *testing.T) {
	t.Run("delete directory", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.Remove("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestListDirectory(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")
		newFile(t, driver, "Folder1/File2")

		// var files []*FileInfo
		dir, err := driver.Open("Folder1")
		require.NoError(t, err)

		files, err := dir.Readdir(1000)
		require.NoError(t, err)

		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Name(), files[j].Name()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Name())
		require.Equal(t, "Folder1/File2", files[1].Name())

		// Remove contents
		require.NoError(t, driver.Remove("Folder1/File1"))
		require.NoError(t, driver.Remove("Folder1/File2"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// File2 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File2")), "`Folder1/File2' does not exist")

		// Test if folder is empty
		dir, err = driver.Open("Folder1")
		require.NoError(t, err)

		files, err = dir.Readdir(1000)
		require.NoError(t, err)

		require.Len(t, files, 0)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		driver := setup(t).AsAfero()

		_, err := driver.Open("Folder1")
		require.EqualError(t, err, "`Folder1' does not exist")
	})

	t.Run("list File", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "File1")

		dir, err := driver.Open("File1")
		require.NoError(t, err)

		_, err = dir.Readdir(1000)
		require.EqualError(t, err, "file File1 is not a directory")
	})
}

func TestMove(t *testing.T) {
	t.Run("move into another folder with another name", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")

		// Rename File
		err := driver.Rename("Folder1/File1", "Folder2/File2")
		require.NoError(t, err)

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File2")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into another folder with same name", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")

		// Rename File
		err := driver.Rename("Folder1/File1", "Folder2/File1")
		require.NoError(t, err)

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File1")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into same folder", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")

		// Rename File
		err := driver.Rename("Folder1/File1", "Folder1/File2")
		require.NoError(t, err)

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")
	})

	t.Run("move root", func(t *testing.T) {
		driver := setup(t).AsAfero()

		require.EqualError(t, driver.Rename("", "Folder1"), "forbidden for root directory")
	})

	t.Run("invalid target", func(t *testing.T) {
		driver := setup(t).AsAfero()

		require.EqualError(t, driver.Rename("Folder1", ""), "path cannot be empty")
	})
}

func TestTrash(t *testing.T) {
	t.Run("trash File", func(t *testing.T) {
		var driver afero.Fs
		{
			src := setup(t)
			src.TrashForDelete = true
			driver = src.AsAfero()
		}

		newFile(t, driver, "Folder1/File1")

		// trash File
		require.NoError(t, driver.Remove("Folder1/File1"))

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("trash folder", func(t *testing.T) {
		var driver afero.Fs
		{
			src := setup(t)
			src.TrashForDelete = true
			driver = src.AsAfero()
		}

		newFile(t, driver, "Folder1/File1")

		// trash folder
		require.NoError(t, driver.Remove("Folder1"))

		// Folder1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1' does not exist")
	})

	t.Run("trash root", func(t *testing.T) {
		var driver afero.Fs
		{
			src := setup(t)
			src.TrashForDelete = true
			driver = src.AsAfero()
		}

		require.EqualError(t, driver.Remove(""), "forbidden for root directory")
	})
}

func TestListTrash(t *testing.T) {
	if hostname, _ := os.Hostname(); hostname != "MacBook-Pro-de-Florent.local" {
		t.Skip("Do not execute trash test")
	}

	t.Run("root", func(t *testing.T) {
		driver := setup(t)

		newFile(t, driver, "Folder1/File1")
		newFile(t, driver, "Folder2/File2")
		newFile(t, driver, "Folder3/File3")

		// trash File1
		require.NoError(t, driver.trashPath("Folder1/File1"))
		// trash Folder2
		require.NoError(t, driver.trashPath("Folder2"))

		files, err := driver.ListTrash("", 1000)
		require.NoError(t, err)
		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, fmt.Sprintf("GDriveTest-TestListTrash-root-%s/Folder1/File1", prefix), files[0].Path())
		require.Equal(t, fmt.Sprintf("GDriveTest-TestListTrash-root-%s/Folder2", prefix), files[1].Path())
	})

	t.Run("of folder", func(t *testing.T) {
		driver := setup(t)

		newFile(t, driver, "Folder1/File1")
		newFile(t, driver, "Folder1/File2")
		newFile(t, driver, "Folder2/File3")

		// trash File1 and File2
		require.NoError(t, driver.trashPath("Folder1/File1"))
		require.NoError(t, driver.trashPath("Folder1/File2"))

		var files []*FileInfo
		files, err := driver.ListTrash("Folder1", 1000)
		require.NoError(t, err)
		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Path())
		require.Equal(t, "Folder1/File2", files[1].Path())
	})
}

func TestIsInRoot(t *testing.T) {
	t.Run("in folder", func(t *testing.T) {
		driver := setup(t)

		newFile(t, driver, "Folder1/File1")

		fi, err := driver.getFile(
			"Folder1/File1",
			googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))),
		)
		require.NoError(t, err)

		inRoot, parentPath, err := isInRoot(driver.srv, driver.rootNode.file.Id, fi.file, "")
		require.NoError(t, err)
		require.True(t, inRoot)
		require.Equal(t, "Folder1", parentPath)
	})

	t.Run("not in folder", func(t *testing.T) {
		driver := setup(t).AsAfero()

		newFile(t, driver, "Folder1/File1")
		require.NoError(t, driver.Mkdir("Folder2", os.FileMode(0)))
		_, err := driver.Stat("Folder1/File1")
		require.NoError(t, err)

		_, err = driver.Open("Folder1/File1")
		require.NoError(t, err)
	})
}

func TestAferoSpecifics(t *testing.T) {
	driver := setup(t).AsAfero()
	t.Run("Chmod", func(t *testing.T) {
		require.NoError(t, writeFile(driver, "Chmod", bytes.NewBufferString("Chmod test")))
		require.NoError(t, driver.Chmod("Chmod", os.FileMode(755)))
	})
	t.Run("Chtimes", func(t *testing.T) {
		require.NoError(t, writeFile(driver, "Chtimes", bytes.NewBufferString("Chtimes test")))
		aTime := time.Unix(1606435200, 0)
		mTime := time.Unix(1582675200, 0)
		require.NoError(t, driver.Chtimes("chtimes", aTime, mTime))
	})
}

func TestOpen(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		t.Run("existing File", func(t *testing.T) {
			driver := setup(t).AsAfero()

			newFile(t, driver, "Folder1/File1")

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.NoError(t, err)
			defer func() { require.NoError(t, f.Close()) }()

			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, "Hello World", string(data))

			t.Run("Partial read", func(t *testing.T) {
				_, err := f.Seek(6, io.SeekStart)
				require.NoError(t, err)
				data, err = ioutil.ReadAll(f)
				require.NoError(t, err)
				require.Equal(t, "World", string(data))
			})
		})
		t.Run("existing big File", func(t *testing.T) {
			driver := setup(t).AsAfero()

			var buf [4096*3 + 15]byte
			_, err := rand.Read(buf[:])
			require.NoError(t, err)

			err = writeFile(driver, "Folder1/File1", bytes.NewBuffer(buf[:]))
			require.NoError(t, err)

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.NoError(t, err)
			defer func() { require.NoError(t, f.Close()) }()

			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			require.EqualValues(t, buf[:], data)
		})
		t.Run("non-existing File", func(t *testing.T) {
			driver := setup(t).AsAfero()

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing File with create", func(t *testing.T) {
			driver := setup(t).AsAfero()

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY|os.O_CREATE, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
	})

	t.Run("write", func(t *testing.T) {
		t.Run("existing File", func(t *testing.T) {
			driver := setup(t).AsAfero()

			newFile(t, driver, "Folder1/File1")

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY, os.FileMode(0))
			require.NoError(t, err)
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare File contents
			r, err := driver.Open("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "Hello Universe", string(received))
		})
		t.Run("non-existing File", func(t *testing.T) {
			driver := setup(t).AsAfero()

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing File with create", func(t *testing.T) {
			driver := setup(t).AsAfero()

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY|os.O_CREATE, os.FileMode(0))
			require.NoError(t, err)
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare File contents
			r, err := driver.Open("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "Hello Universe", string(received))
		})
	})
}

func mustWriteFile(t *testing.T, driver afero.Fs, path string, content string) {
	require.NoError(t, writeFile(driver, path, bytes.NewBufferString(content)))
}

func writeFile(driver afero.Fs, path string, content io.Reader) error {
	f, err := driver.OpenFile(path, os.O_WRONLY|os.O_CREATE, os.FileMode(777))
	if err != nil {
		return err
	}

	defer func() {
		if err = f.Close(); err != nil {
			log.Println("Couldn't close Fi:", err)
		}
	}()

	if _, err := io.Copy(f, content); err != nil {
		return err
	}

	return nil
}

func newFile(t *testing.T, driver afero.Fs, path string) {
	err := writeFile(driver, path, bytes.NewBufferString("Hello World"))
	require.NoError(t, err)
}

func newDirectory(t *testing.T, driver afero.Fs, path string) {
	require.NoError(t, driver.Mkdir(path, os.FileMode(0)))
}

func getError(_ os.FileInfo, err error) error {
	return err
}
