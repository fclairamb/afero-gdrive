// This program helps the setup of credentials for tests
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fclairamb/afero-gdrive/oauthhelper"
)

func main() {
	h := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}

	if h.ClientID == "" || h.ClientSecret == "" {
		fmt.Println("You need to specify GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET")

		return
	}

	// With no Authenticate callback set, NewHTTPClient runs the loopback flow: it
	// prints an authorization URL, then captures the code on a local web server.
	if _, err := h.NewHTTPClient(context.Background()); err != nil {
		fmt.Println("Error:", err)

		return
	}

	if err := oauthhelper.StoreTokenToFile("token.json", h.Token); err != nil {
		fmt.Println("Couldn't save file")

		return
	}

	if base64, err := oauthhelper.GetTokenBase64(h.Token); err != nil {
		fmt.Println("Couldn't get token")
	} else {
		fmt.Println("GOOGLE_TOKEN value:", base64)
	}
}
