#!/bin/sh

set -e

v() {
  >&2 echo '[#]' $@
  $@
}

v go build .
v go test ./...
v install tanlnode /usr/bin/
v install tanlnode@.service /etc/systemd/system
