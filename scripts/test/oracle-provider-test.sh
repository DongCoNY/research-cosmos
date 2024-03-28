#!/bin/bash

set -e

PASSPHRASE="12345678"
TX_OPTS=" --chain-id=injective-1 --broadcast-mode=sync --yes --fees=1500000000000000inj"

USER1=$(injectived keys show user1 -a)
PROVIDER="provider1"
RELAYER2="inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"

echo "🔥Granting provider privilege 🔥"
yes $PASSPHRASE | injectived tx oracle grant-provider-privilege-proposal $PROVIDER $USER1,$RELAYER2 --deposit=10000000inj   --title="grant provider privilege" --description="grant provider privilege" --from "$USER1" $TX_OPTS
sleep 2

echo "🔥 Voting on proposal 1 🔥"
yes $PASSPHRASE | injectived tx gov vote 1 yes --from=genesis $TX_OPTS
sleep 15

echo "🔥Posting prices 🔥"
yes $PASSPHRASE | injectived tx oracle relay-provider-prices $PROVIDER barmad:1,manliv:0 --from="$USER1" $TX_OPTS

echo "🔥Checking provider info 🔥"
injectived q oracle providers-info

echo "🔥Checking all provider prices 🔥"
injectived q oracle provider-prices

echo "🔥Checking specific provider prices 🔥"
injectived q oracle provider-prices provider1

echo "🔥Revoking provider privilege 🔥"
yes $PASSPHRASE | injectived tx oracle revoke-provider-privilege-proposal $PROVIDER $RELAYER2 --deposit=10000000inj   --title="revoke provider relayers" --description="revoke provider relayers privilege" --from "$USER1" $TX_OPTS
sleep 2

echo "🔥 Voting on proposal 2 🔥"
yes $PASSPHRASE | injectived tx gov vote 2 yes --from=genesis $TX_OPTS
sleep 15

echo "🔥Checking provider info 🔥"
injectived q oracle providers-info