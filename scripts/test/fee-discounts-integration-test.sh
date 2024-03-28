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

yes $PASSPHRASE | injectived tx exchange fee-discount-proposal --proposal fee-discount-proposal.json --deposit="1000000000000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS

VALIDATOR_ADDRESS=$(injectived q staking validators --output json  | jq -r '.validators[0].operator_address')

yes $PASSPHRASE | injectived tx staking delegate $VALIDATOR_ADDRESS 1000000000000000000inj $TX_OPTS

echo "Depositing inj"
yes $PASSPHRASE | injectived tx exchange deposit "1000000000000000inj" $TX_OPTS

echo "Depositing atom"
yes $PASSPHRASE | injectived tx exchange deposit "100000atom" $TX_OPTS

echo "Creating buy limit order"
yes $PASSPHRASE | injectived tx exchange create-spot-limit-order "buy" $Ticker 5000 $MinPriceTickSize $TX_OPTS

sleep 1

echo "Creating sell market order"
yes $PASSPHRASE | injectived tx exchange create-spot-market-order "sell" $Ticker 5000 $MinPriceTickSize $TX_OPTS
