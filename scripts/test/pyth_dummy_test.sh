#!/bin/bash

PASSPHRASE="12345678"
USER1="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r"

ARCH=""

if [[ $(arch) = "arm64" ]]; then
  ARCH=-aarch64
fi

echo " "
echo "📀 Store dummy contract ..."
echo " "

yes $PASSPHRASE | injectived tx wasm store ../cw-injective/artifacts/dummy${ARCH}.wasm --from=genesis --chain-id=injective-1 --broadcast-mode=sync --gas=5000000 --fees=5000000000000000inj --yes


DUMMY_CODE_ID=1

echo " "
echo "🥷 Instantiate dummy..."
echo " "

INIT='{}'
yes $PASSPHRASE | injectived tx wasm instantiate $DUMMY_CODE_ID $INIT --label="Injective Dummy Contract" --from=genesis --chain-id="injective-1" --yes --fees=3500000000000000inj --gas=4000000 --no-admin
sleep 5
DUMMY_ADDRESS=$(injectived query wasm list-contract-by-code $DUMMY_CODE_ID --output json | jq -r '.contracts[-1]')
echo "DUMMY_ADDRESS=$DUMMY_ADDRESS"

if [ "$DUMMY_ADDRESS" == "null" ]; then
    echo "DUMMY_ADDRESS is unset, initialization failed?"
    exit 1
fi

echo " "
echo " Create a proposal json file "
echo " "

JSON="{
        \"title\": \"Update pyth oracle contract address\",
        \"description\": \"Update pyth oracle contract address\",
        \"changes\": [
          {
            \"subspace\": \"oracle\",
            \"key\": \"PythContract\",
            \"value\": \"$DUMMY_ADDRESS\"
          }
        ],
        \"deposit\": \"50000000000000000000inj\"
      }"

echo $JSON  > oracle_params.json

echo " "
echo "✉️ Send governance proposal to use dummy as pyth provider..."
echo " "
yes $PASSPHRASE | injectived tx gov submit-proposal param-change oracle_params.json --chain-id=injective-1 --broadcast-mode=sync --yes  --from genesis --fees 1000000000000000000inj
sleep 5

echo " "
echo "✉️ Vote on governance proposal to update pyth contract address.."
echo " "
yes $PASSPHRASE | injectived tx gov vote 1 yes --from genesis --chain-id=injective-1 --keyring-backend=file --yes --node=tcp://localhost:26657 --fees 100000000000000inj
sleep 5

rm oracle_params.json

SET_PRICE='{"trigger_pyth_update":{"price":134521}}'
echo "SET_PRICE=$SET_PRICE"

yes $PASSPHRASE | injectived tx wasm execute $DUMMY_ADDRESS $SET_PRICE --from genesis --chain-id="injective-1" --yes --fees=3500000000000000inj --gas=4000000 --from $USER1

sleep 5
echo " "
echo "✉️ Query pyth states - general and by id (should give same results - price 134.521)"
echo " "
injectived query oracle pyth-price --chain-id="injective-1"

injectived query oracle pyth-price f9c0172ba10dfa4d19088d94f5bf61d3b54d5bd7483a322a982e1373ee8ea31b --chain-id="injective-1"

echo " "
echo "✉️ Update pyth states - to 99.999"
echo " "
injectived query oracle pyth-price --chain-id="injective-1"


SET_PRICE='{"trigger_pyth_update":{"price":99999}}'
echo "SET_PRICE=$SET_PRICE"

yes $PASSPHRASE | injectived tx wasm execute $DUMMY_ADDRESS $SET_PRICE --from genesis --chain-id="injective-1" --yes --fees=3500000000000000inj --gas=4000000 --from $USER1

sleep 5
echo " "
echo "✉️ Query pyth states - general and by id (should give same results - price 99.999)"
echo " "
injectived query oracle pyth-price --chain-id="injective-1"

injectived query oracle pyth-price f9c0172ba10dfa4d19088d94f5bf61d3b54d5bd7483a322a982e1373ee8ea31b --chain-id="injective-1"
