#!/bin/bash

PASSPHRASE="12345678"
TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

Ticker="ATOM/INJ"
BaseDenom="atom"
QuoteDenom="inj"
MinPriceTickSize="1000000000"
MinQuantityTickSize="0.01"

set -e

echo "Launching $Ticker spot market"
yes $PASSPHRASE | injectived tx exchange instant-spot-market-launch $Ticker $BaseDenom $QuoteDenom --min-price-tick-size="$MinPriceTickSize" --min-quantity-tick-size="$MinQuantityTickSize" $TX_OPTS

yes $PASSPHRASE | injectived tx exchange trading-reward-campaign-launch-proposal \
--spot-market-ids="0xfbd55f13641acbb6e69d7b59eb335dabe2ecbfea136082ce2eedaba8a0c917a3" \
--spot-market-weights="1.0" --max-epoch-rewards="3000000000000000000000inj" \
--title="Trade Rewards Campaign Proposal" --description="Trade Rewards Campaign Proposal" --deposit="1000000000000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS

echo "Depositing 10,000 INJ to the community pool "
yes $PASSPHRASE | injectived tx distribution fund-community-pool 10000000000000000000000inj $TX_OPTS

echo "Depositing inj"
yes $PASSPHRASE | injectived tx exchange deposit "100000000000inj" $TX_OPTS

echo "Depositing atom"
yes $PASSPHRASE | injectived tx exchange deposit "1atom" $TX_OPTS

echo "Creating buy limit order"
yes $PASSPHRASE | injectived tx exchange create-spot-limit-order "buy" $Ticker $MinQuantityTickSize $MinPriceTickSize $TX_OPTS

sleep 2

echo "Creating sell market order"
yes $PASSPHRASE | injectived tx exchange create-spot-market-order "sell" $Ticker $MinQuantityTickSize $MinPriceTickSize $TX_OPTS
