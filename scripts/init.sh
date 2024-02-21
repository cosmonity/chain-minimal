#!/usr/bin/env bash

rm -r ~/.minid || true
# Get the value of GOPATH
GOBIN=$(go env GOPATH)/bin

# Check if minid exists in the GOBIN directory
MINID_BIN=$(which $GOBIN/minid)

if [ -n "$MINID_BIN" ]; then
    echo "minid binary found at: $MINID_BIN"
else
    echo "minid binary not found in $GOBIN directory."
fi

# configure minid
$MINID_BIN config set client chain-id demo
$MINID_BIN config set client keyring-backend test
$MINID_BIN keys add alice
$MINID_BIN keys add bob
$MINID_BIN init test --chain-id demo --default-denom mini
# update genesis
$MINID_BIN genesis add-genesis-account alice 10000000mini --keyring-backend test
$MINID_BIN genesis add-genesis-account bob 1000mini --keyring-backend test
# create default validator
$MINID_BIN genesis gentx alice 1000000mini --chain-id demo
$MINID_BIN genesis collect-gentxs
