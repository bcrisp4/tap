#!/usr/bin/env bash
# Smoke-test a built tap image: boot it, hit /healthz, subscribe a
# feed, wait for the poll, list entries, sanity-check the SPA shell
# and report the image size.
set -euo pipefail

IMAGE="${1:?usage: docker-smoke.sh <image>}"
NAME="tap-smoke-$$"
PORT="${TAP_SMOKE_PORT:-18080}"
TMPDIR=$(mktemp -d)
# distroless/static:nonroot runs as UID/GID 65532; bind-mounted host
# dirs need to be writable by that uid. World-writable is fine for an
# ephemeral tmpdir.
chmod 0777 "$TMPDIR"
trap 'docker rm -f "$NAME" >/dev/null 2>&1 || true; rm -rf "$TMPDIR"' EXIT

docker run -d --name "$NAME" \
	-v "$TMPDIR:/data" \
	-p "127.0.0.1:${PORT}:8080" \
	-e TAP_POLL_INTERVAL=2s \
	"$IMAGE" >/dev/null

# Wait for /healthz to come up.
for _ in {1..30}; do
	if curl -fsS "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done

curl -fsS "http://127.0.0.1:${PORT}/healthz" | grep -q '"ok"'
echo "✓ healthz"

curl -fsS -XPOST "http://127.0.0.1:${PORT}/api/v1/feeds" \
	-H 'Content-Type: application/json' \
	-d '{"feed_url":"https://jvns.ca/atom.xml","title":"jvns.ca"}' >/dev/null
echo "✓ subscribed"

# Wait up to 30 s for the poll to populate entries. `jq -er` exits
# non-zero on missing/null so we get a clear failure if the response
# shape ever drifts; the case-glob then guards the integer comparison
# from any stray non-numeric output.
TOTAL=0
for _ in {1..15}; do
	if RAW=$(curl -fsS "http://127.0.0.1:${PORT}/api/v1/entries?limit=1" \
		| jq -er '.pagination.total'); then
		case "$RAW" in
			''|*[!0-9]*) ;;
			*) TOTAL="$RAW"; [ "$TOTAL" -gt 0 ] && break ;;
		esac
	fi
	sleep 2
done
[ "$TOTAL" -gt 0 ] || { echo "✗ poll did not produce entries"; exit 1; }
echo "✓ poll produced $TOTAL entries"

# SPA shell.
curl -fsS "http://127.0.0.1:${PORT}/" | grep -q '<html' && echo "✓ SPA shell"

# Image size sanity (target ≤ 25 MB; hard cap 50 MB).
SIZE=$(docker image inspect "$IMAGE" -f '{{ .Size }}')
SIZE_MB=$(( SIZE / 1024 / 1024 ))
echo "  image size: ${SIZE_MB} MB"
if [ "$SIZE_MB" -gt 50 ]; then
	echo "✗ image is too large"
	exit 1
fi

echo "ok"
