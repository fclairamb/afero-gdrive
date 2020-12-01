package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	gdriver "github.com/fclairamb/afero-gdrive"
	"github.com/fclairamb/afero-gdrive/oauthhelper"
)

func main() {
	// Setup OAuth
	helper := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Authenticate: func(url string) (string, error) {
			fmt.Printf("OpenFile to authorize Example to access your drive\n%s\n", url)

			var code string
			fmt.Printf("Code: ")
			if _, err := fmt.Scan(&code); err != nil {
				return "", fmt.Errorf("Unable to read authorization code %v", err)
			}
			return code, nil
		},
	}

	var err error
	// Try to load a client token from file
	helper.Token, err = oauthhelper.LoadTokenFromFile("token.json")
	if err != nil {
		// if the error is NotExist error continue
		// we will create a token
		if !os.IsNotExist(err) {
			log.Panic(err)
		}
	}

	// Create a new authorized HTTP client
	client, err := helper.NewHTTPClient(context.Background())
	if err != nil {
		log.Panic(err)
	}

	// store the token for future use
	if err = oauthhelper.StoreTokenToFile("token.json", helper.Token); err != nil {
		log.Panic(err)
	}

	// create a gdriver instance
	gdrive, err := gdriver.New(client)
	if err != nil {
		log.Panic(err)
	}

	files, err := gdrive.listDirectoryPath("/", 2000)

	for _, f := range files {
		fmt.Printf("%s\t%d\t%s\n", f.Name(), f.Size(), f.ModTime())
	}

	src, err := os.Open("example")
	if err != nil {
		log.Panic(err)
	}
	defer src.Close()
	dst, err := gdrive.OpenFile("bins/example", os.O_WRONLY|os.O_CREATE, os.FileMode(755))
	if err != nil {
		log.Panic(err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		log.Panic(err)
	}
}
