#!/usr/bin/env bash
set -u

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/../libs/common.sh"

resolve_image "${VARIANT}"
resolve_platform_image "${PLATFORM}" || exit 1

case "${PLATFORM}" in
  amd64) APK_ARCH="x86_64" ;;
  arm64) APK_ARCH="aarch64" ;;
  armv7) APK_ARCH="armv7" ;;
esac

# Wolfi ships no getent, so the user database is read out of /etc/passwd.
case "${VARIANT}" in
  wolfi) OS_ID="wolfi"; LIBC="glibc"; INTERPRETER="/lib/ld-linux-*" ;;
esac

REVISION="${BUILDKITE_COMMIT}"
MARKER="__TEST_OUTPUT__"
FAILURES=0

# Runs a shell script inside the container through /init and with-contenv, the same way the
# image's services run, and returns only the script's output (not the s6 startup banner).
run() {
  local options="$1" script="$2"
  # shellcheck disable=SC2086 # options holds multiple docker run flags and must be word split.
  docker run --rm --platform "${DOCKER_PLATFORM}" ${options} "${PLATFORM_IMAGE}" /command/with-contenv sh -c "echo ${MARKER}; ${script}" 2> /dev/null | sed "1,/^${MARKER}\$/d"
}

check() {
  local description="$1" expected="$2" actual="$3"

  if [[ "${actual}" == "${expected}" ]]; then
    echo "ok - ${description}"
  else
    echo "not ok - ${description}"
    echo "    expected: ${expected}"
    echo "    actual:   ${actual}"
    FAILURES=$((FAILURES + 1))
  fi
}

# Waits for the smart-exporter service to answer on port 9120 and prints the names of the smartexporter metrics it
# serves. With no config mounted, the service's first start falls back to the default config and writes it to /config.
# The container has no disks to scrape, so the checks cover the metrics the default config declares, not their values.
# The service logs to the same stdout as the script, so the checks that start it read only the script's last line.
READY="for i in \$(seq 1 40); do M=\$(wget -qO- http://localhost:9120/metrics 2> /dev/null | sed -n 's/^# HELP \(smartexporter_[a-z_]*\) .*/\1/p' | xargs); [ -n \"\${M}\" ] && echo \"\${M}\" && break; sleep 0.5; done"

echo "--- :label: Image metadata [${DOCKER_PLATFORM}]"
check "image platform is ${DOCKER_PLATFORM}" "${DOCKER_PLATFORM}" \
  "$(docker image inspect -f '{{.Os}}/{{.Architecture}}{{with .Variant}}/{{.}}{{end}}' "${PLATFORM_IMAGE}" | sed 's|^linux/arm64/v8$|linux/arm64|')"
check "entrypoint is /init" '["/init"]' "$(docker image inspect -f '{{json .Config.Entrypoint}}' "${PLATFORM_IMAGE}")"
check "version label is ${BUILD_TAG}" "${BUILD_TAG}" "$(docker image inspect -f '{{index .Config.Labels "version"}}' "${PLATFORM_IMAGE}")"
check "build_date label is set" "set" "$(docker image inspect -f '{{with index .Config.Labels "build_date"}}set{{end}}' "${PLATFORM_IMAGE}")"
check "OCI revision label is ${REVISION}" "${REVISION}" "$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${PLATFORM_IMAGE}")"
check "OCI source label is the GitHub repository" "https://github.com/${GITHUB_REPOSITORY}" \
  "$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.source"}}' "${PLATFORM_IMAGE}")"
check "OCI version label is ${BUILD_TAG}" "${BUILD_TAG}" "$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "${PLATFORM_IMAGE}")"
check "OCI created label is an RFC 3339 timestamp" "valid" \
  "$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.created"}}' "${PLATFORM_IMAGE}" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' && echo valid)"

echo "--- :package: Inherited base image [${VARIANT}]"
check "base is ${OS_ID}" "${OS_ID}" "$(run "" ". /etc/os-release; echo \${ID}")"
check "apk architecture is ${APK_ARCH}" "${APK_ARCH}" "$(run "" "apk --print-arch")"
check "libc is ${LIBC}" "found" "$(run "" "ls ${INTERPRETER} > /dev/null 2>&1 && echo found")"
check "abc passwd entry" "abc:911:911:/config:/bin/false" \
  "$(run "" "grep '^abc:' /etc/passwd | cut -d: -f1,3,4,6,7")"
check "abc is in the users group" "yes" "$(run "" "id -nG abc | tr ' ' '\\n' | grep -qx users && echo yes")"
check "container keeps s6 supervision" "0" "$(docker run --rm --platform "${DOCKER_PLATFORM}" "${PLATFORM_IMAGE}" true > /dev/null 2>&1; echo $?)"
check "node runs" "valid" "$(run "" "node --version | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$' && echo valid")"

echo "--- :floppy_disk: SMART Exporter"
check "smartctl runs" "valid" "$(run "" "smartctl --version | head -n1 | grep -qE '^smartctl [0-9]+\.[0-9]+ ' && echo valid")"
check "application bundle is installed" "/app/main.js" "$(run "" "ls /app/main.js")"
check "application sources and build output are removed" "" \
  "$(run "" "ls -d /app/src /app/build /app/package.json /app/package-lock.json 2> /dev/null" | xargs)"
check "port 9120 is exposed" '{"9120/tcp":{}}' "$(docker image inspect -f '{{json .Config.ExposedPorts}}' "${PLATFORM_IMAGE}")"
check "/config is a volume" '{"/config":{}}' "$(docker image inspect -f '{{json .Config.Volumes}}' "${PLATFORM_IMAGE}")"
check "default config is written to /config on first start, owned by abc" "abc 10" \
  "$(run "" "${READY} > /dev/null; echo \$(stat -c %U /config/smartexporter.json) \$(node -p 'require(\"/config/smartexporter.json\").scrapeInterval')" | tail -n1)"
check "smart-exporter service serves the default config's metrics on port 9120" \
  "smartexporter_temperature smartexporter_airflow_temperature smartexporter_lbas_written smartexporter_lbas_read" \
  "$(run "" "${READY}" | tail -n1)"
check "smart-exporter service runs as abc" "abc" \
  "$(run "" "${READY} > /dev/null; for p in /proc/[0-9]*; do [ \"\$(tr '\\0' ' ' < \${p}/cmdline 2> /dev/null)\" = 'node /app/main.js ' ] && stat -c %U \${p}; done" | tail -n1)"

if [[ ${FAILURES} -gt 0 ]]; then
  echo "^^^ +++"
  echo "${FAILURES} check(s) failed"
  exit 1
fi

echo "All checks passed"
