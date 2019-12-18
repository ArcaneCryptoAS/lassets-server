#!/usr/bin/env bash
# this script downloads bitcoind for CI

# exit on error
set -e

BITCOIND_VERSION=0.19.0.1
BITCOIND_URL=https://bitcoincore.org/bin/bitcoin-core-$BITCOIND_VERSION/bitcoin-$BITCOIND_VERSION-x86_64-linux-gnu.tar.gz
BITCOIND_FILE=bitcoin-$BITCOIND_VERSION/bin/bitcoind

mkdir -p .binaries
cd .binaries

if [[ -f bitcoind ]]; then
  FOUND_VERSION=`./bitcoind -version`
  if [[ ${FOUND_VERSION:29:6} == $BITCOIND_VERSION ]]; then
    echo bitcoind exists with right version
    exit 0
  else
    echo bitcoind exists with wrong version, deleting and redownloading
    rm bitcoind
  fi
fi

curl -L $BITCOIND_URL | tar xvz $BITCOIND_FILE --strip-components 2
