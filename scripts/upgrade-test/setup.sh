#!/bin/bash
source ./util.sh

set -e

killall ./injectived &>/dev/null || true
rm -rf ./n0 ./n1 ./gentx
mkdir -p logs

CHAINID="injective-1"
MONIKER="injective"
PASSPHRASE="12345678"
HOME0="./n0"
HOME1="./n1"

# Set moniker and chain-id for Ethermint (Moniker can be anything, chain-id must be an integer)
./1-7-injectived init $MONIKER --chain-id $CHAINID --home $HOME0
./1-7-injectived init $MONIKER --chain-id $CHAINID --home $HOME1

peer0="$(./1-7-injectived --home $HOME0 tendermint show-node-id)\@127.0.0.1:26656"
peer1="$(./1-7-injectived --home $HOME1 tendermint show-node-id)\@127.0.0.1:26666"

perl -i -pe 's/^timeout_commit = ".*?"/timeout_commit = "1000ms"/' $HOME0/config/config.toml
perl -i -pe 's|persistent_peers = ""|persistent_peers = "'$peer1'"|g' $HOME0/config/config.toml
# perl -i -pe 's|snapshot-interval = 0|snapshot-interval = 5|g' $HOME0/config/app.toml

perl -i -pe 's/^timeout_commit = ".*?"/timeout_commit = "1000ms"/' $HOME1/config/config.toml
perl -i -pe 's|persistent_peers = ""|persistent_peers = "'$peer0'"|g' $HOME1/config/config.toml

cat $HOME0/config/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="inj"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="inj"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="inj"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["gov"]["voting_params"]["voting_period"]="5s"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["mint"]["params"]["mint_denom"]="inj"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["auction"]["params"]["auction_period"]="10"' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json
cat $HOME0/config/genesis.json | jq '.app_state["oracle"]["band_ibc_oracle_requests"]=[{"request_id":"1","oracle_script_id":"23","symbols":["BTC","ETH","USDT","INJ","BNB","OSMO"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"},{"request_id":"2","oracle_script_id":"23","symbols":["LUNA","UST","SCRT","ATOM","CRO","STX"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"},{"request_id":"3","oracle_script_id":"23","symbols":["SOL","DOT","MATIC","XRP","AVAX","NEAR","LINK"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"}]' > $HOME0/config/tmp_genesis.json && mv $HOME0/config/tmp_genesis.json $HOME0/config/genesis.json

cat $HOME1/config/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="inj"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="inj"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="inj"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["gov"]["voting_params"]["voting_period"]="5s"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["mint"]["params"]["mint_denom"]="inj"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["auction"]["params"]["auction_period"]="10"' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json
cat $HOME1/config/genesis.json | jq '.app_state["oracle"]["band_ibc_oracle_requests"]=[{"request_id":"1","oracle_script_id":"23","symbols":["BTC","ETH","USDT","INJ","BNB","OSMO"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"},{"request_id":"2","oracle_script_id":"23","symbols":["LUNA","UST","SCRT","ATOM","CRO","STX"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"},{"request_id":"3","oracle_script_id":"23","symbols":["SOL","DOT","MATIC","XRP","AVAX","NEAR","LINK"],"ask_count":"16","min_count":"10","fee_limit":[{"denom":"uband","amount":"1000"}],"prepare_gas":"20000","execute_gas":"400000"}]' > $HOME1/config/tmp_genesis.json && mv $HOME1/config/tmp_genesis.json $HOME1/config/genesis.json

REGEX_REPLACE="perl -i -pe"
n1cfg=$HOME1/config/config.toml
n1app=$HOME1/config/app.toml

$REGEX_REPLACE 's|addr_book_strict = true|addr_book_strict = false|g' $n1cfg
$REGEX_REPLACE 's|external_address = ""|external_address = "tcp://127.0.0.1:26667"|g' $n1cfg
$REGEX_REPLACE 's|"tcp://127.0.0.1:26657"|"tcp://0.0.0.0:26667"|g' $n1cfg
$REGEX_REPLACE 's|"tcp://0.0.0.0:26656"|"tcp://0.0.0.0:26666"|g' $n1cfg
$REGEX_REPLACE 's|"localhost:6060"|"localhost:6061"|g' $n1cfg
$REGEX_REPLACE 's|"tcp://0.0.0.0:10337"|"tcp://0.0.0.0:11337"|g' $n1app
$REGEX_REPLACE 's|"0.0.0.0:1317"|"0.0.0.0:1417"|g' $n1app
$REGEX_REPLACE 's|"0.0.0.0:9900"|"0.0.0.0:9901"|g' $n1app
$REGEX_REPLACE 's|"0.0.0.0:9091"|"0.0.0.0:9092"|g' $n1app
$REGEX_REPLACE 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' $n1cfg
$REGEX_REPLACE 's|timeout_commit = ".*?"|timeout_commit = "1000ms"|g' $n1cfg

yes $PASSPHRASE | ./1-7-injectived keys add node0 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived keys add node1 --home $HOME1
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(yes $PASSPHRASE | ./1-7-injectived keys show node0 -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599,100000000000000000000000000peggy0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48,100000000000000000000000000ibc/B448C0CA358B958301D328CCDC5D5AD642FC30A6D3AE106FF721DB315F3DDE5C --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(yes $PASSPHRASE | ./1-7-injectived keys show node1 -a --home $HOME1) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599,100000000000000000000000000peggy0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48,100000000000000000000000000ibc/B448C0CA358B958301D328CCDC5D5AD642FC30A6D3AE106FF721DB315F3DDE5C --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account inj1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqe2hm49 1inj --home $HOME0

VAL_KEY="localkey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

USER1_KEY="user1"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

USER2_KEY="user2"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

USER3_KEY="user3"
USER3_MNEMONIC="keep liar demand upon shed essence tip undo eagle run people strong sense another salute double peasant egg royal hair report winner student diamond"

USER4_KEY="user4"
USER4_MNEMONIC="pony glide frown crisp unfold lawn cup loan trial govern usual matrix theory wash fresh address pioneer between meadow visa buffalo keep gallery swear"

USER5_KEY="ocrfeedadmin"
USER5_MNEMONIC="earn front swamp dune level clip shell aware apple spare faith upset flip local regret loud suspect view heavy raccoon satisfy cupboard harbor basic"

USER6_KEY="signer1"
USER6_MNEMONIC="output arrange offer advance egg point office silent diamond fame heart hotel rocket sheriff resemble couple race crouch kit laptop document grape drastic lumber"

USER7_KEY="signer2"
USER7_MNEMONIC="velvet gesture rule caution injury stick property decorate raccoon physical narrow tuition address drum shoot pyramid record sport include rich actress sadness crater seek"

USER8_KEY="signer3"
USER8_MNEMONIC="guitar parrot nuclear sun blue marble amazing extend solar device address better chalk shock street absent follow notice female picnic into trade brass couch"

USER9_KEY="signer4"
USER9_MNEMONIC="rotate fame stamp size inform hurdle match stick brain shrimp fancy clinic soccer fortune photo gloom wear punch shed diet celery blossom tide bulk"

USER10_KEY="signer5"
USER10_MNEMONIC="apart acid night more advance december weather expect pause taxi reunion eternal crater crew lady chaos visual dynamic friend match glow flash couple tumble"

USER11_KEY="trading-bot"
USER11_MNEMONIC="laugh double payment throw enemy bubble reform mad credit north claim number plate medal scan sponsor loud release unit say hello give style side"

USER12_KEY="liquidator-bot"
USER12_MNEMONIC="victory giggle weather improve puppy account rate wrist shield guide spice walk buddy ritual fiber collect reduce settle festival range dust ketchup lizard kiwi"

USER13_KEY="price-oracle-legacy"
USER13_MNEMONIC="chair favorite seven fine winter alley saddle night spin salute palace beauty fiction day nephew sail anchor unaware divorce eyebrow trip trophy uncover engage"

NEWLINE=$'\n'

# Import keys from mnemonics
yes "$VAL_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $VAL_KEY --recover --home $HOME0
yes "$USER1_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER1_KEY --recover --home $HOME0
yes "$USER2_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER2_KEY --recover --home $HOME0
yes "$USER3_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER3_KEY --recover --home $HOME0
yes "$USER4_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER4_KEY --recover --home $HOME0
yes "$USER5_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER5_KEY --recover --home $HOME0
yes "$USER6_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER6_KEY --recover --home $HOME0
yes "$USER7_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER7_KEY --recover --home $HOME0
yes "$USER8_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER8_KEY --recover --home $HOME0
yes "$USER9_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER9_KEY --recover --home $HOME0
yes "$USER10_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER11_KEY --recover --home $HOME0
yes "$USER11_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER12_KEY --recover --home $HOME0
yes "$USER12_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER10_KEY --recover --home $HOME0
yes "$USER13_MNEMONIC$NEWLINE$PASSPHRASE" | ./1-7-injectived keys add $USER13_KEY --recover --home $HOME0

# Allocate genesis accounts (cosmos formatted addresses)
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $VAL_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER1_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER2_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER3_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER4_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER5_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER6_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER7_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER8_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER9_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER10_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER11_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER12_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0
yes $PASSPHRASE | ./1-7-injectived add-genesis-account $(./1-7-injectived keys show $USER13_KEY -a --home $HOME0) 100000000000000000000000000inj,100000000000000000000000000peggy0xdAC17F958D2ee523a2206206994597C13D831ec7,100000000000000000000000000peggy0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599 --home $HOME0

# echo "Signing genesis transaction"
cp $HOME0/config/genesis.json $HOME1/config/genesis.json
mkdir gentx
yes $PASSPHRASE | ./1-7-injectived gentx node0 1000000000000000000000inj --chain-id $CHAINID --home $HOME0 --output-document gentx/gentx0.json
yes $PASSPHRASE | ./1-7-injectived gentx node1 1000000000000000000000inj --chain-id $CHAINID --home $HOME1 --output-document gentx/gentx1.json

echo "Collecting genesis transaction"
yes $PASSPHRASE | ./1-7-injectived collect-gentxs --home $HOME0 --gentx-dir gentx
cp $HOME0/config/genesis.json $HOME1/config/genesis.json

echo "Validating genesis"
./1-7-injectived validate-genesis --home $HOME0

echo "Generating cosmovisor dir..."
for i in {0..1}; do
  mkdir -p n$i/cosmovisor/genesis/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.1/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.2/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.3/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.4/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.5/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.6/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.7/bin
  mkdir -p n$i/cosmovisor/upgrades/v1.8/bin
  cp 1-7-injectived n$i/cosmovisor/genesis/bin/injectived
  cp 2-1-injectived n$i/cosmovisor/upgrades/v1.1/bin/injectived
  cp 2-2-injectived n$i/cosmovisor/upgrades/v1.2/bin/injectived
  cp 3-1-injectived n$i/cosmovisor/upgrades/v1.3/bin/injectived
  cp 4-1-injectived n$i/cosmovisor/upgrades/v1.4/bin/injectived
  cp 5-1-injectived n$i/cosmovisor/upgrades/v1.5/bin/injectived
  cp 6-1-injectived n$i/cosmovisor/upgrades/v1.6/bin/injectived
  cp 7-1-injectived n$i/cosmovisor/upgrades/v1.7/bin/injectived
  cp 8-1-injectived n$i/cosmovisor/upgrades/v1.8/bin/injectived
done

rm -rf gentx

echo "Setup done!"

nukeServices
