#! /usr/bin/env bash

# exit on error
set -e

# echo executed commands
set -o xtrace

# get alices chan_id with bob
CHAN_ACTIVE=`./lnd-alice listchannels | jq .channels[0] | jq .active -r`

if [[ ${CHAN_ACTIVE} == false ]]; then
  echo channel found is not active
  exit 0
fi

# Select the first channel found that is active
CHAN_ID=`./lnd-alice listchannels | jq -r '[.channels | .[] | select(.active == true)][0] .chan_id'`

# opens a contract for 10 dollars
./alice opencontract --amount=10 --chanid=$CHAN_ID