#!/bin/sh
# Rename the project: change the Go module path (and every import of it) and
# the short project name used for the Docker image tag and MIME boundaries.
#
#   scripts/rename.sh github.com/acme/widgets          # short name: widgets
#   scripts/rename.sh github.com/acme/widgets widgets-api
#
# The current module path is read from go.mod, so the script can be run
# again later to rename a second time. Files under old/ are never touched.
set -eu

usage() {
	echo "usage: $0 <new-module-path> [<new-short-name>]" >&2
	exit 2
}

[ $# -ge 1 ] && [ $# -le 2 ] || usage
new_module=$1
new_name=${2:-${new_module##*/}}

cd "$(dirname "$0")/.."

old_module=$(sed -n 's/^module //p' go.mod)
# The short name is whatever the Docker image tag currently says; the module
# path cannot tell (a previous rename may have chosen a different name).
old_name=$(sed -n 's/.*docker build -t \([a-z0-9-]*\):local .*/\1/p' Makefile)
[ -n "$old_name" ] || { echo "could not read the current short name from the Makefile docker-build target" >&2; exit 1; }

case "$new_module" in
	*" "*|"") echo "invalid module path: $new_module" >&2; exit 2 ;;
esac
case "$new_name" in
	*[!a-z0-9-]*|"") echo "short name must be lowercase [a-z0-9-]: $new_name" >&2; exit 2 ;;
esac
if [ "$new_module" = "$old_module" ] && [ "$new_name" = "$old_name" ]; then
	echo "already named $old_module ($old_name); nothing to do"
	exit 0
fi

if command -v git >/dev/null 2>&1 && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
	echo "warning: working tree has uncommitted changes; review the result with git diff" >&2
fi

echo "module: $old_module -> $new_module"
echo "name:   $old_name -> $new_name"

# Go source files outside old/ (import paths) and go.mod (module line).
go_files=$(find . -name '*.go' -not -path './old/*' -not -path './.git/*')
perl -pi -e '
	s{^module \Q'"$old_module"'\E$}{module '"$new_module"'};
' go.mod
# shellcheck disable=SC2086
perl -pi -e '
	s{"\Q'"$old_module"'/}{"'"$new_module"'/}g;
	s{"\Q'"$old_module"'"}{"'"$new_module"'"}g;
' $go_files

# The short name: Docker image tag and the MIME boundary prefix.
perl -pi -e 's{\b\Q'"$old_name"'\E:(local|ci)\b}{'"$new_name"':$1}g' Makefile .github/workflows/ci.yml
perl -pi -e 's{"=_\Q'"$old_name"'\E_"}{"=_'"$new_name"'_"}g' internal/platform/email/smtp.go

go mod tidy
gofmt -l $(find . -name '*.go' -not -path './old/*') | sed 's/^/gofmt needed: /'
go build ./...
go vet ./...

echo "done. Remaining mentions of the old name (review by hand):"
grep -rn --exclude-dir=old --exclude-dir=.git --exclude-dir=bin -I "$old_name" . | grep -vE '^./(go.sum|scripts/rename.sh)' || echo "  none"
