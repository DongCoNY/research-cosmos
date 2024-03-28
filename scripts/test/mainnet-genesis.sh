#!/bin/bash
# Command to run - ./mainnet-genesis.sh  PATH_GENESIS_DIR
# Example - ./mainnet-genesis.sh  ~/.injectived/config/

set -e
GENESIS_DIR=$1

if ! command -v jq &> /dev/null
then
    echo "⚠️ jq command could not be found!"
    echo "jq is a lightweight and flexible command-line JSON processor."
    echo "Install it by checking https://stedolan.github.io/jq/download/"
    exit 1
fi

if [[ -z "$GENESIS_DIR" ]]; then
  echo "Please provide GENESIS_DIR env"
  exit 1
fi

echo "Updating genesis.json at - $1" 
cat $GENESIS_DIR/genesis.json | jq '.chain_id="injective-1"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# AUTH
cat $GENESIS_DIR/genesis.json | jq '.app_state["auth"]["params"]["max_memo_characters"]="256"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["auth"]["params"]["tx_sig_limit"]="7"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["auth"]["params"]["tx_size_cost_per_byte"]="10"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["auth"]["params"]["sig_verify_cost_ed25519"]="590"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["auth"]["params"]["sig_verify_cost_secp256k1"]="1000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# BANK
cat $GENESIS_DIR/genesis.json | jq '.app_state["bank"]["params"]["default_send_enabled"]=true' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# IBC
cat $GENESIS_DIR/genesis.json | jq '.app_state["transfer"]["params"]["send_enabled"]=false' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["transfer"]["params"]["receive_enabled"]=false' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# CRISIS
cat $GENESIS_DIR/genesis.json | jq '.app_state["crisis"]["constant_fee"]["denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["crisis"]["constant_fee"]["amount"]="1000000000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json


# MINT
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["minter"]["inflation"]="0.050000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["mint_denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["inflation_rate_change"]="0.10000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["inflation_max"]="0.100000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["inflation_min"]="0.040000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["goal_bonded"]="0.670000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["mint"]["params"]["blocks_per_year"]="15778800"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# DISTRIBUTION
cat $GENESIS_DIR/genesis.json | jq '.app_state["distribution"]["params"]["community_tax"]="0.020000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["distribution"]["params"]["base_proposer_reward"]="0.010000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["distribution"]["params"]["bonus_proposer_reward"]="0.040000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["distribution"]["params"]["withdraw_addr_enabled"]=true' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json


# STAKING
cat $GENESIS_DIR/genesis.json | jq '.app_state["staking"]["params"]["unbonding_time"]="1814400s"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["staking"]["params"]["max_validators"]=25' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["staking"]["params"]["max_entries"]=7' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["staking"]["params"]["historical_entries"]=10000' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["staking"]["params"]["bond_denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# SLASHING
cat $GENESIS_DIR/genesis.json | jq '.app_state["slashing"]["params"]["signed_blocks_window"]="40000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["slashing"]["params"]["min_signed_per_window"]="0.500000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["slashing"]["params"]["downtime_jail_duration"]="600s"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["slashing"]["params"]["slash_fraction_double_sign"]="0.050000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["slashing"]["params"]["slash_fraction_downtime"]="0.000100000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# GOVERNANCE
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["amount"]="500000000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["deposit_params"]["max_deposit_period"]="172800s"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["voting_params"]["voting_period"]="172800s"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["tally_params"]["quorum"]="0.334000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["tally_params"]["threshold"]="0.500000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["gov"]["tally_params"]["veto_threshold"]="0.334000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# AUCTION
cat $GENESIS_DIR/genesis.json | jq '.app_state["auction"]["params"]["auction_period"]="604800"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["auction"]["params"]["min_next_bid_increment_rate"]="0.002500000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# EXCHANGE
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["spot_market_instant_listing_fee"]["denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["spot_market_instant_listing_fee"]["amount"]="1000000000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["derivative_market_instant_listing_fee"]["denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["derivative_market_instant_listing_fee"]["amount"]="1000000000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_spot_maker_fee_rate"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_spot_taker_fee_rate"]="0.002000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_derivative_maker_fee_rate"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_derivative_taker_fee_rate"]="0.002000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_initial_margin_ratio"]="0.050000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_maintenance_margin_ratio"]="0.020000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_funding_interval"]="3600"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["funding_multiple"]="3600"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["relayer_fee_share_rate"]="0.400000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_hourly_funding_rate_cap"]="0.000625000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["default_hourly_interest_rate"]="0.000004166660000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["params"]["max_derivative_order_side_count"]=20' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["is_spot_exchange_enabled"]=false' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["exchange"]["is_derivatives_exchange_enabled"]=false' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json


# INSURANCE
cat $GENESIS_DIR/genesis.json | jq '.app_state["insurance"]["params"]["default_redemption_notice_period_duration"]="1209600s"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

# PEGGY
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["peggy_id"]="injective-peggyid"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["contract_source_hash"]=""' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["bridge_ethereum_address"]="0xF955C57f9EA9Dc8781965FEaE0b6A2acE2BAD6f3"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["bridge_chain_id"]="1"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["signed_valsets_window"]="25000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["signed_batches_window"]="25000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["signed_claims_window"]="25000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["target_batch_timeout"]="43200000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["average_block_time"]="2000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["average_ethereum_block_time"]="15000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["slash_fraction_valset"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["slash_fraction_batch"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["slash_fraction_claim"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["slash_fraction_conflicting_claim"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["unbond_slashing_valsets_window"]="25000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["slash_fraction_bad_eth_signature"]="0.001000000000000000"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["cosmos_coin_denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["cosmos_coin_erc20_contract"]="0xe28b3b32b6c345a34ff64674606124dd5aceca30"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["claim_slashing_enabled"]=false' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["bridge_contract_start_height"]="12705133"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["valset_reward"]["denom"]="inj"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json
cat $GENESIS_DIR/genesis.json | jq '.app_state["peggy"]["params"]["valset_reward"]["amount"]="0"' > $GENESIS_DIR/tmp_genesis.json && mv $GENESIS_DIR/tmp_genesis.json $GENESIS_DIR/genesis.json

injectived validate-genesis

echo "Mainnet Genesis Setup done!"
