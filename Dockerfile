# syntax=docker/dockerfile:1
ARG BASE_IMAGE="antilax3/node:latest"

# The bundle and every package it requires are plain javascript, with no native addon among them, so they are built
# once on the build platform and copied into the image of each target platform unchanged. The build stage uses the same
# base as the image it feeds, so each variant bundles with the node it runs on.
FROM --platform=${BUILDPLATFORM} ${BASE_IMAGE} AS build

WORKDIR /app

COPY root/app/ ./

SHELL ["/bin/ash", "-euo", "pipefail", "-c"]

RUN <<'EOT'
set -euo pipefail

echo "**** build node application ****"
npm install
# backpack 0.5 bundles with webpack 3, which hashes with md4, and node only offers md4 through the legacy provider.
NODE_OPTIONS=--openssl-legacy-provider npm run build

echo "**** keep only the runtime dependencies ****"
# backpack bundles src/ into build/main.js and leaves every package it requires external, so the image needs those
# packages and none of the toolchain that built the bundle.
npm prune --omit=dev
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
COPY --link root/etc/ /etc/
COPY --link --from=build /app/build/main.js /app/main.js
COPY --link --from=build /app/node_modules/ /app/node_modules/

# install runtime packages
RUN apk add --no-cache \
  smartmontools

# ports and volumes
EXPOSE 9120
VOLUME /config
