# Changelog

## [0.5.1](https://github.com/fclairamb/afero-gdrive/compare/v0.5.0...v0.5.1) (2026-10-28)


### Bug Fixes

* **deps:** update module google.golang.org/api to v0.299.0 ([#390](https://github.com/fclairamb/afero-gdrive/issues/390)) ([e47f63e](https://github.com/fclairamb/afero-gdrive/commit/e47f63eec522800d08f23dda5d10fc9fa4a40de9))
* **deps:** update module google.golang.org/api to v0.300.0 ([#393](https://github.com/fclairamb/afero-gdrive/issues/393)) ([bbcad63](https://github.com/fclairamb/afero-gdrive/commit/bbcad639ba0edf70b4774bc9cd4ba027f63afbf9))
* **deps:** update module google.golang.org/api to v0.301.0 ([#394](https://github.com/fclairamb/afero-gdrive/issues/394)) ([19050f7](https://github.com/fclairamb/afero-gdrive/commit/19050f75397364adc470b3594670763f2c54aabe))
* **deps:** update module google.golang.org/api to v0.302.0 ([#396](https://github.com/fclairamb/afero-gdrive/issues/396)) ([e63f2ef](https://github.com/fclairamb/afero-gdrive/commit/e63f2ef1c72f7eaac3c5168de510f34b3803c94f))
* **deps:** update module google.golang.org/api to v0.303.0 ([#397](https://github.com/fclairamb/afero-gdrive/issues/397)) ([6c0c0a7](https://github.com/fclairamb/afero-gdrive/commit/6c0c0a7d96f57a3b5e1b460c17891c98cb4f5622))
* **deps:** update module google.golang.org/api to v0.304.0 ([#398](https://github.com/fclairamb/afero-gdrive/issues/398)) ([0aec506](https://github.com/fclairamb/afero-gdrive/commit/0aec5065309819852f180be48ec6a5080e0f0548))
* **deps:** update module google.golang.org/api to v0.305.0 ([#401](https://github.com/fclairamb/afero-gdrive/issues/401)) ([d3aeedb](https://github.com/fclairamb/afero-gdrive/commit/d3aeedb7bed17548cc8c428103fd2f9b93fe1901))
* **deps:** update module google.golang.org/api to v0.307.0 ([#402](https://github.com/fclairamb/afero-gdrive/issues/402)) ([b3cc462](https://github.com/fclairamb/afero-gdrive/commit/b3cc462bffaddf61874afadb34b21709c3c26c66))
* **deps:** update module google.golang.org/api to v0.308.0 ([#403](https://github.com/fclairamb/afero-gdrive/issues/403)) ([c6caf26](https://github.com/fclairamb/afero-gdrive/commit/c6caf263478ab197dd67a0eaaea970ac81e7abf8))
* **deps:** update module google.golang.org/api to v0.309.0 ([#404](https://github.com/fclairamb/afero-gdrive/issues/404)) ([055fa93](https://github.com/fclairamb/afero-gdrive/commit/055fa93dceef58860974b00e5623d89cc65f4804))
* **deps:** update module google.golang.org/api to v0.310.0 ([#405](https://github.com/fclairamb/afero-gdrive/issues/405)) ([6a20132](https://github.com/fclairamb/afero-gdrive/commit/6a201328ea9668adfa94d6e131dca0e25458fa3d))

## [0.5.0](https://github.com/fclairamb/afero-gdrive/compare/v0.4.0...v0.5.0) (2026-09-15)

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
