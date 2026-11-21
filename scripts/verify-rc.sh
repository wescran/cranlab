#!/usr/bin/env bash
# Run from a clean candidate checkout. Requires Go, a local Docker engine and Compose.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
if [[ -n $(git status --porcelain) ]]; then
  echo 'Commit or remove local changes before verifying a release candidate.' >&2
  exit 1
fi
endpoint=${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}
[[ $endpoint == unix://* ]] || { echo "Local Docker socket required" >&2; exit 1; }
sha=$(git rev-parse HEAD)
printf 'Verifying %s at %s\n' "$(git branch --show-current)" "$sha"
tmp=$(mktemp -d)
# Archive only the named commit: no ignored config, local cache or credentials.
git archive "$sha" | tar -x -C "$tmp"
cd "$tmp"
project="cranlab-rc-$(basename "$tmp" | tr '[:upper:].' '[:lower:]-')"
compose() { docker compose --project-name "$project" -f scripts/rc/compose.yml "$@"; }
cleanup() {
  if [[ $? != 0 ]]; then compose logs --no-color site pds >&2 || true; fi
  compose down --volumes --rmi local >/dev/null || true
  docker image rm "$project-builder" >/dev/null 2>&1 || true
  cd / && rm -rf "$tmp"
}
trap cleanup EXIT

export GOTOOLCHAIN="go$(awk '/^go / {print $2}' go.mod)"
go version
go vet ./...
go test -race -count=1 ./...
GOOS=linux GOARCH=amd64 go build ./...

# Test inside the production builder, with the dependencies it downloaded.
docker build --target build -t "$project-builder" .
linux_go=$(docker run --rm --network none "$project-builder" go env GOVERSION)
[[ $linux_go == "$GOTOOLCHAIN" ]] || { echo "Unexpected builder: $linux_go" >&2; exit 1; }
docker run --rm --network none "$project-builder" go test -count=1 ./...

compose build site
compose up --wait -d pds
compose up -d site
site=$(compose ps -q site)
user=$(docker inspect -f '{{.Config.User}}' "$site")
case ${user%%:*} in 65532|nonroot) ;; *) echo "Unexpected site user: $user" >&2; exit 1;; esac
printf 'Local image %s, user %s, cold cache:\n' "$(docker inspect -f '{{.Image}}' "$site")" "$user"
compose run --rm --no-deps pds python /fixture.py check

compose stop site pds
compose up -d --force-recreate site
echo 'New site container, existing volume, PDS stopped:'
compose run --rm --no-deps pds python /fixture.py check
printf 'RC verification passed for %s\n' "$sha"
