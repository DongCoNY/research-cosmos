#!/bin/bash

PASSPHRASE="12345678"
Title="INJ/ATOM market"
Description="XX"
Ticker="INJ/ATOM"
BaseDenom="inj"
QuoteDenom="atom"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx exchange instant-spot-market-launch "$Ticker" "$BaseDenom" "$QuoteDenom" $TX_OPTS