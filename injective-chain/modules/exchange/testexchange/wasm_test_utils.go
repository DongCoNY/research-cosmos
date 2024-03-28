package testexchange

import (
	"io/ioutil"
	"os"

	clitypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/client/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type DerivativeVaultInstantiateMsg struct {
	MasterAddress                          string `json:"master_address"`
	ConfigOwner                            string `json:"config_owner"`
	MarketId                               string `json:"market_id"`
	Leverage                               string `json:"leverage"`
	OrderDensity                           uint64 `json:"order_density"`
	ReservationPriceSensitivityRatio       string `json:"reservation_price_sensitivity_ratio"`
	ReservationSpreadSensitivityRatio      string `json:"reservation_spread_sensitivity_ratio"`
	MaxActiveCapitalUtilizationRatio       string `json:"max_active_capital_utilization_ratio"`
	HeadChangeToleranceRatio               string `json:"head_change_tolerance_ratio"`
	HeadToTailDeviationRatio               string `json:"head_to_tail_deviation_ratio"`
	MinProximityToLiquidation              string `json:"min_proximity_to_liquidation"`
	PostReductionPercOfMaxPosition         string `json:"post_reduction_perc_of_max_position"`
	MinOracleVolatilitySampleSize          uint32 `json:"min_oracle_volatility_sample_size"`
	EmergencyOracleVolatilitySampleSize    uint32 `json:"emergency_oracle_volatility_sample_size"`
	DefaultMidPriceVolatilityRatio         string `json:"default_mid_price_volatility_ratio"`
	LastValidMarkPrice                     string `json:"last_valid_mark_price"`
	LpSymbol                               string `json:"lp_symbol"`
	MinVolatilityRatio                     string `json:"min_volatility_ratio"`
	SignedMinHeadToFairPriceDeviationRatio string `json:"signed_min_head_to_fair_price_deviation_ratio"`
	SignedMinHeadToTOBDeviationRatio       string `json:"signed_min_head_to_tob_deviation_ratio"`
	AllowedSubscriptionTypes               uint   `json:"allowed_subscription_types"`
	AllowedRedemptionTypes                 uint   `json:"allowed_redemption_types"`
	PositionPnlPenalty                     string `json:"position_pnl_penalty"`
	NotionalValueCap                       string `json:"notional_value_cap"`
	OracleStaleTime                        uint   `json:"oracle_stale_time"`
	OracleVolatilityMaxAge                 uint   `json:"oracle_volatility_max_age"`
}

type SpotVaultInstantiateMsg struct {
	MasterAddress                          string `json:"master_address"`
	ConfigOwner                            string `json:"config_owner"`
	MarketId                               string `json:"market_id"`
	OrderDensity                           uint64 `json:"order_density"`
	HeadChangeToleranceRatio               string `json:"head_change_tolerance_ratio"`
	ReservationPriceSensitivityRatio       string `json:"reservation_price_sensitivity_ratio"`
	ReservationSpreadSensitivityRatio      string `json:"reservation_spread_sensitivity_ratio"`
	MaxActiveCapitalUtilizationRatio       string `json:"max_active_capital_utilization_ratio"`
	MidPriceTailDeviationRatio             string `json:"mid_price_tail_deviation_ratio"`
	HeadToTailDeviationRatio               string `json:"head_to_tail_deviation_ratio"`
	TargetBaseWeight                       string `json:"target_base_weight"`
	MinOracleVolatilitySampleSize          uint32 `json:"min_oracle_volatility_sample_size"`
	EmergencyOracleVolatilitySampleSize    uint32 `json:"emergency_oracle_volatility_sample_size"`
	DefaultMidPriceVolatilityRatio         string `json:"default_mid_price_volatility_ratio"`
	InventoryImbalanceMarketOrderThreshold string `json:"inventory_imbalance_market_order_threshold"`
	MarketSellMidPriceDeviationPercent     string `json:"market_sell_mid_price_deviation_percent"`
	MarketBuyMidPriceDeviationPercent      string `json:"market_buy_mid_price_deviation_percent"`
	LpSymbol                               string `json:"lp_symbol"`
	SignedMinHeadToFairPriceDeviationRatio string `json:"signed_min_head_to_fair_price_deviation_ratio"`
	SignedMinHeadToTOBDeviationRatio       string `json:"signed_min_head_to_tob_deviation_ratio"`
	ReservationPriceTailDeviationRatio     string `json:"reservation_price_tail_deviation_ratio"`
	MinVolatilityRatio                     string `json:"min_volatility_ratio"`
	OracleType                             int32  `json:"oracle_type,omitempty"`
	AllowedSubscriptionTypes               uint   `json:"allowed_subscription_types"`
	AllowedRedemptionTypes                 uint   `json:"allowed_redemption_types"`
	ImbalanceAdjustmentExponent            string `json:"imbalance_adjustment_exponent"`
	RewardDiminishingFactor                string `json:"reward_diminishing_factor"`
	BaseDecimals                           uint8  `json:"base_decimals"`
	QuoteDecimals                          uint8  `json:"quote_decimals"`
	BaseOracleSymbol                       string `json:"base_oracle_symbol"`
	QuoteOracleSymbol                      string `json:"quote_oracle_symbol"`
	NotionalValueCap                       string `json:"notional_value_cap"`
	OracleStaleTime                        uint   `json:"oracle_stale_time"`
	OracleVolatilityMaxAge                 uint   `json:"oracle_volatility_max_age"`
}

type VaultSubscribeArgs struct {
	VaultSubaccountId      string             `json:"vault_subaccount_id"`
	SubscriberSubaccountId string             `json:"subscriber_subaccount_id"`
	SubscriptionType       map[string]string  `json:"subscription_type"`
	MarginRatio            sdk.Dec            `json:"margin_ratio"`
	Slippage               *clitypes.Slippage `json:"slippage,omitempty"`
}
type VaultSubscribe struct {
	Slippage *clitypes.Slippage `json:"slippage,omitempty"`
	//SubscribeArgs VaultSubscribeArgs `json:"args"`
}

type VaultRedeemArgs struct {
	VaultSubaccountId    string            `json:"vault_subaccount_id"`
	RedeemerSubaccountId string            `json:"redeemer_subaccount_id"`
	RedemptionType       map[string]string `json:"redemption_type"`
	RedemptionRatio      *sdk.Dec          `json:"redemption_ratio"`
	Slippage             interface{}       `json:"slippage,omitempty"`
}

type VaultRedeem struct {
	//RedemptionType map[string]string `json:"redemption_type"`
	RedemptionType string      `json:"redemption_type"`
	Slippage       interface{} `json:"slippage,omitempty"`
}

type VaultSubscribeRedeem struct {
	Subscribe *VaultSubscribe `json:"subscribe,omitempty"`
	Redeem    *VaultRedeem    `json:"redeem,omitempty"`
}

type VaultInput struct {
	VaultSubaccountId  string               `json:"vault_subaccount_id"`
	TraderSubaccountId string               `json:"trader_subaccount_id"`
	Msg                VaultSubscribeRedeem `json:"msg"`
}

type InstantiateMasterMsg struct {
	Owner                string `json:"owner"`
	DistributionContract string `json:"distribution_contract"`
	MitoToken            string `json:"mito_token"`
}

func ReadFile(path string) []byte {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	return b
}

func ReadFileWithError(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}
