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

# copy local files
COPY --link root/ /
COPY --link --from=build /out/ /

# install runtime packages
RUN apk add --no-cache \
  smartmontools

# ports and volumes
EXPOSE 9120
VOLUME /config
