FROM antilax3/node

# set version label
LABEL build_date=""
LABEL version=""
LABEL maintainer="Nightah"

# set environment variables
ENV NODE_CONFIG_DIR /config

# set working directory
WORKDIR /app

# copy local files
COPY root/ /

# install packages
RUN \
 apk add --no-cache \
 	smartmontools && \
 echo "**** build node application ****" && \
 cd /app && npm install && npm run build && mv build/main.js . && \
 echo "**** cleanup ****" && \
 rm -rf \
 	package*.json \
 	build \
 	src

# ports and volumes
EXPOSE 9120
VOLUME /config