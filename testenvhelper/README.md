# Test environment helper

This small program helps create the test environment.

First create an app following [these instructions](https://medium.com/swlh/google-drive-api-with-python-part-i-set-up-credentials-1f729cb0372b).

Set the `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables.

```shell
% export GOOGLE_CLIENT_ID=xxxx.apps.googleusercontent.com
% export GOOGLE_CLIENT_SECRET=xxxxxxxxxxxxxxxxxx
% ./testenvhelper
Go to https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=xxxx.apps.googleusercontent.com&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive&state=state-token
Your code: xxxxxx
GOOGLE_TOKEN value: xxxxxxxxxxxxxxxxxx
```

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