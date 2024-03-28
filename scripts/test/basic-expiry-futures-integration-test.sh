#!/bin/bash

set -e

PASSPHRASE="12345678"
TX_OPTS=" --chain-id=injective-1 --broadcast-mode=sync --yes"

EXPIRY=$((`date +%s`+60))
TICKER="ATOM/INJ"
QUOTE_DENOM="inj"
ORACLE_BASE="atom"
ORACLE_QUOTE="inj"
ORACLE_TYPE="pricefeed"

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

echo "🔥 Launching $TICKER expiry futures market 🔥"
yes $PASSPHRASE | injectived tx exchange instant-expiry-futures-market-launch \
			--ticker=$TICKER \
			--quote-denom=$QUOTE_DENOM \
			--oracle-base=$ORACLE_BASE \
			--oracle-quote=$ORACLE_QUOTE \
			--oracle-type=$ORACLE_TYPE \
			--oracle-scale-factor=0 \
			--expiry=$EXPIRY \
			--maker-fee-rate="0.001" \
			--taker-fee-rate="0.001" \
			--initial-margin-ratio="0.05" \
			--maintenance-margin-ratio="0.02" \
			--min-price-tick-size="0.0001" \
			--min-quantity-tick-size="0.001" \
			--from=user1 \
			$TX_OPTS

sleep 2