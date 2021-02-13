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
		Authenticate: func(url string) (string, error) {
			fmt.Println("Go to", url)
			var authCode string
			fmt.Print("Your code:")
			_, err := fmt.Scanln(&authCode)
			return authCode, err
		},
	}

	if h.ClientID == "" || h.ClientSecret == "" {
		fmt.Println("You need to specify GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET")
		return
	}

	_, err := h.NewHTTPClient(context.Background())

	if err != nil {
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
