
#!/bin/bash

PASSPHRASE="12345678"
Title="Derivative market params update"
Description="XX"
MarketID="0x000001"
InitialMarginRatio="0.01"
MaintenanceMarginRatio="0.01"
MakerFeeRate="0.01"
TakerFeeRate="0.01"
RelayerFeeShareRate="0.01"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx exchange update-derivative-market-params \
			--market-id="$MarketID" \
			--initial-margin-ratio="$InitialMarginRatio" \
			--maintenance-margin-ratio="$MaintenanceMarginRatio" \
			--maker-fee-rate="$MakerFeeRate" \
			--taker-fee-rate="$TakerFeeRate" \
			--relayer-fee-share-rate="$RelayerFeeShareRate" \
			--title="$Title" \
			--description="$Description" \
			--deposit="100000000000inj" \
			$TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS
