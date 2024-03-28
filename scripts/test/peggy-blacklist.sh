#!/bin/bash

PASSPHRASE="12345678"
Description="Add Blacklist Address to Peggy"
Title="Add Blacklist Address to Peggy"

TX_OPTS="--from=genesis  --chain-id=injective-1 --broadcast-mode=sync --yes --fees=100000000000000000inj --gas=2500000"

Description="XX"
Title="Add Blacklist Address to Peggy"

yes $PASSPHRASE | injectived tx peggy blacklist-ethereum-addresses-proposal 0xd882cfc20f52f2599d84b8e8d58c7fb62cfe344b,0xe7aa314c77f4233c18c6cc84384a9247c0cf367b \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS


yes $PASSPHRASE | injectived tx peggy revoke-blacklist-ethereum-addresses-proposal 0xe7aa314c77f4233c18c6cc84384a9247c0cf367b \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 2 yes $TX_OPTS