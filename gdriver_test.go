package gdriver

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fclairamb/afero-gdrive/oauthhelper"
	"github.com/hjson/hjson-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

var (
	prefix string
)

func init() {
	prefix = time.Now().UTC().Format("20060102_150405.000000")
}

func setup(t *testing.T) (*GDriver, func()) {
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
			return "", fmt.Errorf("please specify a valid token.json File")
		},
	}
	var client *http.Client
	var driver *GDriver
	var token []byte

	token, err = base64.StdEncoding.DecodeString(os.Getenv("GOOGLE_TOKEN"))
	require.NoError(t, err)

	helper.Token = new(oauth2.Token)

	require.NoError(t, json.Unmarshal([]byte(token), helper.Token))

	client, err = helper.NewHTTPClient(context.Background())
	require.NoError(t, err)

	driver, err = New(client)

	require.NoError(t, err)

	// prepare test directory

	fullPath := sanitizeName(fmt.Sprintf("GDriveTest-%s-%s", t.Name(), prefix))
	driver.DeleteDirectory(fullPath)
	err = driver.MkdirAll(fullPath, os.FileMode(700))
	require.NoError(t, err)

	_, err = driver.SetRootDirectory(fullPath)
	require.NoError(t, err)

	return driver, func() {
		_, err = driver.SetRootDirectory("")
		require.NoError(t, err)
		require.NoError(t, driver.DeleteDirectory(fullPath))
	}
}

func TestMakeDirectory(t *testing.T) {
	t.Run("simple creation", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		err := driver.MkdirAll("Folder1", os.FileMode(700))
		require.NoError(t, err)

		// Folder1 created?
		fi, err := driver.Stat("Folder1")
		require.NoError(t, err)
		require.Equal(t, "Folder1", fi.Name())
	})

	t.Run("simple creation in existent directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, driver.MkdirAll("Folder1", os.FileMode(700)))

		err := driver.MkdirAll("Folder1/Folder2", os.FileMode(700))
		require.NoError(t, err)

		// Folder1/Folder2 created?
		_, err = driver.Stat("Folder1/Folder2")
		require.NoError(t, err)
	})

	t.Run("create non existent directories", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, driver.MkdirAll("Folder1/Folder2/Folder3", os.FileMode(0)))
		fi, err := driver.Stat("Folder1/Folder2/Folder3")
		require.NoError(t, err)
		require.Equal(t, "Folder3", fi.Name())

		// Folder1 created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// Folder1/Folder2 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2")))

		// Folder1/Folder2/Folder3 created?
		require.NoError(t, getError(driver.Stat("Folder1/Folder2/Folder3")))
	})

	t.Run("creation of existent directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		err := driver.MkdirAll("Folder1/Folder2", os.FileMode(0))
		require.NoError(t, err)

		err = driver.MkdirAll("Folder1/Folder2", os.FileMode(0))
		require.NoError(t, err)
		//require.Equal(t, "Folder1/Folder2", fi.Path())
	})

	t.Run("create folder as a descendant of a File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		require.EqualError(t, driver.MkdirAll("Folder1/File1/Folder2", os.FileMode(0)), "unable to create directory in `Folder1/File1': `File1' is not a directory")
	})

	t.Run("make root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, driver.Mkdir("", os.FileMode(0)))
	})
}

func TestPutFile(t *testing.T) {
	t.Run("in root folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		writeFile(t, driver, "File1", "Hello World")

		fi, err := driver.Stat("File1")
		require.NoError(t, err)

		require.Equal(t, "File1", fi.Name())

		// File created?
		fi, err = driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi.Name())

		// Compare File contents
		_, r, err := driver.GetFile("File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("in non existing folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create File
		writeFile(t, driver, "Folder1/File1", "Hello World")

		// Folder created?
		require.NoError(t, getError(driver.Stat("Folder1")))

		// File created?
		fi, err := driver.Stat("Folder1/File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi.Name())

		// Compare File contents
		_, r, err := driver.GetFile("Folder1/File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))
	})

	t.Run("as descendant of File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create File
		require.NoError(t, putFile(driver, "Folder1/File1", bytes.NewBufferString("Hello World")))

		_, err := driver.putFile("Folder1/File1/File2", bytes.NewBufferString("Hello World"))
		require.EqualError(t, err, "unable to create File in `Folder1/File1': `File1' is not a directory")
	})

	t.Run("empty target", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create File
		require.EqualError(t, putFile(driver, "", bytes.NewBufferString("Hello World")), "path cannot be empty")
	})

	t.Run("overwrite File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		// create File
		writeFile(t, driver, "File1", "Hello World")

		// File created?
		fi1, err := driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi1.Name())

		// Compare File contents
		_, r, err := driver.GetFile("File1")
		require.NoError(t, err)
		received, err := ioutil.ReadAll(r)
		require.Equal(t, "Hello World", string(received))

		// create File
		writeFile(t, driver, "File1", "Hello Universe")

		// File created?
		fi2, err := driver.Stat("File1")
		require.NoError(t, err)
		require.Equal(t, "File1", fi2.Name())

		// Compare File contents
		_, r, err = driver.GetFile("File1")
		require.NoError(t, err)
		received, err = ioutil.ReadAll(r)
		require.Equal(t, "Hello Universe", string(received))
	})
}

func TestGetFile(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	newFile(t, driver, "Folder1/File1", "Hello World")

	// Compare File contents
	fi, r, err := driver.GetFile("Folder1/File1")
	require.NoError(t, err)
	received, err := ioutil.ReadAll(r)
	require.Equal(t, "Hello World", string(received))
	require.Equal(t, "Folder1/File1", fi.Path())

	// Get File contents of an Folder
	_, _, err = driver.GetFile("Folder1")
	require.EqualError(t, err, "`Folder1' is a directory")
}

func TestDelete(t *testing.T) {
	t.Run("delete File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete File
		require.NoError(t, driver.Remove("File1"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("File1")), "`File1' does not exist")
	})

	t.Run("delete directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.Remove("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestDeleteDirectory(t *testing.T) {
	t.Run("delete File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		// delete File
		require.EqualError(t, driver.DeleteDirectory("File1"), "`File1' is not a directory")

		// File  should not be deleted
		require.NoError(t, getError(driver.Stat("File1")))
	})

	t.Run("delete directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newDirectory(t, driver, "Folder1")

		// delete folder
		require.NoError(t, driver.DeleteDirectory("Folder1"))

		// Folder1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})
}

func TestListDirectory(t *testing.T) {
	t.Run("standart", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		newFile(t, driver, "Folder1/File2", "Hello World")

		// var files []*FileInfo
		files, err := driver.ListDirectory("Folder1", 2000)
		require.NoError(t, err)
		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Name(), files[j].Name()) == -1
		})

		require.Equal(t, "File1", files[0].Name())
		require.Equal(t, "File2", files[1].Name())

		// Remove contents
		require.NoError(t, driver.Remove("Folder1/File1"))
		require.NoError(t, driver.Remove("Folder1/File2"))

		// File1 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// File2 deleted?
		require.EqualError(t, getError(driver.Stat("Folder1/File2")), "`Folder1/File2' does not exist")

		// Test if folder is empty
		files, err = driver.ListDirectory("Folder1", 2000)
		require.NoError(t, err)

		require.Len(t, files, 0)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		_, err := driver.ListDirectory("Folder1", 2000)

		require.EqualError(t, err, "`Folder1' does not exist")
	})

	t.Run("list File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "File1", "Hello World")

		_, err := driver.ListDirectory("File1", 2000)

		require.EqualError(t, err, "`File1' is not a directory")
	})

	/*
		t.Run("callback error", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			newFile(t, driver, "File1", "Hello World")

			err := driver.ListDirectory("", func(f *FileInfo) error {
				return errors.New("Custom Error")
			})
			require.IsType(t, CallbackError{}, err)
			require.EqualError(t, err, "callback throwed an error: Custom Error")
		})
	*/
}

func TestRename(t *testing.T) {
	t.Run("rename with simple name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// rename
		err := driver.Rename("Folder1/File1", "File2")
		require.NoError(t, err)
		// require.Equal(t, "Folder1/File2", fi.Path())

		// File renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")
	})

	t.Run("rename with path", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		require.NoError(t, driver.Rename("Folder1/File1", "Folder2/File2"))

		// File renamed?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Folder2 should not have been created
		require.EqualError(t, getError(driver.Stat("Folder2")), "`Folder2' does not exist")
	})

	t.Run("rename directory", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.NoError(t, driver.Mkdir("Folder1", os.FileMode(0)))

		// rename
		require.NoError(t, driver.Rename("Folder1", "Folder2"))
		// require.Equal(t, "Folder2", fi.Path())

		// Folder2 renamed?
		require.NoError(t, getError(driver.Stat("Folder2")))

		// old folder gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")
	})

	t.Run("invalid new name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		require.EqualError(t, driver.Rename("Folder1/File1", ""), "new name cannot be empty")
	})

	t.Run("rename root node", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, driver.Rename("/", "Test"), "root cannot be renamed")
	})
}

func TestMove(t *testing.T) {
	t.Run("move into another folder with another name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move File
		fi, err := driver.Move("Folder1/File1", "Folder2/File2")
		require.NoError(t, err)
		require.Equal(t, "Folder2/File2", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File2")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into another folder with same name", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move File
		fi, err := driver.Move("Folder1/File1", "Folder2/File1")
		require.NoError(t, err)
		require.Equal(t, "Folder2/File1", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder2/File1")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("move into same folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// Move File
		fi, err := driver.Move("Folder1/File1", "Folder1/File2")
		require.NoError(t, err)
		require.Equal(t, "Folder1/File2", fi.Path())

		// File moved?
		require.NoError(t, getError(driver.Stat("Folder1/File2")))

		// Old File gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")
	})

	t.Run("move root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("", "Folder1")), "root cannot be moved")
	})

	t.Run("invalid target", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, getError(driver.Move("Folder1", "")), "new path cannot be empty")
	})
}

func TestTrash(t *testing.T) {
	t.Run("trash File", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash File
		require.NoError(t, driver.Trash("Folder1/File1"))

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1/File1' does not exist")

		// Old Folder still exists?
		require.NoError(t, getError(driver.Stat("Folder1")))
	})

	t.Run("trash folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash folder
		require.NoError(t, driver.Trash("Folder1"))

		// Folder1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1")), "`Folder1' does not exist")

		// File1 gone?
		require.EqualError(t, getError(driver.Stat("Folder1/File1")), "`Folder1' does not exist")
	})

	t.Run("trash root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		require.EqualError(t, driver.Trash(""), "root cannot be trashed")
	})
}

func TestListTrash(t *testing.T) {
	if hostname, _ := os.Hostname(); hostname != "MacBook-Pro-de-Florent.local" {
		t.Skip("Do not execute trash test")
	}
	t.Run("root", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		newFile(t, driver, "Folder2/File2", "Hello World")
		newFile(t, driver, "Folder3/File3", "Hello World")

		// trash File1
		require.NoError(t, driver.Trash("Folder1/File1"))
		// trash Folder2
		require.NoError(t, driver.Trash("Folder2"))

		var files []*FileInfo
		require.NoError(t, driver.ListTrash("", func(f *FileInfo) error {
			files = append(files, f)
			return nil
		}))

		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, fmt.Sprintf("GDriveTest-TestListTrash-root-%s/Folder1/File1", prefix), files[0].Path())
		require.Equal(t, fmt.Sprintf("GDriveTest-TestListTrash-root-%s/Folder2", prefix), files[1].Path())
	})

	t.Run("of folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		newFile(t, driver, "Folder1/File2", "Hello World")
		newFile(t, driver, "Folder2/File3", "Hello World")

		// trash File1 and File2
		require.NoError(t, driver.Trash("Folder1/File1"))
		require.NoError(t, driver.Trash("Folder1/File2"))

		var files []*FileInfo
		require.NoError(t, driver.ListTrash("Folder1", func(f *FileInfo) error {
			files = append(files, f)
			return nil
		}))

		require.Len(t, files, 2)

		// sort so we can be sure the test works with random order
		sort.Slice(files, func(i, j int) bool {
			return strings.Compare(files[i].Path(), files[j].Path()) == -1
		})

		require.Equal(t, "Folder1/File1", files[0].Path())
		require.Equal(t, "Folder1/File2", files[1].Path())
	})

	t.Run("callback error", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		// trash File1
		require.NoError(t, driver.Trash("Folder1/File1"))

		err := driver.ListTrash("", func(f *FileInfo) error {
			return errors.New("Custom Error")
		})
		require.IsType(t, CallbackError{}, err)
		require.EqualError(t, err, "callback throwed an error: Custom Error")
	})
}

func TestIsInRoot(t *testing.T) {
	t.Run("in folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")

		fi, err := driver.getFile(driver.rootNode, "Folder1/File1", googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))))
		require.NoError(t, err)

		inRoot, parentPath, err := isInRoot(driver.srv, driver.rootNode.file.Id, fi.file, "")
		require.NoError(t, err)
		require.True(t, inRoot)
		require.Equal(t, "Folder1", parentPath)
	})

	t.Run("not in folder", func(t *testing.T) {
		driver, teardown := setup(t)
		defer teardown()

		newFile(t, driver, "Folder1/File1", "Hello World")
		require.NoError(t, driver.Mkdir("Folder2", os.FileMode(0)))
		_, err := driver.Stat("Folder1/File1")
		// folder2Id := fi.file.Id

		_, err = driver.getFile(driver.rootNode, "Folder1/File1", googleapi.Field(fmt.Sprintf("files(%s,parents)", googleapi.CombineFields(fileInfoFields))))
		require.NoError(t, err)

		/*
			inRoot, parentPath, err := isInRoot(driver.srv, folder2Id, fi.file, "")
			require.NoError(t, err)
			require.False(t, inRoot)
			require.Equal(t, "", parentPath)
		*/
	})
}

func TestGetHash(t *testing.T) {
	driver, teardown := setup(t)
	defer teardown()

	buf := bytes.NewBufferString("Hello World")
	hash1 := md5.Sum(buf.Bytes())
	err := putFile(driver, "File1", buf)
	require.NoError(t, err)

	_, hash2, err := driver.GetFileHash("File1", HashMethodMD5)
	require.NoError(t, err)

	hash2, err = hex.DecodeString(string(hash2))
	require.NoError(t, err)

	require.EqualValues(t, hash1[:], hash2)
}

func TestOpen(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		t.Run("existing File", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			newFile(t, driver, "Folder1/File1", "Hello World")

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.NoError(t, err)
			defer f.Close()

			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			require.Equal(t, "Hello World", string(data))
		})
		t.Run("existing big File", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			var buf [4096*3 + 15]byte
			_, err := rand.Read(buf[:])
			require.NoError(t, err)

			err = putFile(driver, "Folder1/File1", bytes.NewBuffer(buf[:]))
			require.NoError(t, err)

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.NoError(t, err)
			defer f.Close()

			data, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			require.EqualValues(t, buf[:], data)
		})
		t.Run("non-existing File", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing File with create", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.OpenFile("Folder1/File1", os.O_RDONLY|os.O_CREATE, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
	})

	t.Run("write", func(t *testing.T) {
		t.Run("existing File", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			newFile(t, driver, "Folder1/File1", "Hello World")

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY, os.FileMode(0))
			require.NoError(t, err)
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare File contents
			_, r, err := driver.GetFile("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.Equal(t, "Hello Universe", string(received))
		})
		t.Run("non-existing File", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY, os.FileMode(0))
			require.EqualError(t, err, FileNotExistError{Path: "Folder1/File1"}.Error())
			require.Nil(t, f)
		})
		t.Run("non-existing File with create", func(t *testing.T) {
			driver, teardown := setup(t)
			defer teardown()

			f, err := driver.OpenFile("Folder1/File1", os.O_WRONLY|os.O_CREATE, os.FileMode(0))
			n, err := io.WriteString(f, "Hello Universe")
			require.NoError(t, err)
			require.Equal(t, 14, n)
			require.NoError(t, f.Close())

			// Compare File contents
			_, r, err := driver.GetFile("Folder1/File1")
			require.NoError(t, err)
			received, err := ioutil.ReadAll(r)
			require.Equal(t, "Hello Universe", string(received))
		})
	})
}

func writeFile(t *testing.T, driver *GDriver, path string, content string) {
	require.NoError(t, putFile(driver, path, bytes.NewBufferString(content)))
}

func putFile(driver *GDriver, path string, content io.Reader) error {
	f, err := driver.OpenFile(path, os.O_WRONLY|os.O_CREATE, os.FileMode(777))
	if err != nil {
		return err
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Println("Couldn't close file:", err)
		}
	}()
	if _, err := io.Copy(f, content); err != nil {
		return err
	}
	return nil
}

func newFile(t *testing.T, driver *GDriver, path, contents string) {
	err := putFile(driver, path, bytes.NewBufferString(contents))
	require.NoError(t, err)
}

func newDirectory(t *testing.T, driver *GDriver, path string) {
	err := driver.Mkdir(path, os.FileMode(0))
	require.NoError(t, err)
}

func getError(_ os.FileInfo, err error) error {
	return err
}
