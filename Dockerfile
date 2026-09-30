# syntax=docker/dockerfile:1
ARG BASE_IMAGE="antilax3/wolfi:latest"

FROM --platform=${BUILDPLATFORM} golang:1.27-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

SHELL ["/bin/ash", "-euo", "pipefail", "-c"]

RUN <<'EOT'
set -euo pipefail

echo "**** test smartexporter ****"
go test ./...

echo "**** build smartexporter ****"
CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build -trimpath -buildvcs=false -ldflags="-s -w" \
  -o /out/app/smartexporter ./cmd/smartexporter
EOT

FROM ${BASE_IMAGE}

# set version labels
ARG build_date
ARG version
LABEL build_date="${build_date}"
LABEL version="${version}"
LABEL maintainer="Nightah"

# set working directory
WORKDIR /app

SHELL ["/bin/ash", "-euo", "pipefail", "-c"]

# copy local files
COPY --link root/ /
COPY --link --from=build /out/ /

# install runtime packages
RUN <<'EOT'
set -euo pipefail

# the two bases package setcap under different names
if ls /lib/ld-musl-* > /dev/null 2>&1; then
  SETCAP_PACKAGE="libcap-setcap"
else
  SETCAP_PACKAGE="libcap-utils"
fi

echo "**** install smartmontools ****"
apk add --no-cache smartmontools

echo "**** let smartctl open the disks as abc ****"
# The service runs as abc, which can neither open a disk's device node nor send it the raw ATA, SCSI and NVMe
# commands SMART is read with, so smartctl carries the three capabilities those need. The container still has to be
# granted them, by --privileged or by --cap-add with --device.
apk add --no-cache --virtual .setcap "${SETCAP_PACKAGE}"
setcap cap_dac_override,cap_sys_admin,cap_sys_rawio+ep "$(command -v smartctl)"
apk del --no-cache .setcap
EOT

# ports and volumes
EXPOSE 9120
VOLUME /config
