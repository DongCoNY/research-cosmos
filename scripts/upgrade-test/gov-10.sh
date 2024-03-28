source ./util.sh

# calculate halt height
CHAIN_ID=injective-1
CUR_HEIGHT=$(curl -sS localhost:26657/block | jq .result.block.header.height | tr -d '"')
HALT_HEIGHT=$(($CUR_HEIGHT + 10))

# halt proposal
PROPOSAL_ID=16
yes 12345678 | ./7-1-injectived tx gov submit-proposal software-upgrade v1.8 \
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

ADDR1=inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r
ADDR2=inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs
yes 12345678 | injectived tx tokenfactory create-denom \
customcoin --from user1 \
--keyring-backend file \
--chain-id $CHAIN_ID \
--home n0 \
-y --broadcast-mode sync

yes 12345678 | injectived tx tokenfactory \
mint 1000000factory/$ADDR1/customcoin \
--from user1 --chain-id $CHAIN_ID --home n0 \
-y --broadcast-mode sync

# Set denom metadata via bank
# burn token
yes 12345678 | injectived tx tokenfactory \
burn 600000factory/$ADDR1/customcoin \
--from user1 --chain-id $CHAIN_ID --home n0 \
-y --broadcast-mode sync

# transfer
yes 12345678 | injectived tx bank \
send $ADDR1 $ADDR2 10000factory/$ADDR1/customcoin \
-y --chain-id $CHAIN_ID --broadcast-mode sync

# change new admin
yes 12345678 | injectived tx tokenfactory \
change-admin factory/$ADDR1/customcoin $ADDR2 \
--from user1 --home n0 -y --chain-id $CHAIN_ID --broadcast-mode sync

# now user2 can mint
yes 12345678 | injectived tx tokenfactory \
mint 2000000factory/$ADDR1/customcoin \
--from user2 --chain-id $CHAIN_ID --home n0 \
-y --broadcast-mode sync

# user2 can unset admin, no one can mint, inj1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqe2hm49 is 0x0
# https://github.com/cosmos/cosmos-sdk/issues/11020, can use ethToInjective cmd
yes 12345678 | injectived tx tokenfactory \
change-admin factory/$ADDR1/customcoin \
inj1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqe2hm49 \
--from user2 --home n0 -y --chain-id $CHAIN_ID --broadcast-mode sync
