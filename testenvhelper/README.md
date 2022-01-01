# Test environment helper

This small program helps create the test environment.

To use it, you need to set the `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables.

```shell
% export GOOGLE_CLIENT_ID=xxxx.apps.googleusercontent.com
% export GOOGLE_CLIENT_SECRET=xxxxxxxxxxxxxxxxxx
% ./testenvhelper
Go to https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=xxxx.apps.googleusercontent.com&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive&state=state-token
Your code: xxxxxx
GOOGLE_TOKEN value: xxxxxx
```