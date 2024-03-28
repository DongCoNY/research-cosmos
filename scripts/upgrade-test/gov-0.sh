source ./util.sh

mkdir -p logs

# propose oracle
PROPOSAL_ID=1
yes 12345678 | ./1-7-injectived tx oracle grant-price-feeder-privilege-proposal \
"inj" "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" "inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r" \
--title "grant price feeder privilege" \
--description "XX" \
--deposit "500000000000000000000inj" \
--home n0 \
--from user1 \
--keyring-backend file \
--chain-id injective-1 --broadcast-mode sync \
--yes
vote $PROPOSAL_ID

# launch spot market
yes 12345678 | ./1-7-injectived tx exchange instant-spot-market-launch \
"INJ/USDT" "inj" "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" \
--min-price-tick-size 0.000000000000001 \
--min-quantity-tick-size 1000000000000000 \
--home n0 --from node0 --keyring-backend file --yes \
--chain-id injective-1 --broadcast-mode sync

yes 12345678 | ./1-7-injectived tx exchange instant-spot-market-launch \
"INJ/USDC" "inj" "peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599" \
--min-price-tick-size 0.000000000000001 \
--min-quantity-tick-size 1000000000000000 \
--home n0 --from node0 --keyring-backend file --yes \
--chain-id injective-1 --broadcast-mode sync

# launch derv insurance fund
yes 12345678 | ./1-7-injectived tx insurance create-insurance-fund \
--ticker "BTC/USDT PERP" \
--quote-denom "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" \
--oracle-base "BTC" \
--oracle-quote "USD" \
--oracle-type "coinbase" \
--expiry "-1" \
--initial-deposit "1000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" \
--home n0 --from node0 --keyring-backend file --yes \
--chain-id injective-1 --broadcast-mode sync

cwd=$PWD
startServices

cd $cwd
# launch derv market
yes 12345678 | ./1-7-injectived tx exchange instant-perpetual-market-launch \
--ticker="BTC/USDT PERP" \
--quote-denom "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" \
--oracle-base "BTC" \
--oracle-quote "USD" \
--oracle-type "coinbase" \
--oracle-scale-factor "6" \
--maker-fee-rate "0.001" \
--taker-fee-rate "0.001" \
--initial-margin-ratio "0.095" \
--maintenance-margin-ratio "0.05" \
--min-price-tick-size "100000" \
--min-quantity-tick-size "0.0001" \
--home n0 \
--from node0 \
--keyring-backend file \
--chain-id injective-1 \
--broadcast-mode sync \
--yes
