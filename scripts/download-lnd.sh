#!/usr/bin/env bash
# this script downloads LND for CI

# exit on error
set -e

LND_VERSION=0.8.1-beta
LND_SUFFIX=lnd-linux-386-v$LND_VERSION
LND_URL=https://github.com/lightningnetwork/lnd/releases/download/v$LND_VERSION/$LND_SUFFIX.tar.gz

mkdir -p .binaries
cd .binaries

if [[ -f lnd ]]; then
  FOUND_VERSION=`./lnd --version`
  if [[ ${FOUND_VERSION:12:10} == $LND_VERSION ]]; then
    echo lnd exists with right version
    exit 0
  else
    echo lnd exists with wrong version, deleting and redownloading
  fi
fi

curl -L $LND_URL | tar xvz $LND_SUFFIX/lnd --strip-components 1
