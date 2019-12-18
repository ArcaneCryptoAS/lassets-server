#! /usr/bin/env bash

# exit on error
set -e

# echo executed commands
set -o xtrace

BOB_IP=`docker inspect --format '{{json .NetworkSettings}}' la-bob-lnd | jq .Networks.server_default.IPAddress --raw-output`
BOB_PUBKEY=`./lnd-bob getinfo | jq --raw-output .identity_pubkey`
ALICE_IP=`docker inspect --format '{{json .NetworkSettings}}' la-alice-lnd | jq .Networks.server_default.IPAddress --raw-output`

ALICE_ADDR=`./lnd-alice newaddress p2wkh | jq --raw-output .address`
BOB_ADDR=`./lnd-bob newaddress p2wkh | jq --raw-output .address`

# give Alice some balance
./bitcoin-cli generatetoaddress 100 $ALICE_ADDR

sleep 5

# open channel from alice to bob with money on both sides
./lnd-alice openchannel --node_key $BOB_PUBKEY --connect $BOB_IP --local_amt 16000000 --push_amt 10000000

# confirm channel
./bitcoin-cli generatetoaddress 6 $ALICE_ADDR
