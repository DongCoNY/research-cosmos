#!/bin/bash

PASSPHRASE="12345678"

EXPIRY=-1
TICKER="UFC-KHABIB-TKO-05/30/2023"
QUOTE_DENOM="inj"
ORACLE_BASE="atom"
ORACLE_QUOTE="inj"
ORACLE_TYPE="pricefeed"
ORACLE_SCALE_FACTOR=6
MIN_PRICE_TICK_SIZE="0.0001"
MIN_QUANTITY_TICK_SIZE="0.001"

TX_OPTS="--chain-id=injective-1 --broadcast-mode=sync --gas=2000000 --gas-prices=500000000inj --yes"
USER1="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r"
#USER1=$(injectived keys show user1 -a)

PROVIDER="provider1"
RELAYER2="inj1l0zxkd8tkam0tvg68uqh7xvym79mtw8329vd43"

alias e2i="injectived q exchange inj-address-from-eth-address"
alias i2e="injectived q exchange eth-address-from-inj-address"

MASTER_ETHEREUM_ADDRESS=$(injectived q exchange eth-address-from-inj-address $USER1) # | tr "[:upper:]" "[:lower:]"
#SUBACCOUNT_ID=${MASTER_ETHEREUM_ADDRESS}000000000000000000000000
SUBACCOUNT_ID="0xc6fe5d33615a1c52c08018c47e8bc53646a0e101000000000000000000000000"

echo "🔥Subaccount ID:  🔥"
echo $SUBACCOUNT_ID

#USER_SUBACCOUNT_ID="0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17000000000000000000000000"

echo "🔥Granting provider privilege 🔥"
yes $PASSPHRASE | injectived tx oracle grant-provider-privilege-proposal $PROVIDER $USER1,$RELAYER2 --deposit=10000000inj --title="grant provider privilege" --description="grant provider privilege" --from "$USER1" $TX_OPTS
sleep 2

echo "🔥 Voting on proposal 1 🔥"
yes $PASSPHRASE | injectived tx gov vote 1 yes --from=genesis $TX_OPTS
sleep 15

echo " "
echo "🔥 Creating instant binary options market launch 🔥"
echo " "
yes $PASSPHRASE | injectived tx exchange instant-binary-options-market-launch \
  --ticker=$TICKER \
  --quote-denom="inj" \
  --oracle-symbol=$TICKER \
  --oracle-provider=$PROVIDER \
  --oracle-type="provider" \
  --oracle-scale-factor="6" \
  --maker-fee-rate="0.0005" \
  --taker-fee-rate="0.0012" \
  --expiry="1685460582" \
  --admin=$USER1 \
  --settlement-time="1690730982" \
  --min-price-tick-size="10000" \
  --min-quantity-tick-size="0.001" \
  --from=genesis \
  --keyring-backend=file \
  --yes \
  $TX_OPTS
sleep 1

MARKET_ID=$(injectived q exchange binary-options-markets | grep market_id | cut -f2 -d ":" | xargs)

echo " "
echo "🔥 Posting binary options market admin update 🔥"
echo " "

yes $PASSPHRASE | injectived tx exchange admin-update-binary-options-market \
  --market-id=$MARKET_ID \
  --settlement-time="1690790000" \
  --expiration-time="1685460582" \
  --from $USER1 --keyring-backend=file \
  --yes \
  $TX_OPTS

echo " "
echo "🔥 Posting deposit to subaccount 🔥"
yes $PASSPHRASE | injectived tx exchange deposit 100000000inj \
  --from $USER1 \
  --keyring-backend=file \
  --yes \
  $TX_OPTS
echo " "
sleep 5

echo " "
echo "🔥 Posting binary options limit order 🔥"
echo " "

yes $PASSPHRASE | injectived tx exchange create-binary-options-limit-order \
  --order-type="buy" \
  --market-id=$MARKET_ID \
  --subaccount-id=${SUBACCOUNT_ID} \
  --fee-recipient=$USER1 \
  --price="70000" \
  --quantity="100.0" \
  --from $USER1 --keyring-backend=file \
  --yes \
  $TX_OPTS

echo " "
echo "🔥 Posting binary options post-only limit order 🔥"
echo " "

yes $PASSPHRASE | injectived tx exchange create-binary-options-limit-order \
  --order-type="buyPostOnly" \
  --market-id=$MARKET_ID \
  --subaccount-id=${SUBACCOUNT_ID} \
  --fee-recipient=$USER1 \
  --price="70000" \
  --quantity="50.0" \
  --from $USER1 --keyring-backend=file \
  --yes \
  $TX_OPTS

echo " "
echo "🔥 Cancelling binary options order 🔥"
echo " "
sleep 2

#hardcoded as it's difficult to find otherwise
TXHASH="0xae6211bcdda69049ac7946caa809b4923a6e2bbb34c4b1e86972714e6636516a"

yes $PASSPHRASE | injectived tx exchange cancel-binary-options-order \
  --market-id=$MARKET_ID \
  --subaccount-id=$SUBACCOUNT_ID \
  --order-hash=$TXHASH \
  --from $USER1 \
  --keyring-backend=file \
  --yes \
  $TX_OPTS

echo " "
echo "🔥 Posting binary options market order 🔥"
echo " "

yes $PASSPHRASE | injectived tx exchange create-binary-options-market-order \
  --order-type="sell" \
  --market-id=$MARKET_ID \
  --subaccount-id=$SUBACCOUNT_ID \
  --fee-recipient=$USER1 \
  --price="10000" \
  --quantity="30.0" \
  --from $USER1 --keyring-backend=file \
  --yes \
  $TX_OPTS

echo " "
echo "🔥 Posting settlement price and demolishing market 🔥"
echo " "

yes $PASSPHRASE | injectived tx exchange admin-update-binary-options-market \
  --market-id=$MARKET_ID \
  --settlement-price="1.0" \
  --settlement-time="1690790000" \
  --expiration-time="1685460582" \
  --market-status="demolished" \
  --from $USER1 \
  --keyring-backend=file \
  --yes \
  $TX_OPTS
