<p align="center">
  <a href="https://github.com/AntilaX-3/"><img src="https://avatars.githubusercontent.com/u/35715409" width="150" title="AntilaX-3"></a>
</p>

<p align="center">
  <a href="https://buildkite.com/antilax-3/smartexporter"><picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fbuildkite%2Fb2243e303cea552d7f82f20e59c8bf93f9302811f85347406f%2Fmaster.json&query=%24.message&label=build&logo=buildkite&logoColor=%2314cc80&mode=dark&size=sm&variant=outline"><img alt="Build" src="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fbuildkite%2Fb2243e303cea552d7f82f20e59c8bf93f9302811f85347406f%2Fmaster.json&query=%24.message&label=build&logo=buildkite&logoColor=%2314cc80&mode=light&size=sm&variant=outline"></picture></a>
  <a href="https://github.com/antilax-3/docker-smartexporter/tags"><picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/github/antilax-3/docker-smartexporter/tag.svg?logo=github&mode=dark&size=sm&variant=outline"><img alt="GitHub Tag" src="https://shieldcn.dev/github/antilax-3/docker-smartexporter/tag.svg?logo=github&logoColor=%23181717&mode=light&size=sm&variant=outline"></picture></a>
  <a href="https://www.gnu.org/licenses/lgpl-3.0"><picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/github/antilax-3/docker-smartexporter/license.svg?logo=gnu&logoColor=%23a42e2b&mode=dark&size=sm&variant=outline"><img alt="License" src="https://shieldcn.dev/github/antilax-3/docker-smartexporter/license.svg?logo=gnu&logoColor=%23a42e2b&mode=light&size=sm&variant=outline"></picture></a>
  <a href="https://hub.docker.com/r/antilax3/smart-exporter/tags"><picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fdocker%2Fimage-size%2Fantilax3%2Fsmart-exporter%2Flatest.json&query=%24.message&label=image%20size&logo=docker&logoColor=%232496ed&mode=dark&size=sm&variant=outline"><img alt="Docker Size" src="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fdocker%2Fimage-size%2Fantilax3%2Fsmart-exporter%2Flatest.json&query=%24.message&label=image%20size&logo=docker&logoColor=%232496ed&mode=light&size=sm&variant=outline"></picture></a>
  <a href="https://hub.docker.com/r/antilax3/smart-exporter"><picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fdocker%2Fpulls%2Fantilax3%2Fsmart-exporter.json&query=%24.message&label=pulls&logo=docker&logoColor=%232496ed&mode=dark&size=sm&variant=outline"><img alt="Docker Pulls" src="https://shieldcn.dev/badge/dynamic/json.svg?url=https%3A%2F%2Fimg.shields.io%2Fdocker%2Fpulls%2Fantilax3%2Fsmart-exporter.json&query=%24.message&label=pulls&logo=docker&logoColor=%232496ed&mode=light&size=sm&variant=outline"></picture></a>
</p>

# AntilaX-3/smart-exporter

[smart-exporter](https://github.com/AntilaX-3/docker-smartexporter) is a simple server that periodically scrapes S.M.A.R.T stats and exports them via HTTP for Prometheus consumption, written in Go.
The attributes it supplies to Prometheus are configurable, as well as the labels it supplies.
## Usage
```
docker create --name=smartexporter \
-v <path to config>:/config \
-p 9120:9120 \
--privileged=true \
antilax3/smart-exporter
```

smartctl reads the disks as the container's `abc` user, through the `SYS_RAWIO` and `SYS_ADMIN` capabilities, which the container has to be granted. `--privileged` grants them along with every disk; to grant only those, replace it with `--cap-add SYS_RAWIO --cap-add SYS_ADMIN` and a `--device` for each disk, e.g. `--device /dev/sda`. Without them smartctl can't run at all.
## Tags

Two variants are built from the one Dockerfile, for `linux/amd64` and `linux/arm64`.

| Variant | Base | Tags |
| --- | --- | --- |
| wolfi | [antilax3/wolfi](https://hub.docker.com/r/antilax3/wolfi) | `latest` |
| alpine | [antilax3/alpine](https://hub.docker.com/r/antilax3/alpine) | `alpine` |

Wolfi is the default. Both variants run the same statically linked binary, with smartmontools from each base's own repository, so the choice between them is only the base. Every build is also tagged `BK<build>`, with `-alpine` appended for the alpine variant.

## Parameters
The parameters are split into two halves, separated by a colon, the left hand side representing the host and the right the container side. For example with a volume -v external:internal - what this shows is the volume mapping from internal to external of the container. So -v /mnt/app/config:/config would map /config from inside the container to be accessible from /mnt/app/config on the host's filesystem.

- `-v /config` - local path for smartexporter config file
- `-p 9120` - HTTP port for webserver
- `-e PUID` - for UserID, see below for explanation
- `-e PGID` - for GroupID, see below for explanation
- `-e TZ` - for setting timezone information, eg Australia/Melbourne

It is based on wolfi, or alpine linux for the `alpine` tag, with s6 overlay, for shell access whilst the container is running do `docker exec -it smartexporter /bin/bash`.

## User / Group Identifiers
Sometimes when using data volumes (-v flags) permissions issues can arise between the host OS and the container. We avoid this issue by allowing you to specify the user `PUID` and group `PGID`. Ensure the data volume directory on the host is owned by the same user you specify and it will "just work".

In this instance `PUID=1001` and `PGID=1001`. To find yours use `id user` as below:
`$ id <dockeruser>`
    `uid=1001(dockeruser) gid=1001(dockergroup) groups=1001(dockergroup)`
    
## Volumes

The container uses a single volume mounted at '/config'. This volume stores the configuration file 'smartexporter.json'.

    config
    |-- smartexporter.json

## Configuration

The smartexporter.json is copied to the /config volume when first run. It has one mandatory parameter and two optional ones.

    attributes:     Array (Required)  | The SMART attributes to report, described below
    scrapeInterval: Number (Optional) | Seconds between scrapes of the disks, 15 if left out; the default file sets 10
    port:           Number (Optional) | The port the metrics are served on, 9120 by default

A missing configuration file is replaced by the default one. One that can't be parsed, or has no `attributes`, is left in place for you to fix and the defaults are used until it is.

*attributes* is an array of objects. The objects define the SMART attributes that will be parsed and reported. The [default file](https://github.com/AntilaX-3/docker-smartexporter/blob/master/internal/config/default.json) has examples.

**Only one of either attributeID or attributeName is required.**

    attributeID: Number | The attribute ID
    attributeName: String | The attribute name
    name: String (Required) | The name reported to Prometheus, prepended with 'smartexporter_'
    help: String (Required) | Help text provided to Prometheus
    labelNames: Array of Strings | Mapped to data from the information section of smartctl. Can be used for labels, ie "Device" for /dev/sdx or "Serial Number" for the serial number of the HDD.

Metrics are served on `/metrics`, alongside the exporter's own `go_` and `process_` metrics. A disk is reported as of the last scrape, so a disk that is removed, or that smartctl can no longer query, stops being reported rather than keeping its last value. Only `/dev/sd*` disks are scraped.

[Known S.M.A.R.T. attributes (Wikipedia)](https://en.wikipedia.org/w/index.php?title=S.M.A.R.T.#Known_ATA_S.M.A.R.T._attributes)
## Development

Linting runs locally through [lefthook](https://github.com/evilmartians/lefthook). Install the hooks once per clone:

```bash
lefthook install
```

`pre-commit` runs [golangci-lint](https://golangci-lint.run) and `go test`, [editorconfig-checker](https://github.com/editorconfig-checker/editorconfig-checker), [hadolint](https://github.com/hadolint/hadolint), `jq`, [shellcheck](https://github.com/koalaman/shellcheck), [typos](https://github.com/crate-ci/typos) and [yamllint](https://github.com/adrienverge/yamllint) over the staged files, and `commit-msg` enforces [Conventional Commits](https://www.conventionalcommits.org). Run everything on demand with:

```bash
lefthook run pre-commit --all-files
```

### Dependencies

Renovate bumps the go modules in `go.mod`, tidying `go.sum` after each update, the go version and the `golang` build image in the Dockerfile. The base images are followed at `antilax3/wolfi:latest` and `antilax3/alpine:latest`, so each build picks up their changes, and smartmontools follows each base image's package repository.

## Version
- **30/09/26:** Let smartctl read the disks as the container user
- **30/09/26:** Rewrite smart-exporter in Go and build it on the wolfi and alpine base images
- **30/09/26:** Build on wolfi by default and publish alpine under its own tag, for amd64 and arm64
- **04/07/25:** Updated to use alpine 3.22 image and s6 v3 service structure
- **24/06/19:** Add ability to capture attributes from SAS drives
- **24/02/18:** Updated to use alpine 3.7 image and build with jenkins
- **24/01/18:** Corrected documentation
- **24/01/18:** Refactoring & Cleaning
- **23/01/18:** Initial Release
