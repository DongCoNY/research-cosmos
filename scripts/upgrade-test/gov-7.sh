source ./util.sh

# calculate halt height
CUR_HEIGHT=$(curl -sS localhost:26657/block | jq .result.block.header.height | tr -d '"')
HALT_HEIGHT=$(($CUR_HEIGHT + 10))

# halt proposal
PROPOSAL_ID=9
yes 12345678 | ./4-1-injectived tx gov submit-proposal software-upgrade v1.5 \
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

# trigger new message MsgRegisterAsDMM
yes 12345678 | ./5-1-injectived tx exchange register-as-dmm inj10tq6q4p67prfmhmzmdwg7zwx66v0gpfdp6p5l5 \
--from signer1 --home n0 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync

# trigger new typed authz flow
# user1, user2 deposit INJ, USDT, USDC to their subaccounts
yes 12345678 | ./5-1-injectived tx exchange deposit 1000000000000000000inj  --from user1 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync
yes 12345678 | ./5-1-injectived tx exchange deposit 10000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7 --from user1 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync
yes 12345678 | ./5-1-injectived tx exchange deposit 10000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --from user1 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync
yes 12345678 | ./5-1-injectived tx exchange deposit 1000000000000000000inj  --from user2 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync
yes 12345678 | ./5-1-injectived tx exchange deposit 10000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7 --from user2 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync
yes 12345678 | ./5-1-injectived tx exchange deposit 10000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --from user2 --gas-prices=500000000inj --chain-id=injective-1 --yes --broadcast-mode sync

# user1 grants user3 ability to execute MsgCreateSpotLimitOrder
# only via subaccount 0xc6fe5d33615a1c52c08018c47e8bc53646a0e101000000000000000000000000
# only in markets [INJ/USDT]
yes 12345678 | ./5-1-injectived tx exchange authz \
inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz \
0xc6fe5d33615a1c52c08018c47e8bc53646a0e101000000000000000000000000 \
MsgCreateSpotLimitOrder \
0xa508cb32923323679f29a032c70342c147c17d0145625922b0ef22e955c844c0 \
--expiration 1679733799 \
--from user1 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj

# user2 grants user3 ability to execute MsgCreateSpotLimitOrder
# only via subaccount 0xc6fe5d33615a1c52c08018c47e8bc53646a0e101000000000000000000000000
# only in markets [INJ/USDT, INJ/USDC]
yes 12345678 | ./5-1-injectived tx exchange authz \
inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz \
0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17000000000000000000000000 \
MsgCreateSpotLimitOrder \
0xa508cb32923323679f29a032c70342c147c17d0145625922b0ef22e955c844c0,0xfd30930cb70d176c37d0c405cde055e551c5b1116b7049a88bcf821766b62d62 \
--expiration 1679733799 \
--from user2 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj

# generate txs from user1, user2
./5-1-injectived tx exchange create-spot-limit-order sell INJ/USDT 1000000000000000 1000000 \
--from inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r --home n0 --chain-id=injective-1 --generate-only > tx0.json
./5-1-injectived tx exchange create-spot-limit-order sell INJ/USDC 1000000000000000 1000000 \
--from inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs --home n0 --chain-id=injective-1 --generate-only > tx1.json

# exec txs from user3
yes 12345678 | ./5-1-injectived tx authz exec tx0.json \
--from user3 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj
yes 12345678 | ./5-1-injectived tx authz exec tx1.json \
--from user3 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj

# trigger generic authz flow
# generate tx for INJ/USDC market from user1
./5-1-injectived tx exchange create-spot-limit-order sell INJ/USDC 1000000000000000 1000000 \
--from inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r --home n0 --chain-id=injective-1 --generate-only > tx2.json

# user3 cannot exec order in INJ/USDC according to typed authz rules above
yes 12345678 | ./5-1-injectived tx authz exec tx2.json \
--from user3 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj

# user1 grants user3 generic authz for MsgCreateSpotLimitOrder without any restrictions
yes 12345678 | ./5-1-injectived tx authz grant \
inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz generic \
--msg-type /injective.exchange.v1beta1.MsgCreateSpotLimitOrder \
--from user1 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj

# user3 can exec order in INJ/USDC now
yes 12345678 | ./5-1-injectived tx authz exec tx2.json \
--from user3 --home n0 --chain-id=injective-1 --yes --broadcast-mode sync --gas-prices=500000000inj
