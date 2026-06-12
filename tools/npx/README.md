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
| `upgrade`            | Upgrade to the latest release (side-by-side)       |
| `try <use-case>`     | Download and launch a use-case sample app          |
| `integrate <tech>`   | Configure a technology integration _(coming soon)_ |

## Flags

| Flag              | Description                              |
| ----------------- | ---------------------------------------- |
| `--setup`         | Force re-run setup                       |
| `--verbose`, `-v` | Show detailed output                     |
| `--help`, `-h`    | Show help                                |

### Upgrade flags

| Flag       | Description                                          |
| ---------- | ---------------------------------------------------- |
| `--direct` | Upgrade in-place (stop current, upgrade, restart)    |

## Requirements

- **Node.js** `>= 18`

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
