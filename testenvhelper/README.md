# Test environment helper

This small program helps create the test environment.

First create an app following [these instructions](https://medium.com/swlh/google-drive-api-with-python-part-i-set-up-credentials-1f729cb0372b).

When configuring the OAuth client, make sure to add `http://127.0.0.1` to the
list of authorized redirect URIs (the helper uses the
[loopback flow](https://developers.google.com/identity/protocols/oauth2/native-app)
and picks a random local port at runtime).

Set the `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables.

```shell
% export GOOGLE_CLIENT_ID=xxxx.apps.googleusercontent.com
% export GOOGLE_CLIENT_SECRET=xxxxxxxxxxxxxxxxxx
% ./testenvhelper
Open the following URL in your browser to authorize access:

https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=xxxx.apps.googleusercontent.com&redirect_uri=http%3A%2F%2F127.0.0.1%3A12345%2F&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive&state=xxxx

GOOGLE_TOKEN value: xxxxxxxxxxxxxxxxxx
```

Open the printed URL in your browser and approve the access request. The helper
runs a temporary local web server that captures the authorization code
automatically, so there is nothing to copy and paste.

Once you get the `GOOGLE_TOKEN`, you can set it locally, as secret
in your CI.

Please note that you can also set a `.env.json` looking like this to define your credentials:
```json
{
  "GOOGLE_CLIENT_ID": "xxxx.apps.googleusercontent.com",
  "GOOGLE_CLIENT_SECRET": "xxxxxxxxxxxxxxxxxx",
  "GOOGLE_TOKEN": "xxxxxxxxxxxxxxxxxx"
}
```
