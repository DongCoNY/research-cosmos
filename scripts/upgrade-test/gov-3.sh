source ./util.sh

# calculate halt height
CUR_HEIGHT=$(curl -sS localhost:26657/block | jq .result.block.header.height | tr -d '"')
HALT_HEIGHT=$(($CUR_HEIGHT + 10))

# halt proposal
PROPOSAL_ID=5
yes 12345678 | ./2-1-injectived tx gov submit-proposal software-upgrade v1.2 \
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
