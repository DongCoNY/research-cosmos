#!/bin/bash

PASSPHRASE="12345678"

EXPIRY=-1
TICKER="ATOM/INJ"
QUOTE_DENOM="inj"
ORACLE_BASE="atom"
ORACLE_QUOTE="inj"
ORACLE_TYPE="pricefeed"
ORACLE_SCALE_FACTOR=6
MIN_PRICE_TICK_SIZE="0.0001"
MIN_QUANTITY_TICK_SIZE="0.001"

TX_OPTS="--chain-id=injective-1 --broadcast-mode=sync --gas=2000000 --gas-prices=500000000inj --yes"
USER1="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r" #USER1=$(injectived keys show user1 -a)

echo " "
echo "🔥 Creating price-feeder-privilege-proposal 🔥"
echo " "
yes $PASSPHRASE | injectived tx oracle grant-price-feeder-privilege-proposal $ORACLE_BASE $ORACLE_QUOTE $USER1 --deposit=10000000inj --title="price feeder inj/atom" --description="price feeder inj/atom" --from=user1 $TX_OPTS
sleep 1

echo " "
echo "🔥 Voting on proposal 1 🔥"
echo " "
yes $PASSPHRASE | injectived tx gov vote 1 yes --from=genesis $TX_OPTS
sleep 15

echo " "
echo "🔥 Relaying pricefeed price 🔥"
echo " "
yes $PASSPHRASE | injectived tx oracle relay-price-feed-price $ORACLE_BASE $ORACLE_QUOTE 0.25 --from=user1 $TX_OPTS

echo " "
echo "🔥 Creating insurance fund 🔥"
echo " "
yes $PASSPHRASE | injectived tx insurance create-insurance-fund --ticker=$TICKER --quote-denom=$QUOTE_DENOM --oracle-base=$ORACLE_BASE --oracle-quote=$ORACLE_QUOTE --oracle-type=$ORACLE_TYPE --expiry=$EXPIRY --initial-deposit=10000000inj --from=genesis $TX_OPTS
sleep 1

yes $PASSPHRASE | injectived tx exchange instant-perpetual-market-launch \
  --ticker=$TICKER \
  --quote-denom=$QUOTE_DENOM \
  --oracle-base=$ORACLE_BASE \
  --oracle-quote=$ORACLE_QUOTE \
  --oracle-type=$ORACLE_TYPE \
  --oracle-scale-factor=$ORACLE_SCALE_FACTOR \
  --maker-fee-rate="0.001" \
  --taker-fee-rate="0.002" \
  --initial-margin-ratio="0.05" \
  --maintenance-margin-ratio="0.02" \
  --min-price-tick-size=$MIN_PRICE_TICK_SIZE \
  --min-quantity-tick-size=$MIN_QUANTITY_TICK_SIZE \
  --from=genesis \
  $TX_OPTS

