# ThunderID

![ThunderID NPX](https://raw.githubusercontent.com/thunder-id/thunderid/refs/heads/main/docs/static/assets/images/readme/repo-banner-npx.png)

Run ThunderID instantly — no manual download or setup required.

## Quick Start

```bash
npx thunderid
```

On first run this downloads the latest ThunderID release, initializes the platform, and starts it. Later runs reuse
the cached installation and start immediately.

## Commands

| Command              | Description                                        |
| -------------------- | -------------------------------------------------- |
| _(none)_             | Install (if needed) and start ThunderID            |
| `upgrade`            | Upgrade to the latest release                      |
| `try <use-case>`     | Download and launch a use-case sample app          |
| `integrate <tech>`   | Configure a technology integration _(coming soon)_ |

## Flags

| Flag                | Description                                                      |
| ------------------- | ---------------------------------------------------------------- |
| `--product-version` | Install a specific release, or read releases from another source |
| `--setup`           | Force re-run setup                                               |
| `--verbose`, `-v`   | Show detailed output                                             |
| `--version`, `-V`   | Show the CLI version                                             |
| `--help`, `-h`      | Show help                                                        |

## Choosing a Version

By default ThunderID installs the latest release. `--product-version` takes either a version or a
manifest URL, so you can pin a release or point at your own mirror.

```bash
# Install and run a specific release instead of the latest.
npx thunderid --product-version 1.0.1

# Read releases from somewhere other than thunderid.dev, such as an internal mirror.
npx thunderid --product-version https://releases.example.com/releases.json

# Both: that version, from that mirror.
npx thunderid --product-version https://releases.example.com/releases.json --product-version 1.0.1
```

The same value can be given as `THUNDERID_PRODUCT_VERSION` for scripted runs. Flags win over the
environment.

A few things worth knowing:

- A version that is already installed starts straight away; nothing is downloaded again.
- A version the manifest does not carry is an error listing the ones it does, rather than
  quietly installing something else.
- Pinning suppresses the upgrade notice, and `upgrade` refuses to run while pinned, so it cannot
  move you off the release you asked for.
- A custom source is used on its own: ThunderID will not fall back to the public releases behind
  your back, and it prints the source it is reading so a stray environment variable is never
  silent.
- Releases are executable, so a custom source must be `https`, except on `localhost`.

## Requirements

- **Node.js** `>= 22.23.1`

## Supported Platforms

| OS      | Architectures  |
| ------- | -------------- |
| macOS   | `x64`, `arm64` |
| Linux   | `x64`, `arm64` |
| Windows | `x64`          |

## About

- **npm:** [`thunderid`](https://www.npmjs.com/package/thunderid)
- **source:** <https://github.com/thunder-id/thunderid>
- **docs:** <https://thunderid.dev>

## License

[Apache License 2.0](https://github.com/thunder-id/thunderid/blob/main/LICENSE)
