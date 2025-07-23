#!/usr/bin/env bash

set -euxo pipefail

TMP_FILE=$(mktemp)
DATE=$(date +%s)

cd ignored-files

CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG} ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: false" "$TMP_FILE"

echo $RANDOM > random

CMD="mimosa remember -- docker buildx build --build-arg DATE=${DATE} --file ../Dockerfile --platform linux/amd64,linux/arm64 --push -t ${FULL_IMAGE_TAG}-v2 ."
$CMD 2>&1 | tee "$TMP_FILE"
grep -q "mimosa-cache-hit: true" "$TMP_FILE"

rm "$TMP_FILE"