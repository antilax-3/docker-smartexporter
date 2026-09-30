#!/usr/bin/env bash
set -u

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/../libs/common.sh"

resolve_image "${VARIANT}"
resolve_platform_image "${PLATFORM}" || exit 1

case "${PLATFORM}" in
  amd64) APK_ARCH="x86_64"; ELF_MACHINE="62" ;;
  arm64) APK_ARCH="aarch64"; ELF_MACHINE="183" ;;
  armv7) APK_ARCH="armv7"; ELF_MACHINE="40" ;;
esac

# The variants differ in libc and in the interpreter smartctl is linked against. Wolfi also ships no getent, so the user
# database is read out of /etc/passwd, which both bases have.
case "${VARIANT}" in
  wolfi) OS_ID="wolfi"; LIBC="glibc"; INTERPRETER="/lib/ld-linux-*" ;;
  alpine) OS_ID="alpine"; LIBC="musl"; INTERPRETER="/lib/ld-musl-*" ;;
esac

REVISION="${BUILDKITE_COMMIT}"
MARKER="__TEST_OUTPUT__"
FAILURES=0

# Runs a shell script inside the container through /init and with-contenv, the same way the image's services run, and
# returns only the script's output. Every line the script prints is prefixed with a marker and only marked lines are
# kept, because the s6 startup banner and the smart-exporter service share the container's stdout, and under emulation
# the service's log lines land in the middle of the script's output.
run() {
  local options="$1" script="$2"
  # shellcheck disable=SC2086 # options holds multiple docker run flags and must be word split.
  docker run --rm --platform "${DOCKER_PLATFORM}" ${options} "${PLATFORM_IMAGE}" /command/with-contenv sh -c "( ${script} ) | sed 's/^/${MARKER} /'" 2> /dev/null | sed -n "s/^${MARKER} //p"
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

# Waits for the smart-exporter service to answer on port 9120. With no config mounted, the service's first start falls
# back to the default config and writes it to /config.
READY="for i in \$(seq 1 40); do wget -q -O /dev/null http://localhost:9120/metrics && break; sleep 0.5; done"

# The container has no disks, so smartctl is replaced by a script answering from the go tests' fixtures: an ATA drive
# at /dev/sda and a SAS drive at /dev/sdb. SCRAPED waits for the first scrape to be served and prints the name and
# value of every smartexporter sample, in the order the service serves them.
FAKE_SMARTCTL="-v ${REPOSITORY_ROOT}/.buildkite/steps/testdata/smartctl:/usr/local/bin/smartctl:ro -v ${REPOSITORY_ROOT}/internal/smartctl/testdata:/testdata:ro"
SCRAPED="for i in \$(seq 1 40); do M=\$(wget -q -O - http://localhost:9120/metrics 2> /dev/null | grep '^smartexporter_'); [ -n \"\${M}\" ] && break; sleep 0.5; done; echo \"\${M}\""

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

echo "--- :floppy_disk: SMART Exporter"
check "smartctl runs" "valid" "$(run "" "smartctl --version | head -n1 | grep -qE '^smartctl [0-9]+\.[0-9]+ ' && echo valid")"
check "smartexporter is installed" "/app/smartexporter" "$(run "" "ls /app/smartexporter")"
check "smartexporter is built for ${APK_ARCH}" "${ELF_MACHINE}" "$(run "" "od -An -tu2 -j18 -N2 /app/smartexporter" | xargs)"
check "port 9120 is exposed" '{"9120/tcp":{}}' "$(docker image inspect -f '{{json .Config.ExposedPorts}}' "${PLATFORM_IMAGE}")"
check "/config is a volume" '{"/config":{}}' "$(docker image inspect -f '{{json .Config.Volumes}}' "${PLATFORM_IMAGE}")"
check "default config is written to /config on first start, owned by abc" "abc 644 1" \
  "$(run "" "${READY} > /dev/null; echo \$(stat -c '%U %a' /config/smartexporter.json) \$(grep -c '\"scrapeInterval\": 10' /config/smartexporter.json)")"
check "smart-exporter service serves the default config's attributes of the ATA drive on port 9120" \
  "smartexporter_airflow_temperature=31 smartexporter_lbas_read=1.2345678901e+10 smartexporter_lbas_written=5.4321098765e+10 smartexporter_temperature=33" \
  "$(run "${FAKE_SMARTCTL}" "${SCRAPED} | sed -E 's/\{.*\} /=/'" | xargs)"
check "smart-exporter service labels a sample with the configured information fields" \
  'smartexporter_temperature{device="/dev/sda",device_model="WDC WD40EFRX-68N32N0",serial_number="WD-WCC7K1234567"} 33' \
  "$(run "${FAKE_SMARTCTL}" "${SCRAPED} | grep '^smartexporter_temperature'")"
check "smart-exporter service answers on / with a pointer to the metrics" "HTTP/1.1 200 OK" \
  "$(run "" "${READY} > /dev/null; wget -S -O /dev/null http://localhost:9120/ 2>&1 | grep -m1 -o 'HTTP/1\.[01] [0-9]* [A-Za-z ]*'")"
# Under emulation the process's command line starts with the qemu interpreter, so only its tail is matched.
check "smart-exporter service runs as abc" "abc" \
  "$(run "" "${READY} > /dev/null; for p in /proc/[0-9]*; do case \"\$(tr '\\0' ' ' < \${p}/cmdline 2> /dev/null)\" in *'/app/smartexporter ') stat -c %U \${p} ;; esac; done")"

if [[ ${FAILURES} -gt 0 ]]; then
  echo "^^^ +++"
  echo "${FAILURES} check(s) failed"
  exit 1
fi

echo "All checks passed"
