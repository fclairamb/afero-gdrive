# Afero Google Drive Driver

## About
It provides an [afero filesystem](https://github.com/spf13/afero/) implementation of a [Google Drive](https://aws.amazon.com/s3/) backend.

This was created to provide a backend to the [ftpserver](https://github.com/fclairamb/ftpserver) but can definitely be used in any other code.

I'm very opened to any improvement through issues or pull-request that might lead to a better implementation or even better testing.

## Key points
- Download & upload file streaming
- 65% coverage: This isn't great, I intend to improve it to reach 80%. As it's a third-party API more would take way too much time.
- Very carefully linted


## Known limitations
- File appending / seeking for write is not supported because Google Drive doesn't support it, it could be simulated by rewriting entire files.
- Chmod is saved as a property and not used at this time
- No cache 

## How to use
Note: Errors handling is skipped for brevity but you definitely have to handle it.
```golang

import (
	"github.com/fclairamb/afero-gdrive/oauthhelper"
)

func main() {
  // We declare the OAuh2 app
	helper := oauthhelper.Auth{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Authenticate: func(url string) (string, error) {
			return "", ErrNotSupported
		},
	}

  // Pass a token 
	token, _ := base64.StdEncoding.DecodeString(os.Getenv("GOOGLE_TOKEN"))
	
  // Initialize the authenticated client
  client, _ := helper.NewHTTPClient(context.Background())
  
  // Initialize the FS from the athenticated http client
  fs, _ := New(client)

  // And use it
  file, _ := fs.OpenFile("my_file.txt", os.O_WRONLY, 0777)
  file.WriteString("Hello world !")
  file.Close()
}
```


## Credits
This is a fork from [T4cC0re/gdriver](https://github.com/T4cC0re/gdriver) which is itself a fork of [eun/gdriver](https://github.com/eun/gdriver).

The code was massively modified. Most of the changes are improvements. The file listing API from the original implemented based on callbacks is much better though,  but this was needed to respect the afero API.
