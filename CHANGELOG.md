# Changelog

## [0.5.0](https://github.com/fclairamb/afero-gdrive/compare/v0.4.0...afero-gdrive-v0.5.0) (2026-09-15)

Dependency maintenance release. No API changes.

### Dependencies

* Update `google.golang.org/api` to v0.298.0.
* Update `golang.org/x/oauth2` to v0.37.0.
* Update `github.com/stretchr/testify` to v1.12.1.
* Update the Go toolchain to 1.27.1.
* CI: golangci-lint to v2.13.2, actions/checkout to v7, actions/setup-go to v7.

## [0.4.0](https://github.com/fclairamb/afero-gdrive/compare/v0.3.0...v0.4.0) (2026-06-30)

### ⚠ BREAKING CHANGES

* Logging now uses the standard library `log/slog` instead of `fclairamb/go-log`. Set `GDriver.Logger` to a `*slog.Logger`. ([#354](https://github.com/fclairamb/afero-gdrive/issues/354))

### Features

* Detect the MIME type from the file extension on upload, and persist the refreshed OAuth2 token. ([#257](https://github.com/fclairamb/afero-gdrive/issues/257))

### Bug Fixes

* Read all entries when globbing instead of only the first page. ([#249](https://github.com/fclairamb/afero-gdrive/issues/249))
* Use the OAuth2 loopback flow instead of the removed out-of-band flow, and skip integration tests when no credentials are set. ([#349](https://github.com/fclairamb/afero-gdrive/issues/349))

## [0.3.0](https://github.com/fclairamb/afero-gdrive/compare/v0.2.0...v0.3.0) (2021-07-27)

* Switch logging to go-log.

## [0.2.0](https://github.com/fclairamb/afero-gdrive/compare/v0.1.1...v0.2.0) (2021-02-14)

* Performance improvements.

## [0.1.1](https://github.com/fclairamb/afero-gdrive/compare/v0.1.0...v0.1.1) (2020-12-11)

* Fix release.

## 0.1.0 (2020-12-08)

* First release.
