#!/bin/bash

set -e

PASSPHRASE="12345678"

EXPIRY=$((`date +%s`+60))
TICKER="ATOM/INJ"
QUOTE_DENOM="inj"
ORACLE_BASE="atom"
ORACLE_QUOTE="inj"
ORACLE_TYPE="pricefeed"

Title="INJ expiry futures market"
Description="XX"
Ticker="INJ expiry futures"

TX_OPTS="--from=genesis --chain-id=injective-1 --broadcast-mode=sync --yes"

#USER1=$(injectived keys show user1 -a)
USER1="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r"

echo "🔥 Creating price-feeder-privilege-proposal 🔥"
yes $PASSPHRASE | injectived tx oracle grant-price-feeder-privilege-proposal $ORACLE_BASE $ORACLE_QUOTE $USER1 --deposit=10000000inj --title="price feeder inj/atom" --description="price feeder inj/atom" --from=user1 $TX_OPTS
sleep 2

echo "🔥 Voting on proposal 1 🔥"
yes $PASSPHRASE | injectived tx gov vote 1 yes --from=genesis $TX_OPTS
sleep 15

echo "🔥 Relaying pricefeed price 🔥"
yes $PASSPHRASE | injectived tx oracle relay-price-feed-price $ORACLE_BASE $ORACLE_QUOTE 0.1 --from=user1 $TX_OPTS

echo "🔥 Creating insurance fund 🔥"
yes $PASSPHRASE | injectived tx insurance create-insurance-fund --ticker=$TICKER --quote-denom=$QUOTE_DENOM --oracle-base=$ORACLE_BASE --oracle-quote=$ORACLE_QUOTE --oracle-type=$ORACLE_TYPE --expiry=$EXPIRY --initial-deposit=10000000inj --from=genesis $TX_OPTS
sleep 1

yes $PASSPHRASE | injectived tx exchange propose-expiry-futures-market \
  --ticker=$TICKER \
  --quote-denom=$QUOTE_DENOM \
  --oracle-base=$ORACLE_BASE \
  --oracle-quote=$ORACLE_QUOTE \
  --oracle-type=$ORACLE_TYPE \
  --oracle-scale-factor=0 \
  --expiry=$EXPIRY \
  --maker-fee-rate="0.001" \
  --taker-fee-rate="0.002" \
  --initial-margin-ratio="0.05" \
  --maintenance-margin-ratio="0.02" \
  --min-price-tick-size="0.0001" \
  --min-quantity-tick-size="0.001" \
  --title="INJ/ATOM expiry futures market" \
  --description="XX" \
  --deposit="100000000000inj" \
  $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 2 yes $TX_OPTS
sleep 15
