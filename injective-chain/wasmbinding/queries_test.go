package wasmbinding_test

import (
	"encoding/json"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	simapp "github.com/InjectiveLabs/injective-core/injective-chain/app"
	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/testexchange"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	oracletypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/oracle/types"
	chaintypes "github.com/InjectiveLabs/injective-core/injective-chain/types"

	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"

	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding"
	"github.com/InjectiveLabs/injective-core/injective-chain/wasmbinding/bindings"
)

type UnknownQuery struct{}
type QueryWrapper struct {
	SomeQuery UnknownQuery `json:"someQuery"`
}

var _ = Describe("Queries tests", func() {
	config := sdk.GetConfig()
	chaintypes.SetBech32Prefixes(config)

	var (
		app         *simapp.InjectiveApp
		ctx         sdk.Context
		queryPlugin wasmbinding.QueryPlugin
	)

	BeforeEach(func() {
		app = simapp.Setup(false)
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:  app.LastBlockHeight() + 1,
				AppHash: app.LastCommitID().Hash,
			}})
		ctx = app.BaseApp.NewContext(false, tmproto.Header{
			Height: 1234567,
			Time:   time.Date(2020, time.April, 22, 12, 0, 0, 0, time.UTC),
		})
		bk := app.BankKeeper.(bankkeeper.BaseKeeper)
		queryPlugin = *wasmbinding.NewQueryPlugin(&app.AuthzKeeper, &app.ExchangeKeeper, &app.OracleKeeper, &bk, &app.TokenFactoryKeeper, &app.WasmxKeeper, &app.FeeGrantKeeper)
	})

	Context("staking queries", func() {

		Context("valid staked amount query", func() {

			It("shouldn't return any errors", func() {
				query := bindings.StakingQuery{
					StakedAmount: &bindings.StakingDelegationAmount{
						DelegatorAddress: "inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt",
						MaxDelegations:   10,
					},
				}
				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleStakingQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp bindings.StakingDelegationAmountResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
			})
		})

		Context("invalid query", func() {

			It("should return an error", func() {
				bz, err := json.Marshal("bla bla")
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleStakingQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("Error parsing Injective StakingQuery"), "wrong error returned")
			})
		})

		Context("unknown query", func() {

			It("should return an error", func() {
				query := QueryWrapper{SomeQuery: UnknownQuery{}}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleStakingQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("unknown staking query variant"), "wrong error returned")
			})
		})
	})

	Context("oracle queries", func() {

		Context("valid volatility query", func() {

			It("shouldn't return any errors", func() {
				query := bindings.OracleQuery{
					OracleVolatility: &oracletypes.QueryOracleVolatilityRequest{
						BaseInfo: &oracletypes.OracleInfo{
							Symbol:     "ETH0",
							OracleType: oracletypes.OracleType_PriceFeed,
						},
						QuoteInfo: &oracletypes.OracleInfo{
							Symbol:     "USDT0",
							OracleType: oracletypes.OracleType_PriceFeed,
						},
						OracleHistoryOptions: &oracletypes.OracleHistoryOptions{
							MaxAge:            3600,
							IncludeRawHistory: false,
							IncludeMetadata:   false,
						},
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleOracleQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp oracletypes.QueryOracleVolatilityResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
			})
		})

		Context("invalid query", func() {

			It("should return an error", func() {
				bz, err := json.Marshal("bla bla")
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleOracleQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("Error parsing Injective OracleQuery"), "wrong error returned")
			})
		})

		Context("unknown query", func() {

			It("should return an error", func() {
				query := QueryWrapper{SomeQuery: UnknownQuery{}}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleOracleQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("unknown oracle query variant"), "wrong error returned")
			})
		})
	})

	Context("authz queries", func() {

		Context("valid grants query", func() {

			var (
				subaccountID   string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
				granter        sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID))
				subaccountID_2 string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"
				grantee        sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID_2))
				grant          authz.Grant
			)

			JustBeforeEach(func() {
				var err error

				testInput, ctx := testexchange.SetupTest(app, ctx, 1, 1, 0)
				testexchange.OrFail(err)
				coin := sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(10000000))
				curBlockTime := ctx.BlockTime()
				oneYear := curBlockTime.AddDate(1, 0, 0)

				authorization := banktypes.NewSendAuthorization(sdk.NewCoins(coin), []sdk.AccAddress{granter})
				grant, err = authz.NewGrant(curBlockTime, authorization, &oneYear)
				testexchange.OrFail(err)

				_, err = app.AuthzKeeper.Grant(ctx, &authz.MsgGrant{
					Granter: granter.String(),
					Grantee: grantee.String(),
					Grant:   grant,
				})

				if err != nil {
					panic(err)
				}

			})

			It("should return grants", func() {

				grants := []*authz.Grant{
					{
						Authorization: grant.Authorization,
						Expiration:    grant.Expiration,
					},
				}
				wasmbinding.ForceMarshalJSONAny(grants[0].Authorization)

				expected_response, _ := json.Marshal(authz.QueryGrantsResponse{
					Grants:     grants,
					Pagination: &query.PageResponse{Total: 1},
				})

				query := bindings.AuthzQuery{
					Grants: &authz.QueryGrantsRequest{
						Grantee: grantee.String(),
						Granter: granter.String(),
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})

			It("should return grantee grants", func() {

				grants := []*authz.GrantAuthorization{
					{
						Granter:       granter.String(),
						Grantee:       grantee.String(),
						Authorization: grant.Authorization,
						Expiration:    grant.Expiration,
					},
				}
				wasmbinding.ForceMarshalJSONAny(grants[0].Authorization)

				expected_response, _ := json.Marshal(authz.QueryGranteeGrantsResponse{
					Grants:     grants,
					Pagination: &query.PageResponse{Total: 1},
				})

				query := bindings.AuthzQuery{
					GranteeGrants: &authz.QueryGranteeGrantsRequest{
						Grantee:    grantee.String(),
						Pagination: nil,
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGranteeGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})

			It("should return granter grants", func() {

				grants := []*authz.GrantAuthorization{
					{
						Granter:       granter.String(),
						Grantee:       grantee.String(),
						Authorization: grant.Authorization,
						Expiration:    grant.Expiration,
					},
				}
				wasmbinding.ForceMarshalJSONAny(grants[0].Authorization)

				expected_response, _ := json.Marshal(authz.QueryGranterGrantsResponse{
					Grants:     grants,
					Pagination: &query.PageResponse{Total: 1},
				})

				query := bindings.AuthzQuery{
					GranterGrants: &authz.QueryGranterGrantsRequest{
						Granter:    granter.String(),
						Pagination: nil,
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGranterGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})
		})
		Context("invalid grants query", func() {

			var (
				subaccountID   string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
				granter        sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID))
				subaccountID_2 string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"
				grantee        sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID_2))
			)

			It("should not return grants", func() {

				expected_response, _ := json.Marshal(authz.QueryGrantsResponse{
					Pagination: &query.PageResponse{Total: 0},
				})

				query := bindings.AuthzQuery{
					Grants: &authz.QueryGrantsRequest{
						Grantee: grantee.String(),
						Granter: granter.String(),
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})

			It("should not return grantee grants", func() {

				expected_response, _ := json.Marshal(authz.QueryGranteeGrantsResponse{
					Pagination: &query.PageResponse{Total: 0},
				})

				query := bindings.AuthzQuery{
					GranteeGrants: &authz.QueryGranteeGrantsRequest{
						Grantee:    grantee.String(),
						Pagination: nil,
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGranteeGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})

			It("should not return granter grants", func() {
				expected_response, _ := json.Marshal(authz.QueryGranterGrantsResponse{
					Pagination: &query.PageResponse{Total: 0},
				})

				query := bindings.AuthzQuery{
					GranterGrants: &authz.QueryGranterGrantsRequest{
						Granter:    granter.String(),
						Pagination: nil,
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleAuthzQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp authz.QueryGranterGrantsResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
				Expect(expected_response).To(Equal(respBz), "query returned incorrect response")
			})
		})

	})

	Context("token factory queries", func() {

		Context("valid denom authority query", func() {

			It("shouldn't return any errors", func() {
				query := bindings.TokenfactoryQuery{
					DenomAdmin: &bindings.DenomAdmin{
						Subdenom: "subdemon",
					},
				}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				respBz, err := queryPlugin.HandleTokenFactoryQuery(ctx, bz)
				Expect(err).To(BeNil(), "query handling failed")

				var resp bindings.DenomAdminResponse
				err = json.Unmarshal(respBz, &resp)
				Expect(err).To(BeNil(), "query returned incorrect response type")
			})
		})

		Context("invalid query", func() {

			It("should return an error", func() {
				bz, err := json.Marshal("bla bla")
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleTokenFactoryQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("Error parsing Injective TokenfactoryQuery"), "wrong error returned")
			})
		})

		Context("unknown query", func() {

			It("should return an error", func() {
				query := QueryWrapper{SomeQuery: UnknownQuery{}}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleTokenFactoryQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("unknown tokenfactory query variant"), "wrong error returned")
			})
		})
	})

	Context("exchange queries", func() {

		Context("valid exchange query", func() {
			var (
				spotMarket       *exchangetypes.SpotMarket
				derivativeMarket *exchangetypes.DerivativeMarket
				subaccountID     string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000000"
				subaccountID_2   string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000001"
				subaccountID_3   string         = "eb8cf88b739fe12e303e31fb88fc37751e17cf3d000000000000000000000002"
				address          sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID))
				address_2        sdk.AccAddress = exchangetypes.SubaccountIDToSdkAddress(common.HexToHash(subaccountID_2))
			)
			JustBeforeEach(func() {
				var err error

				testInput, ctx := testexchange.SetupTest(app, ctx, 1, 1, 0)
				spotMarket, err = app.ExchangeKeeper.SpotMarketLaunch(ctx, testInput.Spots[0].Ticker, testInput.Spots[0].BaseDenom, testInput.Spots[0].QuoteDenom, testInput.Spots[0].MinPriceTickSize, testInput.Spots[0].MinQuantityTickSize)
				testexchange.OrFail(err)

				startingPrice := sdk.NewDec(2000)
				oracleBase, oracleQuote, oracleType := testInput.Perps[0].OracleBase, testInput.Perps[0].OracleQuote, testInput.Perps[0].OracleType
				app.OracleKeeper.SetPriceFeedPriceState(ctx, oracleBase, oracleQuote, oracletypes.NewPriceState(startingPrice, ctx.BlockTime().Unix()))

				funder, _ := sdk.AccAddressFromBech32("inj1x2ck0ql2ngyxqtw8jteyc0tchwnwxv7npaungt")
				insuranceFundCoin := sdk.NewCoin(testInput.Perps[0].QuoteDenom, sdk.NewInt(10000000))
				app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(insuranceFundCoin))
				app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, funder, sdk.NewCoins(insuranceFundCoin))
				testexchange.OrFail(app.InsuranceKeeper.CreateInsuranceFund(ctx, funder, insuranceFundCoin, testInput.Perps[0].Ticker, testInput.Perps[0].QuoteDenom, oracleBase, oracleQuote, oracleType, -1))

				derivativeMarket, _, err = app.ExchangeKeeper.PerpetualMarketLaunch(
					ctx,
					testInput.Perps[0].Ticker,
					testInput.Perps[0].QuoteDenom,
					oracleBase, oracleQuote,
					0,
					oracleType,
					testInput.Perps[0].InitialMarginRatio,
					testInput.Perps[0].MaintenanceMarginRatio,
					testInput.Perps[0].MakerFeeRate,
					testInput.Perps[0].TakerFeeRate,
					testInput.Perps[0].MinPriceTickSize,
					testInput.Perps[0].MinQuantityTickSize,
				)
				testexchange.OrFail(err)

				coinsToCreate := []sdk.Coin{sdk.NewCoin(testInput.Spots[0].QuoteDenom, sdk.NewInt(10000000)), sdk.NewCoin(testInput.Spots[0].BaseDenom, sdk.NewInt(10000000))}

				for _, coin := range coinsToCreate {
					app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
					app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, address, sdk.NewCoins(coin))
					app.ExchangeKeeper.IncrementDepositWithCoinOrSendToBank(ctx, common.HexToHash(subaccountID), coin)
					err = app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, sdk.NewCoins(coin))
					testexchange.OrFail(err)

					app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(coin))
					app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, address_2, sdk.NewCoins(coin))
					app.ExchangeKeeper.IncrementDepositWithCoinOrSendToBank(ctx, common.HexToHash(subaccountID_2), coin)
					err = app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, exchangetypes.ModuleName, sdk.NewCoins(coin))
					testexchange.OrFail(err)
				}

				dervLimitOrders := make([]*exchangetypes.MsgCreateDerivativeLimitOrder, 0)
				// create resting orders
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(1500)), sdk.NewDec(int64(100)), sdk.NewDec(int64(150000)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID)))
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(3000)), sdk.NewDec(int64(100)), sdk.NewDec(int64(300000)), exchangetypes.OrderType_SELL, common.HexToHash(subaccountID)))

				// create orders that will result in a position
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(1900)), sdk.NewDec(int64(100)), sdk.NewDec(int64(190000)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID)))
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(1800)), sdk.NewDec(int64(100)), sdk.NewDec(int64(180000)), exchangetypes.OrderType_SELL, common.HexToHash(subaccountID_2)))

				spotLimitOrders := make([]*exchangetypes.MsgCreateSpotLimitOrder, 0)
				spotLimitOrders = append(spotLimitOrders, testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(15)), sdk.NewDec(int64(100)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID)))
				spotLimitOrders = append(spotLimitOrders, testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(40)), sdk.NewDec(int64(10)), exchangetypes.OrderType_SELL, common.HexToHash(subaccountID)))

				msgServer := exchangekeeper.NewMsgServerImpl(app.ExchangeKeeper)
				for _, ord := range dervLimitOrders {
					_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), ord)
					testexchange.OrFail(err)
				}

				for _, ord := range spotLimitOrders {
					_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), ord)
					testexchange.OrFail(err)
				}

				ctx, _ = testexchange.EndBlockerAndCommit(app, ctx)

				// create some transient orders
				dervLimitOrders = make([]*exchangetypes.MsgCreateDerivativeLimitOrder, 0)
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(1500)), sdk.NewDec(int64(100)), sdk.NewDec(int64(150000)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID)))
				dervLimitOrders = append(dervLimitOrders, testInput.NewMsgCreateDerivativeLimitOrder(sdk.NewDec(int64(3000)), sdk.NewDec(int64(100)), sdk.NewDec(int64(300000)), exchangetypes.OrderType_SELL, common.HexToHash(subaccountID)))

				spotLimitOrders = make([]*exchangetypes.MsgCreateSpotLimitOrder, 0)
				spotLimitOrders = append(spotLimitOrders, testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(15)), sdk.NewDec(int64(100)), exchangetypes.OrderType_BUY, common.HexToHash(subaccountID)))
				spotLimitOrders = append(spotLimitOrders, testInput.NewMsgCreateSpotLimitOrder(sdk.NewDec(int64(40)), sdk.NewDec(int64(10)), exchangetypes.OrderType_SELL, common.HexToHash(subaccountID)))

				for _, ord := range dervLimitOrders {
					_, err := msgServer.CreateDerivativeLimitOrder(sdk.WrapSDKContext(ctx), ord)
					testexchange.OrFail(err)
				}

				for _, ord := range spotLimitOrders {
					_, err := msgServer.CreateSpotLimitOrder(sdk.WrapSDKContext(ctx), ord)
					testexchange.OrFail(err)
				}
			})

			DescribeTable("with results",
				func(queryFunc func() bindings.ExchangeQuery, response interface{}) {
					query := queryFunc()
					bz, err := json.Marshal(query)
					testexchange.OrFail(err)

					respBz, err := queryPlugin.HandleExchangeQuery(ctx, bz)
					Expect(err).To(BeNil(), "query handling failed")

					err = json.Unmarshal(respBz, &response)
					Expect(err).To(BeNil(), "query returned incorrect response type")
				},
				Entry("exchange params", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						ExchangeParams: &exchangetypes.QueryExchangeParamsRequest{}}
				}, exchangetypes.QueryExchangeParamsResponse{}),

				Entry("subaccount deposits", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountDeposit: &exchangetypes.QuerySubaccountDepositRequest{
							SubaccountId: subaccountID,
							Denom:        "USDT0",
						},
					}
				}, exchangetypes.QuerySubaccountDepositResponse{}),

				Entry("existing spot market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SpotMarket: &exchangetypes.QuerySpotMarketRequest{
							MarketId: spotMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySpotMarketResponse{}),

				Entry("existing derivative market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						DerivativeMarket: &exchangetypes.QueryDerivativeMarketRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryDerivativeMarketResponse{}),

				Entry("subaccount positions", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountPositions: &exchangetypes.QuerySubaccountPositionsRequest{
							SubaccountId: subaccountID,
						},
					}
				}, exchangetypes.QuerySubaccountPositionsResponse{}),

				Entry("subaccount position in market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountPositionInMarket: &exchangetypes.QuerySubaccountPositionInMarketRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountPositionInMarketResponse{}),

				Entry("subaccount effective position in market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountEffectivePositionInMarket: &exchangetypes.QuerySubaccountEffectivePositionInMarketRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountEffectivePositionInMarketResponse{}),

				Entry("subaccount orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountOrders: &exchangetypes.QuerySubaccountOrdersRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountOrdersResponse{}),

				Entry("trader derivative orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderDerivativeOrders: &exchangetypes.QueryTraderDerivativeOrdersRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderDerivativeOrdersResponse{}),

				//has processed full amount true/false | verify request?
				Entry("trader spot orders up to amount", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderSpotOrdersToCancelUpToAmountRequest: &exchangetypes.QueryTraderSpotOrdersToCancelUpToAmountRequest{
							SubaccountId: subaccountID,
							MarketId:     spotMarket.MarketId,
							BaseAmount:   sdk.NewDecFromInt(sdk.NewInt(10)),
							QuoteAmount:  sdk.NewDecFromInt(sdk.NewInt(10)),
							Strategy:     0,
						},
					}
				}, exchangetypes.QueryTraderSpotOrdersToCancelUpToAmountRequest{}),

				//has processed full amount true/false | verify request?
				Entry("trader derivative orders up to amount", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderDerivativeOrdersToCancelUpToAmountRequest: &exchangetypes.QueryTraderDerivativeOrdersToCancelUpToAmountRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
							QuoteAmount:  sdk.NewDecFromInt(sdk.NewInt(1000)),
							Strategy:     0,
						},
					}
				}, exchangetypes.QueryTraderDerivativeOrdersToCancelUpToAmountRequest{}),

				Entry("trader spot orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderSpotOrders: &exchangetypes.QueryTraderSpotOrdersRequest{
							SubaccountId: subaccountID,
							MarketId:     spotMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderSpotOrdersResponse{}),

				Entry("trader transient spot orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderTransientSpotOrders: &exchangetypes.QueryTraderSpotOrdersRequest{
							SubaccountId: subaccountID,
							MarketId:     spotMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderSpotOrdersResponse{}),

				Entry("trader transient deriative orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderTransientDerivativeOrders: &exchangetypes.QueryTraderDerivativeOrdersRequest{
							SubaccountId: subaccountID,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderDerivativeOrdersResponse{}),

				Entry("perpetual market info", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						PerpetualMarketInfo: &exchangetypes.QueryPerpetualMarketInfoRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryPerpetualMarketInfoResponse{}),

				Entry("perpetual market funding", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						PerpetualMarketFunding: &exchangetypes.QueryPerpetualMarketFundingRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryPerpetualMarketFundingResponse{}),

				Entry("expiry futures market info", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						ExpiryFuturesMarketInfo: &exchangetypes.QueryExpiryFuturesMarketInfoRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryExpiryFuturesMarketInfoResponse{}),

				Entry("market volatility", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						MarketVolatility: &exchangetypes.QueryMarketVolatilityRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryMarketVolatilityResponse{}),

				Entry("spot market mid price and TOB", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SpotMarketMidPriceAndTOB: &exchangetypes.QuerySpotMidPriceAndTOBRequest{
							MarketId: spotMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySpotMidPriceAndTOBResponse{}),

				Entry("derivative market mid price and TOB", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						DerivativeMarketMidPriceAndTOB: &exchangetypes.QueryDerivativeMidPriceAndTOBRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryDerivativeMidPriceAndTOBResponse{}),

				Entry("atomic market order fee multiplier", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						MarketAtomicExecutionFeeMultiplier: &exchangetypes.QueryMarketAtomicExecutionFeeMultiplierRequest{
							MarketId: derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryMarketAtomicExecutionFeeMultiplierResponse{}),
			)

			DescribeTable("without results",
				func(queryFunc func() bindings.ExchangeQuery, response interface{}) {
					query := queryFunc()
					bz, err := json.Marshal(query)
					testexchange.OrFail(err)

					respBz, err := queryPlugin.HandleExchangeQuery(ctx, bz)
					Expect(err).To(BeNil(), "query handling failed")

					err = json.Unmarshal(respBz, &response)
					Expect(err).To(BeNil(), "query returned incorrect response type")
				},
				Entry("subaccount deposits with incorrect denom", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountDeposit: &exchangetypes.QuerySubaccountDepositRequest{
							SubaccountId: subaccountID,
							Denom:        "USD",
						},
					}
				}, exchangetypes.QuerySubaccountDepositResponse{}),

				Entry("subaccount deposits without any balance", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountDeposit: &exchangetypes.QuerySubaccountDepositRequest{
							SubaccountId: subaccountID_3,
							Denom:        "USDT0",
						},
					}
				}, exchangetypes.QuerySubaccountDepositResponse{}),

				Entry("non-existent spot market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SpotMarket: &exchangetypes.QuerySpotMarketRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QuerySpotMarketResponse{}),

				Entry("non-existent derivative market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						DerivativeMarket: &exchangetypes.QueryDerivativeMarketRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryDerivativeMarketResponse{}),

				Entry("subaccount positions without any position", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountPositions: &exchangetypes.QuerySubaccountPositionsRequest{
							SubaccountId: subaccountID_3,
						},
					}
				}, exchangetypes.QuerySubaccountPositionsResponse{}),

				Entry("subaccount position in market  without any position", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountPositionInMarket: &exchangetypes.QuerySubaccountPositionInMarketRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountPositionInMarketResponse{}),

				Entry("subaccount effective position in market without any position", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountEffectivePositionInMarket: &exchangetypes.QuerySubaccountEffectivePositionInMarketRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountEffectivePositionInMarketResponse{}),

				Entry("subaccount orders without any orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SubaccountOrders: &exchangetypes.QuerySubaccountOrdersRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QuerySubaccountOrdersResponse{}),

				Entry("trader derivative orders without any orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderDerivativeOrders: &exchangetypes.QueryTraderDerivativeOrdersRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderDerivativeOrdersResponse{}),

				Entry("trader spot orders without any orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderSpotOrders: &exchangetypes.QueryTraderSpotOrdersRequest{
							SubaccountId: subaccountID_3,
							MarketId:     spotMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderSpotOrdersResponse{}),

				Entry("trader transient spot orders without any orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderTransientSpotOrders: &exchangetypes.QueryTraderSpotOrdersRequest{
							SubaccountId: subaccountID_3,
							MarketId:     spotMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderSpotOrdersResponse{}),

				Entry("trader transient deriative orders without any orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderTransientDerivativeOrders: &exchangetypes.QueryTraderDerivativeOrdersRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
						},
					}
				}, exchangetypes.QueryTraderDerivativeOrdersResponse{}),

				Entry("perpetual market info of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						PerpetualMarketInfo: &exchangetypes.QueryPerpetualMarketInfoRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryPerpetualMarketInfoResponse{}),

				Entry("perpetual market funding of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						PerpetualMarketFunding: &exchangetypes.QueryPerpetualMarketFundingRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryPerpetualMarketFundingResponse{}),

				Entry("expiry futures market info of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						ExpiryFuturesMarketInfo: &exchangetypes.QueryExpiryFuturesMarketInfoRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryExpiryFuturesMarketInfoResponse{}),

				Entry("market volatility of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						MarketVolatility: &exchangetypes.QueryMarketVolatilityRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryMarketVolatilityResponse{}),

				Entry("spot market mid price and TOB of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						SpotMarketMidPriceAndTOB: &exchangetypes.QuerySpotMidPriceAndTOBRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QuerySpotMidPriceAndTOBResponse{}),

				Entry("derivative market mid price and TOB of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						DerivativeMarketMidPriceAndTOB: &exchangetypes.QueryDerivativeMidPriceAndTOBRequest{
							MarketId: "market_id",
						},
					}
				}, exchangetypes.QueryDerivativeMidPriceAndTOBResponse{}),
			)

			DescribeTable("returning error",
				func(queryFunc func() bindings.ExchangeQuery) {
					query := queryFunc()
					bz, err := json.Marshal(query)
					testexchange.OrFail(err)

					_, err = queryPlugin.HandleExchangeQuery(ctx, bz)
					Expect(err).To(Not(BeNil()), "query did not return an error")
				},

				Entry("trader spot orders up to amount when there are no orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderSpotOrdersToCancelUpToAmountRequest: &exchangetypes.QueryTraderSpotOrdersToCancelUpToAmountRequest{
							SubaccountId: subaccountID_3,
							MarketId:     spotMarket.MarketId,
							BaseAmount:   sdk.NewDecFromInt(sdk.NewInt(10)),
							QuoteAmount:  sdk.NewDecFromInt(sdk.NewInt(10)),
							Strategy:     0,
						},
					}
				}),

				Entry("trader derivative orders up to amount when there are no orders", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						TraderDerivativeOrdersToCancelUpToAmountRequest: &exchangetypes.QueryTraderDerivativeOrdersToCancelUpToAmountRequest{
							SubaccountId: subaccountID_3,
							MarketId:     derivativeMarket.MarketId,
							QuoteAmount:  sdk.NewDecFromInt(sdk.NewInt(1000)),
							Strategy:     0,
						},
					}
				}),

				Entry("atomic market order fee multiplier of non-existent market", func() bindings.ExchangeQuery {
					return bindings.ExchangeQuery{
						MarketAtomicExecutionFeeMultiplier: &exchangetypes.QueryMarketAtomicExecutionFeeMultiplierRequest{
							MarketId: "market_id",
						},
					}
				}),
			)

			DescribeTable("tokenfactory with results",
				func(queryFunc func() bindings.TokenfactoryQuery, response interface{}) {
					query := queryFunc()
					bz, err := json.Marshal(query)
					testexchange.OrFail(err)

					respBz, err := queryPlugin.HandleTokenFactoryQuery(ctx, bz)
					Expect(err).To(BeNil(), "query handling failed")

					err = json.Unmarshal(respBz, &response)
					Expect(err).To(BeNil(), "query returned incorrect response type")
				},

				Entry("token factory denom creation fee", func() bindings.TokenfactoryQuery {
					return bindings.TokenfactoryQuery{
						DenomCreationFee: &bindings.DenomCreationFee{},
					}
				}, bindings.DenomCreationFeeResponse{}),
			)
		})

		Context("invalid query", func() {

			It("should return an error", func() {
				bz, err := json.Marshal("bla bla")
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleExchangeQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("Error parsing Injective ExchangeQuery"), "wrong error returned")
			})
		})

		Context("unknown query", func() {

			It("should return an error", func() {
				query := QueryWrapper{SomeQuery: UnknownQuery{}}

				bz, err := json.Marshal(query)
				testexchange.OrFail(err)

				_, err = queryPlugin.HandleExchangeQuery(ctx, bz)
				Expect(err).To(Not(BeNil()), "query handling succeeded")
				Expect(err.Error()).To(ContainSubstring("unknown exchange query variant"), "wrong error returned")
			})
		})
	})
})
