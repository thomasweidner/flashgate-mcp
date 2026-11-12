# Third-party notices

This file records third-party components and assets distributed with or used
to build or test FlashGate MCP.

## Font Awesome Free

- Component: `bolt-lightning` icon, Classic Solid
- Version: `7.2.0`
- Project: Font Awesome Free by Fonticons, Inc.
- Source: `FortAwesome/Font-Awesome`, path `svgs/solid/bolt-lightning.svg`
- License: Creative Commons Attribution 4.0 International (`CC BY 4.0`)
- License text: <https://creativecommons.org/licenses/by/4.0/>
- Copyright: Copyright 2026 Fonticons, Inc.

The attribution and CC BY 4.0 license apply to the unmodified upstream
`assets/branding/fontawesome-bolt-lightning-solid.svg` and to the derived
`assets/branding/flashgate.ico`. The SVG retains its embedded Font Awesome
attribution; the ICO is a deterministic raster derivative of that SVG.

## goversioninfo

- Module: `github.com/josephspurrier/goversioninfo`
- Version: `v1.7.0`
- Purpose: generation of Windows `VERSIONINFO` and icon resources
- License: MIT

## rsrc

- Module: `github.com/akavel/rsrc`
- Version: resolved through the pinned `goversioninfo` dependency
- Purpose: Windows COFF resource generation
- License: MIT

## go-toml

- Module: `github.com/pelletier/go-toml/v2`
- Version: `v2.4.3`
- Purpose: strict decoding of FlashGate's startup configuration
- License: MIT

## jsonschema

- Module: `github.com/santhosh-tekuri/jsonschema/v6`
- Version: `v6.0.3`
- Purpose: Draft 2020-12 conformance tests
- License: Apache 2.0

## Go text

- Module: `golang.org/x/text`
- Version: `v0.41.0`
- Purpose: transitive test dependency of the schema validator
- License: BSD 3-Clause

The exact Go dependency graph and checksums are recorded in `go.mod`,
`go.sum`, and `vendor/modules.txt`.
