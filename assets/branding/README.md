# FlashGate branding assets

## Application icon

FlashGate MCP uses the Font Awesome Free `bolt-lightning` icon in the Classic
Solid style.

| Field | Value |
|---|---|
| Upstream project | `FortAwesome/Font-Awesome` |
| Upstream release | `7.2.0` |
| Upstream path | `svgs/solid/bolt-lightning.svg` |
| Icon license | Creative Commons Attribution 4.0 International (`CC BY 4.0`) |
| Upstream copyright | Copyright 2026 Fonticons, Inc. |
| Repository SVG SHA-256 (including final LF) | `e4a71aa99bccffbf50801129bf015a95ee52edff333ff9bbcf96b3808bbb3934` |
| SVG SHA-256 without final LF | `82691d34c696ab16fc079f9607ac3e4db9a1d56c0b386ef10800c16a7e467c86` |
| Generated ICO SHA-256 | `5931fe2714e38999356b5d607dfaa78ae3798df95274549de8fbe1db9b24a173` |
| Normalized seven-frame identity SHA-256 | `9cd0c1943ddc4dbd7f7c3b475e1d7ffbe99c9c6c436d24478a3497351bc6c032` |

The SVG retains the upstream content and embedded attribution comment. The
repository representation adds one final LF byte; the two hashes above
distinguish exact repository bytes from the representation without that byte.
Supply-chain evidence hashes the committed repository bytes.

`flashgate.ico` is a deterministic raster derivative generated from the SVG
with the following fixed parameters:

- square canvas: `512 x 512`;
- transparent background;
- glyph color: `#F5C542`;
- original path scaled by `0.875`;
- translation: `x=88`, `y=32`;
- ICO sizes: `16`, `24`, `32`, `48`, `64`, `128`, and `256` pixels;
- CairoSVG `2.8.2`;
- Pillow `12.2.0`.

No Font Awesome font files are included.

`cmd/iconverify` parses PE icon-group and icon resources without Explorer or
thumbnail caches, normalizes all frame descriptors and payload hashes, and
requires the embedded identity above for both Windows architectures.
