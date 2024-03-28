source ./util.sh

# calculate halt height
CUR_HEIGHT=$(curl -sS localhost:26657/block | jq .result.block.header.height | tr -d '"')
HALT_HEIGHT=$(($CUR_HEIGHT + 10))

# halt proposal
PROPOSAL_ID=10
yes 12345678 | ./5-1-injectived tx gov submit-proposal software-upgrade v1.6 \
--title "Injective Protocol canonical upgrade" \
--description "Injective Protocol canonical upgrade" \
--upgrade-height $HALT_HEIGHT \
--deposit 500000000000000000000inj \
--keyring-backend file \
--home n0 \
--from node0 \
--chain-id injective-1 \
--yes \
--broadcast-mode sync
vote $PROPOSAL_ID
sleep 20

# ORACLE
# grant provider privilege to 2 relayers
PROPOSAL_ID=11
export RELAYER1=$(yes 12345678 | ./6-1-injectived keys show -a signer1 --home n0)
export RELAYER2=$(yes 12345678 | ./6-1-injectived keys show -a signer2 --home n0)
yes 12345678 | ./6-1-injectived tx oracle grant-provider-privilege-proposal providerA $RELAYER1,$RELAYER2 \
--title "invoke provider" \
--description "send grant-provider-privilege-proposal" \
--deposit 500000000000000000000inj \
--keyring-backend file \
--home n0 \
--from node0 \
--chain-id injective-1 \
--yes \
--broadcast-mode sync
vote $PROPOSAL_ID
sleep 10

# revoke provider privilege from 1 relayer
PROPOSAL_ID=12
yes 12345678 | ./6-1-injectived tx oracle revoke-provider-privilege-proposal providerA $RELAYER1 \
--title "revoke provider" \
--description "send revoke-provider-privilege-proposal" \
--deposit 500000000000000000000inj \
--keyring-backend file \
--home n0 \
--from node0 \
--chain-id injective-1 \
--yes \
--broadcast-mode sync
vote $PROPOSAL_ID
sleep 10

# send provider data from invalid relayer
yes 12345678 | ./6-1-injectived tx oracle relay-provider-prices providerA super:2.17,nova:4.34 \
--from signer1 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj \
--yes \
--broadcast-mode sync

# send provider data from valid relayer 2 times
yes 12345678 | ./6-1-injectived tx oracle relay-provider-prices providerA super:2.17,nova:4.34 \
--from signer2 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj \
--yes \
--broadcast-mode sync

yes 12345678 | ./6-1-injectived tx oracle relay-provider-prices providerA super:2.17,nova:4.34 \
--from signer2 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj \
--yes \
--broadcast-mode sync

# EXCHANGE
# propose bo market
yes 12345678 | ./6-1-injectived tx exchange instant-binary-options-market-launch \
--ticker "UFC-KHABIB-TKO-05/30/2023" \
--quote-denom "peggy0xdAC17F958D2ee523a2206206994597C13D831ec7" \
--oracle-type "provider" \
--oracle-provider "providerA" \
--oracle-symbol "nova" \
--oracle-scale-factor "6" \
--maker-fee-rate "0.0005" \
--taker-fee-rate "0.0012" \
--expiry "1685460582" \
--settlement-time "1690730982" \
--min-price-tick-size "10000" \
--min-quantity-tick-size "0.001" \
--home n0 --from node0 --chain-id injective-1 \
--yes \
--broadcast-mode sync

# deposit to subaccount
yes 12345678 | ./6-1-injectived tx exchange deposit 10000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7 \
--home n0 --from node0 --chain-id injective-1 \
--gas-prices=500000000inj \
--yes \
--broadcast-mode sync

# place and cancel bo orders
yes 12345678 | ./6-1-injectived tx exchange create-binary-options-limit-order \
--market-id="0x8ca5df34bb14d7ead7b6a5470912dfae2c758d883896cc40dd5e1f324d46b99c" \
--subaccount-id="0x713f3d4cc940a33c8c339ab341ec217a38eb4ef4000000000000000000000000" \
--order-type="buy" \
--fee-recipient="inj10jmp6sgh4cc6zt3e8gw05wavvejgr5pw6m8j75" \
--price="10000" \
--quantity="10.25" \
--home n0 --from node0 --chain-id injective-1 \
--keyring-backend=file \
--yes \
--broadcast-mode sync

yes 12345678 | ./6-1-injectived tx exchange create-binary-options-limit-order \
--market-id="0x8ca5df34bb14d7ead7b6a5470912dfae2c758d883896cc40dd5e1f324d46b99c" \
--subaccount-id="0x713f3d4cc940a33c8c339ab341ec217a38eb4ef4000000000000000000000000" \
--order-type="sell" \
--fee-recipient="inj10jmp6sgh4cc6zt3e8gw05wavvejgr5pw6m8j75" \
--price="20000" \
--quantity="4" \
--home n0 --from node0 --chain-id injective-1 \
--keyring-backend=file \
--yes \
--broadcast-mode sync

# place bo buy market order
yes 12345678 | ./6-1-injectived tx exchange create-binary-options-market-order \
--market-id="0x8ca5df34bb14d7ead7b6a5470912dfae2c758d883896cc40dd5e1f324d46b99c" \
--subaccount-id="0x713f3d4cc940a33c8c339ab341ec217a38eb4ef4000000000000000000000000" \
--fee-recipient="inj10jmp6sgh4cc6zt3e8gw05wavvejgr5pw6m8j75" \
--price="10000" \
--quantity="5.75" \
--order-type="buy" \
--trigger-price="10.0" \
--home n0 --from node0 --chain-id injective-1 \
--keyring-backend=file \
--yes \
--broadcast-mode sync

# export CODE_CREATOR=$(yes 12345678 | ./6-1-injectived keys show -a node0 --home n0)
# PROPOSAL_ID=13
# yes 12345678 | ./6-1-injectived tx gov submit-proposal wasm-store artifacts/infiniteloop.wasm \
# --title "Store infiniteloop contract" \
# --description "Store infiniteloop contract" \
# --deposit 500000000000000000000inj \
# --from=node0 --home n0 --chain-id="injective-1" --fees=1500000000000000inj \
# --gas=8000000 \
# --run-as $CODE_CREATOR \
# --yes \
# --broadcast-mode sync
# vote $PROPOSAL_ID
#
# # instantiate contract
# CODE_ID=1
# INIT='{"receiver_address": "'$CODE_CREATOR'", "count": 7}'
# yes 12345678 | ./6-1-injectived tx wasm instantiate $CODE_ID $INIT --label="Injective Infinite Loop Contract" \
# --from=node0 --home n0 --chain-id="injective-1" \
# --fees=1500000000000000inj --gas=3000000 --no-admin \
# --yes \
# --broadcast-mode sync
#
# CONTRACT_ADDR=$(./6-1-injectived query wasm list-contract-by-code $CODE_ID --output json | jq -r '.contracts[-1]')
# BEGIN_BLOCKER_MSG='{"begin_blocker":{}}'
# yes 12345678 | injectived tx wasm execute $CONTRACT_ADDR $BEGIN_BLOCKER_MSG \
# --from=node0 --home n0 --chain-id="injective-1" \
# --fees=15000000000000000inj --gas=30000000 \
# --yes \
# --broadcast-mode sync
#
# QUERY_MSG='{"get_count":{}}'
# injectived query wasm contract-state smart $CONTRACT_ADDR $QUERY_MSG
#
# BEGIN_BLOCKER_MSG='{"begin_blocker":{}}'
# yes 12345678 | injectived tx wasm execute $CONTRACT_ADDR $BEGIN_BLOCKER_MSG \
# --from=node0 --home n0 --chain-id="injective-1" \
# --fees=15000000000000000inj --gas=30000000 \
# --dry-run \
# --broadcast-mode sync
#
# BEGIN_BLOCKER_MSG='{"begin_blocker":{}}'
# yes 12345678 | injectived tx wasm execute $CONTRACT_ADDR $BEGIN_BLOCKER_MSG \
# --from=node0 --home n0 --chain-id="injective-1" \
# --fees=25000000000000000inj --gas=5000000 \
# --yes \
# --broadcast-mode sync
