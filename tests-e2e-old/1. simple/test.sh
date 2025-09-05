#!/usr/bin/env bash

set -euxo pipefail

TMP_FILE=$(mktemp)
DATE=$(date +%s)

echo
echo ">>> TEST 1.1 <<<"
echo
CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo
echo ">>> TEST 1.2 <<<"
echo
CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"

rm "$TMP_FILE"