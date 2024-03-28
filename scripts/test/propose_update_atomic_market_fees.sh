
#!/bin/bash

PASSPHRASE="12345678"
Title="Atomic market orders fee multiplier update"
Description="XX"
MultiplierFee="5.00"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

MARKET_ID=$(injectived q exchange derivative-markets | grep market_id | cut -f2 -d ":" | head -n 1 | xargs)

echo $MARKET_ID

yes $PASSPHRASE | injectived tx exchange propose-atomic-fee-multiplier $MARKET_ID:$MultiplierFee \
			--title="$Title" \
			--description="$Description" \
			--deposit="100000000000inj" \
			$TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS

sleep 15
