package keeper_test

import (
	"encoding/json"
	"testing"

	"cosmossdk.io/errors"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	exchangekeeper "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/keeper"
	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/keeper"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func TestNamespaces(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	subdenom := "tst"
	denom := formatDenom(sender.String(), subdenom)

	var namespace *types.Namespace
	var err error

	t.Run("No namespace should be found", func(t *testing.T) {
		namespace, err = k.GetNamespaceForDenom(s.Ctx, denom, false)
		require.Nil(t, namespace, "no namespace should be found")
		require.Nil(t, err, "no error either")
	})

	t.Run("Create namespace should work", func(t *testing.T) {
		denomResp, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
		require.NoError(t, err)
		require.Equal(t, denom, denomResp.NewTokenDenom)

		createNamespaceMsg := &types.MsgCreateNamespace{
			Sender: sender.String(),
			Namespace: types.Namespace{
				Denom: denom,
				RolePermissions: []*types.Role{
					{
						Role:        types.EVERYONE,
						Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
					}, {
						Role:        "blacklisted",
						Permissions: 0,
					},
				},
				AddressRoles: []*types.AddressRoles{
					{
						Address: badGuy.String(),
						Roles:   []string{"blacklisted"},
					},
				},
			},
		}

		err = createNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
		require.NoError(t, err)
	})

	t.Run("Existing namespace should be found", func(t *testing.T) {
		namespace, err = k.GetNamespaceForDenom(s.Ctx, denom, false)
		require.NotNil(t, namespace, "namespace should be found")
		require.NoError(t, err, "no error either")
	})

	t.Run("Should not be able to create namespace twice", func(t *testing.T) {
		createNamespaceMsg := &types.MsgCreateNamespace{
			Sender: sender.String(),
			Namespace: types.Namespace{
				Denom: denom,
				RolePermissions: []*types.Role{
					{
						Role:        types.EVERYONE,
						Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
					}, {
						Role:        "blacklisted",
						Permissions: 0,
					},
				},
				AddressRoles: []*types.AddressRoles{
					{
						Address: badGuy.String(),
						Roles:   []string{"blacklisted"},
					},
				},
			},
		}

		err = createNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
		require.True(t, errors.IsOf(err, types.ErrDenomNamespaceExists))
	})

	t.Run("Creating namespace for denom not from permissions module should fail", func(t *testing.T) {
		createNamespaceMsg := &types.MsgCreateNamespace{
			Sender: sender.String(),
			Namespace: types.Namespace{
				Denom: "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9/atom",
				RolePermissions: []*types.Role{
					{
						Role:        types.EVERYONE,
						Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
					}, {
						Role:        "blacklisted",
						Permissions: 0,
					},
				},
				AddressRoles: []*types.AddressRoles{
					{
						Address: badGuy.String(),
						Roles:   []string{"blacklisted"},
					},
				},
			},
		}

		err = createNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
		require.NotNil(t, err, "should fail")
		require.Contains(t, err.Error(), "permissions namespace can only be applied to tokenfactory denoms")
	})

	t.Run("Should not be able to receive tokens to address", func(t *testing.T) {
		_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(100))))
		require.NoError(t, err)

		err = s.App.BankKeeper.SendCoins(s.Ctx, sender, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))
	})

	t.Run("Should be able to send tokens to a non-blacklisted address", func(t *testing.T) {
		err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.Nil(t, err)
	})

	t.Run("Only denom admin should be able to update namespace, non-denom admins should fail", func(t *testing.T) {
		updateNamespaceMsg := &types.MsgUpdateNamespace{
			Sender:         badGuy.String(),
			NamespaceDenom: denom,
			MintsPaused:    &types.MsgUpdateNamespace_MsgSetMintsPaused{NewValue: true},
			SendsPaused:    &types.MsgUpdateNamespace_MsgSetSendsPaused{NewValue: true},
			BurnsPaused:    &types.MsgUpdateNamespace_MsgSetBurnsPaused{NewValue: true},
		}
		err = updateNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.UpdateNamespace(s.Ctx, updateNamespaceMsg)
		require.True(t, errors.IsOf(err, types.ErrUnauthorized))

		updateNamespaceMsg.Sender = sender.String()
		_, err = s.MsgServer.UpdateNamespace(s.Ctx, updateNamespaceMsg)
		require.NoError(t, err)
	})

	t.Run("Only denom admin should be able to update roles, non-denom admin should fail", func(t *testing.T) {
		updateRolesMsg := &types.MsgUpdateNamespaceRoles{
			Sender:         badGuy.String(),
			NamespaceDenom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        "burner",
					Permissions: uint32(types.Action_BURN),
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: goodGuy.String(),
					Roles:   []string{"blacklisted", "burner"},
				},
			},
		}

		err = updateRolesMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
		require.True(t, errors.IsOf(err, types.ErrUnauthorized))

		updateRolesMsg.Sender = sender.String()
		_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
		require.NoError(t, err)
	})

	t.Run("Only denom admin should be able to revoke roles, non-denom admin should fail", func(t *testing.T) {
		revokeNamespaceMsg := &types.MsgRevokeNamespaceRoles{
			Sender:         badGuy.String(),
			NamespaceDenom: denom,
			AddressRolesToRevoke: []*types.AddressRoles{
				{
					Address: goodGuy.String(),
					Roles:   []string{"blacklisted", "burner"},
				},
			},
		}
		err = revokeNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, revokeNamespaceMsg)
		require.True(t, errors.IsOf(err, types.ErrUnauthorized))

		revokeNamespaceMsg.Sender = sender.String()
		_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, revokeNamespaceMsg)
		require.NoError(t, err)
	})

	t.Run("Only denom admin should be able to delete a namespace, non-denom admin should fail", func(t *testing.T) {
		deleteNamespaceMsg := &types.MsgDeleteNamespace{
			Sender:         badGuy.String(),
			NamespaceDenom: denom,
		}
		err = deleteNamespaceMsg.ValidateBasic()
		require.NoError(t, err)

		_, err = s.MsgServer.DeleteNamespace(s.Ctx, deleteNamespaceMsg)
		require.True(t, errors.IsOf(err, types.ErrUnauthorized))

		deleteNamespaceMsg.Sender = sender.String()
		_, err = s.MsgServer.DeleteNamespace(s.Ctx, deleteNamespaceMsg)
		require.NoError(t, err)
	})
}

func TestPermissions(t *testing.T) {
	s, _ := setupTestSuite(t, 8)

	admin := s.TestAccs[0]
	mintOnly := s.TestAccs[1]
	receiveOnly := s.TestAccs[2]
	burnOnly := s.TestAccs[3]
	mintBurn := s.TestAccs[4]
	receiveBurn := s.TestAccs[5]
	mintReceive := s.TestAccs[6]
	allActions := s.TestAccs[7]

	subdenom := "tst2"
	denom := formatDenom(admin.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(admin.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	// first top-up all accounts before we applied any restrictions to later test sends from those accounts
	s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(admin.String(), sdk.NewCoin(denom, sdk.NewInt(100000000000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, mintOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, receiveOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, burnOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, mintBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, receiveBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, mintReceive, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))
	s.App.BankKeeper.SendCoins(s.Ctx, admin, allActions, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10000))))

	createNamespaceMsg := &types.MsgCreateNamespace{
		Sender: admin.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: 0,
				}, {
					Role:        "minter",
					Permissions: uint32(types.Action_MINT),
				}, {
					Role:        "receiver",
					Permissions: uint32(types.Action_RECEIVE),
				}, {
					Role:        "burner",
					Permissions: uint32(types.Action_BURN),
				}, {
					Role:        "burner_minter",
					Permissions: uint32(types.Action_BURN | types.Action_MINT),
				}, {
					Role:        "burner_receiver",
					Permissions: uint32(types.Action_BURN | types.Action_RECEIVE),
				}, {
					Role:        "minter_receiver",
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE),
				}, {
					Role:        "all_actions",
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: mintOnly.String(),
					Roles:   []string{"minter"},
				}, {
					Address: receiveOnly.String(),
					Roles:   []string{"receiver"},
				}, {
					Address: burnOnly.String(),
					Roles:   []string{"burner"},
				}, {
					Address: mintBurn.String(),
					Roles:   []string{"burner_minter"},
				}, {
					Address: receiveBurn.String(),
					Roles:   []string{"burner_receiver"},
				}, {
					Address: mintReceive.String(),
					Roles:   []string{"minter_receiver"},
				}, {
					Address: allActions.String(),
					Roles:   []string{"all_actions"},
				}, {
					Address: admin.String(),
					Roles:   []string{"all_actions"},
				},
			},
		},
	}

	err = createNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.NoError(t, err)

	t.Run("Mint permissioned role should only be able to mint", func(t *testing.T) {
		// denom admin to some account is MINT, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, mintOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to non-admin account is RECEIVE, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, mintOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction), err)

		// non-admin account to denom admin account is BURN, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))
	})

	t.Run("Receive permissioned role should only be able to receive", func(t *testing.T) {
		// denom admin to some account is MINT, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, receiveOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

		// non-admin account to non-admin account is RECEIVE, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, receiveOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to denom admin account is BURN, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, receiveOnly, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))
	})

	t.Run("Burn permissioned role should only be able to burn", func(t *testing.T) {
		// denom admin to some account is MINT, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, burnOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

		// non-admin account to non-admin account is RECEIVE, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, burnOnly, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

		// non-admin account to denom admin account is BURN, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, burnOnly, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)
	})

	t.Run("MintBurn permissioned role should only be able to mint and burn", func(t *testing.T) {
		// denom admin to some account is MINT, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, mintBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to non-admin account is RECEIVE, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, mintBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

		// non-admin account to denom admin account is BURN, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintBurn, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)
	})

	t.Run("ReceiveBurn permissioned role should only be able to send and burn", func(t *testing.T) {
		// denom admin to some account is MINT, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, receiveBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

		// non-admin account to non-admin account is RECEIVE, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, receiveOnly, receiveBurn, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to denom admin account is BURN, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, receiveBurn, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)
	})

	t.Run("MintReceive permissioned role should only be able to mint and send", func(t *testing.T) {
		// denom admin to some account is MINT, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, mintReceive, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to non-admin account is RECEIVE, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, receiveOnly, mintReceive, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to denom admin account is BURN, should fail
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintReceive, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction))
	})

	t.Run("AllActions permissioned role should be able to mint, send and burn", func(t *testing.T) {
		// denom admin to some account is MINT, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, admin, allActions, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to non-admin account is RECEIVE, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, mintOnly, allActions, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)

		// non-admin account to denom admin account is BURN, should work
		err = s.App.BankKeeper.SendCoins(s.Ctx, allActions, admin, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.NoError(t, err)
	})
}

func TestNamespace(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, false)
	require.Nil(t, ns, "no namespace should be found")
	require.Nil(t, err, "no error either")

	_, err = s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := &types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				}, {
					Role:        "blacklisted",
					Permissions: 0,
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: badGuy.String(),
					Roles:   []string{"blacklisted"},
				},
			},
		},
	}

	err = createNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.NoError(t, err)

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.True(t, errors.IsOf(err, types.ErrDenomNamespaceExists))

	// test transfer permissions
	_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(100))))
	require.NoError(t, err)

	// MINT
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)

	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

	/* TEST UPDATE */
	updateNamespaceMsg := &types.MsgUpdateNamespace{
		Sender:         badGuy.String(),
		NamespaceDenom: denom,
		MintsPaused:    &types.MsgUpdateNamespace_MsgSetMintsPaused{NewValue: true},
		SendsPaused:    &types.MsgUpdateNamespace_MsgSetSendsPaused{NewValue: true},
		BurnsPaused:    &types.MsgUpdateNamespace_MsgSetBurnsPaused{NewValue: true},
	}
	err = updateNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.UpdateNamespace(s.Ctx, updateNamespaceMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized))

	updateNamespaceMsg.Sender = sender.String()
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, updateNamespaceMsg)
	require.NoError(t, err)
	// now even good send should fail cause mints are paused
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.True(t, errors.IsOf(err, types.ErrRestrictedAction))
	// restore factory settings
	updateNamespaceMsg.MintsPaused.NewValue = false
	updateNamespaceMsg.SendsPaused.NewValue = false
	updateNamespaceMsg.BurnsPaused.NewValue = false
	//updateNamespaceMsg.WasmHook.NewValue = ""
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, updateNamespaceMsg)
	require.NoError(t, err)

	/* TEST UPDATE ROLES */

	updateRolesMsg := &types.MsgUpdateNamespaceRoles{
		Sender:         badGuy.String(),
		NamespaceDenom: denom,
		RolePermissions: []*types.Role{
			{
				Role:        "burner",
				Permissions: uint32(types.Action_BURN),
			},
		},
		AddressRoles: []*types.AddressRoles{
			{
				Address: goodGuy.String(),
				Roles:   []string{"blacklisted", "burner"},
			},
		},
	}

	err = updateRolesMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized))

	updateRolesMsg.Sender = sender.String()
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
	require.NoError(t, err)
	// now even good send should fail cause he is blacklisted
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.True(t, errors.IsOf(err, types.ErrRestrictedAction))

	/* TEST REVOKE */
	revokeNamespaceMsg := &types.MsgRevokeNamespaceRoles{
		Sender:         badGuy.String(),
		NamespaceDenom: denom,
		AddressRolesToRevoke: []*types.AddressRoles{
			{
				Address: goodGuy.String(),
				Roles:   []string{"blacklisted", "burner"},
			},
		},
	}
	err = revokeNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, revokeNamespaceMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized))

	revokeNamespaceMsg.Sender = sender.String()
	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, revokeNamespaceMsg)
	require.NoError(t, err)
	// shoud succeed again since we revoked all roles
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)

	/* TEST DELETE */
	deleteNamespaceMsg := &types.MsgDeleteNamespace{
		Sender:         badGuy.String(),
		NamespaceDenom: denom,
	}
	err = deleteNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.DeleteNamespace(s.Ctx, deleteNamespaceMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized))

	deleteNamespaceMsg.Sender = sender.String()
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, deleteNamespaceMsg)
	require.NoError(t, err)

	// and send should succeed
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)
}

func TestNamespaceWasm(t *testing.T) {
	s, _ := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	codeId, err := storeContract(s, "../wasm-hook-example/artifacts/wasm_hook_example-aarch64.wasm", sender.String())
	require.NoError(t, err)
	initMsg, _ := json.Marshal(map[string]any{"owner": sender.String()})
	wasmAddress, _, err := initialiseContract(s, codeId, "wasm_hook", sender.String(), initMsg)
	require.NoError(t, err)

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	_, err = s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := &types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom:    denom,
			WasmHook: wasmAddress.String(),
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				}, {
					Role:        "blacklisted",
					Permissions: 0,
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: badGuy.String(),
					Roles:   []string{"blacklisted"},
				},
			},
		},
	}

	err = createNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.NoError(t, err)

	// test transfer permissions
	// we try to mint to sender, but wasm contract will replace the sender address with contract address
	_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(100))))
	if sender.String()[len(sender.String())-1] < 'd' {
		require.True(t, errors.IsOf(err, types.ErrWasmHookError), err)
	} else {
		require.NoError(t, err)
	}

	if err == nil {
		// since coins are now minted to contract, we send them from that address, and the receiver address will also be overwritten to contract
		err = s.App.BankKeeper.SendCoins(s.Ctx, wasmAddress, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		if goodGuy.String()[len(goodGuy.String())-1] < 'd' {
			require.True(t, errors.IsOf(err, types.ErrWasmHookError), err)
		} else {
			require.NoError(t, err)
			require.Equal(t, int64(0), s.App.BankKeeper.GetBalance(s.Ctx, goodGuy, denom).Amount.Int64())
			require.Equal(t, int64(100), s.App.BankKeeper.GetBalance(s.Ctx, wasmAddress, denom).Amount.Int64())
		}

		err = s.App.BankKeeper.SendCoins(s.Ctx, wasmAddress, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
		require.True(t, errors.IsOf(err, types.ErrRestrictedAction), err)
	}
}

func TestNamespaceVoucher(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	// goodGuy := s.TestAccs[2]
	exchangeModuleAddress := authtypes.NewModuleAddress(exchangetypes.ModuleName)
	permissionsModuleAddress := authtypes.NewModuleAddress(types.ModuleName)

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, false)
	require.Nil(t, ns, "no namespace should be found")
	require.Nil(t, err, "no error either")

	_, err = s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := &types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				}, {
					Role:        "blacklisted",
					Permissions: 0,
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: badGuy.String(),
					Roles:   []string{"blacklisted"},
				},
			},
		},
	}

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.NoError(t, err)

	// test send from module to address
	_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(100))))
	require.NoError(t, err)
	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, exchangeModuleAddress, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(100))))
	require.NoError(t, err)
	// it should be instead sent to voucher
	err = s.App.BankKeeper.SendCoinsFromModuleToAccount(s.Ctx, exchangetypes.ModuleName, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)
	require.Equal(t, "0", s.App.BankKeeper.GetBalance(s.Ctx, badGuy, denom).Amount.String())
	require.Equal(t, "10", s.App.BankKeeper.GetBalance(s.Ctx, permissionsModuleAddress, denom).Amount.String())

	// test vouchers query
	queryServer := keeper.NewQueryServerImpl(k)
	res, err := queryServer.VouchersForAddress(s.Ctx, &types.QueryVouchersForAddressRequest{Address: badGuy.String()})
	require.NoError(t, err)
	require.Equal(t, []*types.AddressVoucher{{Address: exchangeModuleAddress.String(), Voucher: &types.Voucher{Coins: sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10)))}}}, res.Vouchers)

	// redeem voucher should fail
	err = s.App.BankKeeper.SendCoinsFromModuleToAccount(s.Ctx, "permissions", badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.True(t, errors.IsOf(err, types.ErrRestrictedAction), err)

	// Update namespace to allow exchange module to send to badGuy
	updateRolesMsg := &types.MsgUpdateNamespaceRoles{
		Sender:         sender.String(),
		NamespaceDenom: denom,
		RolePermissions: []*types.Role{
			{
				Role:        "burner",
				Permissions: uint32(types.Action_BURN),
			},
		},
		AddressRoles: []*types.AddressRoles{
			{
				Address: badGuy.String(),
				Roles:   []string{},
			},
		},
	}

	err = updateRolesMsg.ValidateBasic()
	require.NoError(t, err)

	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
	require.NoError(t, err)

	updateRolesMsg.Sender = sender.String()
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, updateRolesMsg)
	require.NoError(t, err)

	// redeem voucher should work
	require.Equal(t, "10", s.App.BankKeeper.GetBalance(s.Ctx, permissionsModuleAddress, denom).Amount.String())
	err = s.App.BankKeeper.SendCoinsFromModuleToAccount(s.Ctx, "permissions", badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)
}

func storeContract(s *KeeperTestSuite, wasmFilePath string, sender string) (uint64, error) {
	msgServer := wasmkeeper.NewMsgServerImpl(&s.App.WasmKeeper)

	storeContractCodeMsg := &wasmtypes.MsgStoreCode{
		Sender:       sender,
		WASMByteCode: exchangekeeper.ReadFile(wasmFilePath),
	}
	respStore, err := msgServer.StoreCode(
		sdk.WrapSDKContext(s.Ctx),
		storeContractCodeMsg,
	)
	if err != nil {
		return 0, err
	}

	return respStore.CodeID, nil
}

func initialiseContract(s *KeeperTestSuite, codeId uint64, label string, sender string, message []byte) (sdk.AccAddress, uint64, error) {
	ctx := s.Ctx
	app := s.App
	msgServer := wasmkeeper.NewMsgServerImpl(&app.WasmKeeper)

	instantiateContractMsg := &wasmtypes.MsgInstantiateContract{
		Sender: sender,
		Admin:  sender,
		CodeID: codeId,
		Label:  label,
		Msg:    wasmtypes.RawContractMessage(message),
		Funds:  sdk.NewCoins(),
	}

	_, err := msgServer.InstantiateContract(
		sdk.WrapSDKContext(ctx),
		instantiateContractMsg,
	)

	if err != nil {
		return nil, 0, err
	}

	var contractAddress string
	app.WasmKeeper.IterateContractsByCode(ctx, codeId, func(addr sdk.AccAddress) bool {
		contractAddress = addr.String()
		return true
	})

	contractAddr, err := sdk.AccAddressFromBech32(contractAddress)
	if err != nil {
		return nil, 0, err
	}
	return contractAddr, codeId, nil
}

func TestGenesis(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, false)
	require.Nil(t, ns, "no namespace should be found")
	require.Nil(t, err, "no error either")

	_, err = s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	gs := types.GenesisState{
		Namespaces: []types.Namespace{{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				}, {
					Role:        "blacklisted",
					Permissions: 0,
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: badGuy.String(),
					Roles:   []string{"blacklisted"},
				},
			},
		}},
	}

	// test import
	k.InitGenesis(s.Ctx, gs)

	namespaces, err := k.GetAllNamespaces(s.Ctx)
	require.NoError(t, err)

	require.Equal(t, 1, len(namespaces))
	require.Equal(t, denom, namespaces[0].Denom)

	// test export
	gs = *k.ExportGenesis(s.Ctx)
	require.Equal(t, 1, len(gs.Namespaces))
	require.Equal(t, denom, gs.Namespaces[0].Denom)
}

func BenchmarkHook(b *testing.B) {
	s, k := setupTestSuite(b, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, false)
	require.Nil(b, ns, "no namespace should be found")
	require.Nil(b, err, "no error either")

	_, err = s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(b, err)

	createNamespaceMsg := &types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				}, {
					Role:        "blacklisted",
					Permissions: 0,
				},
			},
			AddressRoles: []*types.AddressRoles{
				{
					Address: badGuy.String(),
					Roles:   []string{"blacklisted"},
				},
			},
		},
	}

	err = createNamespaceMsg.ValidateBasic()
	require.NoError(b, err)

	_, err = s.MsgServer.CreateNamespace(s.Ctx, createNamespaceMsg)
	require.NoError(b, err)

	_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(10000000000))))
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = s.App.BankKeeper.SendCoins(s.Ctx, sender, goodGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(1))))
		require.NoError(b, err)
	}
}
