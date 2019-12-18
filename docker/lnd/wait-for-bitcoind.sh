#!/usr/bin/env bash

case $BITCOIN_NETWORK in
"mainnet")
    PORT=8332
    ;;
"testnet")
    PORT=18332
    ;;
"regtest")
    PORT=18443
    ;;
"")
    echo "BITCOIN_NETWORK is not set!"
    exit 1
esac

ATTEMPTS=50
SLEEP_DURATION=1
URL="http://$RPCUSER:$RPCPASS@$RPCHOST:$PORT"
SUCCESS=0
echo Trying to get contact with Bitcoin Core at $URL
for i in $(seq 1 $ATTEMPTS); do
    RESULT=`curl -s -o /dev/null -w '%{http_code}\n' $URL \
      --data-binary '{"jsonrpc": "1.0", "method": "getblockchaininfo", "params": [] }'`
    
    if [[ "$RESULT" -eq "200" ]]; then
        echo Got contact with Bitcoin Core!
        SUCCESS=1 
        break
    fi
    sleep $SLEEP_DURATION
done

if [[ "$SUCCESS" -eq "0" ]]; then 
    echo "Couldn't reach bitcoind after $ATTEMPTS attempts and sleeping ${SLEEP_DURATION}s each time"
    exit 1
fi