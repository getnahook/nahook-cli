# Changelog

All notable changes to the Nahook CLI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial release scaffolding (`nahook login`, `nahook logout`, `nahook whoami`).
- RFC 8628 device authorization grant for browser-based login.
- Token storage in `~/.nahook/config.toml` (override with `$NAHOOK_CONFIG_DIR`).
- `User-Agent: nahook-cli/<version> (<os>/<arch>; <go-version>)` and
  `X-Nahook-Client: cli/<version> <os>/<arch>` on every request.
