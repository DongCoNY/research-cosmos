package keeper_test

import (
	"testing"

	"golang.org/x/exp/slices"

	"cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/stretchr/testify/require"

	exchangetypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/exchange/types"
	"github.com/InjectiveLabs/injective-core/injective-chain/modules/permissions/types"
	tftypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

func TestMsgServerCreateNamespace(t *testing.T) {
	s, k := setupTestSuite(t, 2)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := types.MsgCreateNamespace{
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

	// not our denom
	wrongMsg := createNamespaceMsg
	wrongMsg.Namespace.Denom += "zzz"
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized), err)
	wrongMsg = createNamespaceMsg
	wrongMsg.Sender = badGuy.String()
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized), err)
	// not a tokenfactory denom
	wrongMsg = createNamespaceMsg
	wrongMsg.Namespace.Denom = "inj"
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, tftypes.ErrInvalidDenom), err)
	// no EVERYONE role defined
	wrongMsg = createNamespaceMsg
	wrongMsg.Namespace.RolePermissions = slices.Clone(createNamespaceMsg.Namespace.RolePermissions)
	slices.Delete(wrongMsg.Namespace.RolePermissions, 0, 1) // delete the EVERYONE role
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidPermission), err)
	// invalid permissions for role
	wrongMsg.Namespace.RolePermissions = slices.Clone(createNamespaceMsg.Namespace.RolePermissions)
	wrongMsg.Namespace.RolePermissions[0].Permissions = 10
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidPermission), err)
	// invalid address for role
	wrongMsg.Namespace.RolePermissions = slices.Clone(createNamespaceMsg.Namespace.RolePermissions)
	wrongMsg.Namespace.AddressRoles = slices.Clone(createNamespaceMsg.Namespace.AddressRoles)
	wrongMsg.Namespace.AddressRoles = append(wrongMsg.Namespace.AddressRoles, &types.AddressRoles{
		Address: "bezoomny",
		Roles:   []string{"blacklisted"},
	})
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// no defined role found
	wrongMsg.Namespace.AddressRoles = slices.Clone(createNamespaceMsg.Namespace.AddressRoles)
	wrongMsg.Namespace.AddressRoles = append(wrongMsg.Namespace.AddressRoles, &types.AddressRoles{
		Address: badGuy.String(),
		Roles:   []string{"macarena"},
	})
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidPermission), err)
	// everyone explicitly assigned
	wrongMsg.Namespace.AddressRoles = slices.Clone(createNamespaceMsg.Namespace.AddressRoles)
	wrongMsg.Namespace.AddressRoles = append(wrongMsg.Namespace.AddressRoles, &types.AddressRoles{
		Address: badGuy.String(),
		Roles:   []string{types.EVERYONE},
	})
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidPermission), err)
	// invalid wasm address
	wrongMsg = createNamespaceMsg
	wrongMsg.Namespace.WasmHook = "hook_address"
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// unknown wasm address
	wrongMsg = createNamespaceMsg
	wrongMsg.Namespace.WasmHook = badGuy.String()
	wrongMsg.Namespace.RolePermissions[0].Permissions = uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnknownWasmHook), err)
	// namespace already exists
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.True(t, errors.IsOf(err, types.ErrDenomNamespaceExists), err)
	// governance can create namespace for any TF denom
	wrongMsg = createNamespaceMsg
	wrongMsg.Sender = authtypes.NewModuleAddress(govtypes.ModuleName).String()
	wrongMsg.Namespace.Denom += "cal"
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// check that created namespaces are good
	denomNs, err := k.GetNamespaceForDenom(s.Ctx, denom, true)
	require.NoError(t, err)
	require.Equal(t, createNamespaceMsg.Namespace, *denomNs)
	denomNs, err = k.GetNamespaceForDenom(s.Ctx, denom+"cal", true)
	require.NoError(t, err)
	require.Equal(t, wrongMsg.Namespace, *denomNs)
}

func TestMsgServerDeleteNamespace(t *testing.T) {
	s, k := setupTestSuite(t, 2)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	var createNamespaceMsg = types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
				{
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
	deleteNamespaceMsg := types.MsgDeleteNamespace{
		Sender:         sender.String(),
		NamespaceDenom: denom,
	}

	err = deleteNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	// incorrect denom
	wrongMsg := deleteNamespaceMsg
	wrongMsg.NamespaceDenom = "zzz"
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// unknown denom
	wrongMsg = deleteNamespaceMsg
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnknownDenom), err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// do not have rights to delete
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	wrongMsg.Sender = badGuy.String()
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized), err)
	// governance can delete
	wrongMsg.Sender = authtypes.NewModuleAddress(govtypes.ModuleName).String()
	_, err = s.MsgServer.DeleteNamespace(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// check that namespace was in fact deleted
	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, true)
	require.NoError(t, err)
	require.Nil(t, ns)

}

func TestMsgServerUpdateNamespace(t *testing.T) {
	s, k := setupTestSuite(t, 2)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]

	subdenom := "awd"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
				{
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
	updateNamespaceMsg := types.MsgUpdateNamespace{
		Sender:         sender.String(),
		NamespaceDenom: denom,
		SendsPaused:    &types.MsgUpdateNamespace_MsgSetSendsPaused{NewValue: true},
	}

	err = updateNamespaceMsg.ValidateBasic()
	require.NoError(t, err)

	// incorrect denom
	wrongMsg := updateNamespaceMsg
	wrongMsg.NamespaceDenom = "zzz"
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// unknown denom
	wrongMsg = updateNamespaceMsg
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnknownDenom), err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// do not have rights to delete
	wrongMsg.Sender = badGuy.String()
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized), err)
	// governance can update
	wrongMsg.Sender = authtypes.NewModuleAddress(govtypes.ModuleName).String()
	_, err = s.MsgServer.UpdateNamespace(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// check that namespace was in fact updated
	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, true)
	require.NoError(t, err)
	require.Equal(t, ns.SendsPaused, true)
	require.Equal(t, ns.MintsPaused, false)
	require.Equal(t, ns.BurnsPaused, false)
}

func TestMsgServerUpdateNamespaceRoles(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	subdenom := "chelloveck"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
				{
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

	updateNamespaceRolesMsg := types.MsgUpdateNamespaceRoles{
		Sender:         sender.String(),
		NamespaceDenom: denom,
		RolePermissions: []*types.Role{
			{
				Role:        "devotchka",
				Permissions: 0,
			},
			{
				Role:        "bog",
				Permissions: 2,
			},
		},
		AddressRoles: []*types.AddressRoles{
			{
				Address: goodGuy.String(),
				Roles:   []string{"devotchka"},
			},
		},
	}
	wrongMsg := updateNamespaceRolesMsg
	cloneMsg := func() {
		wrongMsg = updateNamespaceRolesMsg
		wrongMsg.RolePermissions = slices.Clone(updateNamespaceRolesMsg.RolePermissions)
		wrongMsg.AddressRoles = slices.Clone(updateNamespaceRolesMsg.AddressRoles)
	}

	err = updateNamespaceRolesMsg.ValidateBasic()
	require.NoError(t, err)
	// incorrect denom
	cloneMsg()
	wrongMsg.NamespaceDenom = "droog"
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// unknown denom
	cloneMsg()
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnknownDenom), err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// incorrect role permissions
	cloneMsg()
	wrongMsg.RolePermissions = append(wrongMsg.RolePermissions, &types.Role{
		Role:        "koshka",
		Permissions: 999,
	})
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidPermission), err)
	// invalid address roles
	cloneMsg()
	wrongMsg.AddressRoles = append(wrongMsg.AddressRoles, &types.AddressRoles{
		Address: "horrorshow",
		Roles:   []string{"devotchka"},
	})
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// EVERYONE explicit role is prohibited
	cloneMsg()
	wrongMsg.AddressRoles = append(wrongMsg.AddressRoles, &types.AddressRoles{
		Address: goodGuy.String(),
		Roles:   []string{types.EVERYONE},
	})
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrInvalidRole), err)
	// should be denom admin or gov to update roles
	cloneMsg()
	wrongMsg.Sender = badGuy.String()
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnauthorized), err)
	wrongMsg.Sender = authtypes.NewModuleAddress(govtypes.ModuleName).String()
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	// unknown role
	cloneMsg()
	wrongMsg.AddressRoles = append(wrongMsg.AddressRoles, &types.AddressRoles{
		Address: goodGuy.String(),
		Roles:   []string{"rooker"},
	})
	_, err = s.MsgServer.UpdateNamespaceRoles(s.Ctx, &wrongMsg)
	// TODO: CHECK ME
	require.True(t, errors.IsOf(err, types.ErrInvalidRole), err)
	// check that namespace was in fact updated
	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, true)
	require.NoError(t, err)
	require.ElementsMatch(t, ns.RolePermissions, []*types.Role{
		{
			Role:        types.EVERYONE,
			Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
		}, {
			Role:        "blacklisted",
			Permissions: 0,
		}, {
			Role:        "devotchka",
			Permissions: 0,
		}, {
			Role:        "bog",
			Permissions: 2,
		},
	})
	require.ElementsMatch(t, ns.AddressRoles, []*types.AddressRoles{
		{
			Address: badGuy.String(),
			Roles:   []string{"blacklisted"},
		},
		{
			Address: goodGuy.String(),
			Roles:   []string{"devotchka"},
		},
	})
}

func TestMsgServerRevokeNamespaceRoles(t *testing.T) {
	s, k := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]
	goodGuy := s.TestAccs[2]

	subdenom := "chelloveck"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
				{
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

	revokeNamespaceRolesMsg := types.MsgRevokeNamespaceRoles{
		Sender:         sender.String(),
		NamespaceDenom: denom,
		AddressRolesToRevoke: []*types.AddressRoles{
			{
				Address: badGuy.String(),
				Roles:   []string{"blacklisted"},
			}, {
				Address: goodGuy.String(),
				Roles:   []string{"devotchka"},
			},
		},
	}
	wrongMsg := revokeNamespaceRolesMsg
	cloneMsg := func() {
		wrongMsg = revokeNamespaceRolesMsg
		wrongMsg.AddressRolesToRevoke = slices.Clone(revokeNamespaceRolesMsg.AddressRolesToRevoke)
	}

	err = revokeNamespaceRolesMsg.ValidateBasic()
	require.NoError(t, err)
	// incorrect denom
	cloneMsg()
	wrongMsg.NamespaceDenom = "droog"
	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, &wrongMsg)
	require.Error(t, err)
	// unknown denom
	cloneMsg()
	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, &wrongMsg)
	require.True(t, errors.IsOf(err, types.ErrUnknownDenom), err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, &wrongMsg)
	require.NoError(t, err)
	ns, err := k.GetNamespaceForDenom(s.Ctx, denom, true)
	require.NoError(t, err)
	require.Equal(t, ns.AddressRoles, []*types.AddressRoles{})
}

func TestMsgServerClaimVoucher(t *testing.T) {
	s, _ := setupTestSuite(t, 3)

	sender := s.TestAccs[0]
	badGuy := s.TestAccs[1]

	exchangeModuleAddress := authtypes.NewModuleAddress(exchangetypes.ModuleName)
	permissionsModuleAddress := authtypes.NewModuleAddress(types.ModuleName)

	subdenom := "chelloveck"
	denom := formatDenom(sender.String(), subdenom)

	_, err := s.TFMsgServer.CreateDenom(s.Ctx, tftypes.NewMsgCreateDenom(sender.String(), subdenom, subdenom, subdenom))
	require.NoError(t, err)

	createNamespaceMsg := types.MsgCreateNamespace{
		Sender: sender.String(),
		Namespace: types.Namespace{
			Denom: denom,
			RolePermissions: []*types.Role{
				{
					Role:        types.EVERYONE,
					Permissions: uint32(types.Action_MINT | types.Action_RECEIVE | types.Action_BURN),
				},
				{
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

	claimVoucherMsg := types.MsgClaimVoucher{
		Sender:     badGuy.String(),
		Originator: exchangeModuleAddress.String(),
	}
	err = claimVoucherMsg.ValidateBasic()
	require.NoError(t, err)
	_, err = s.MsgServer.CreateNamespace(s.Ctx, &createNamespaceMsg)
	require.NoError(t, err)

	// test claiming non-existing voucher
	_, err = s.MsgServer.ClaimVoucher(s.Ctx, &claimVoucherMsg)
	require.True(t, errors.IsOf(err, types.ErrVoucherNotFound), err)
	// test claiming voucher when still restricted
	_, err = s.TFMsgServer.Mint(s.Ctx, tftypes.NewMsgMint(sender.String(), sdk.NewCoin(denom, sdk.NewInt(100))))
	require.NoError(t, err)

	err = s.App.BankKeeper.SendCoins(s.Ctx, sender, exchangeModuleAddress, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(100))))
	require.NoError(t, err)

	err = s.App.BankKeeper.SendCoinsFromModuleToAccount(s.Ctx, exchangetypes.ModuleName, badGuy, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(10))))
	require.NoError(t, err)
	require.Equal(t, "10"+denom, s.App.BankKeeper.GetBalance(s.Ctx, permissionsModuleAddress, denom).String())
	require.Equal(t, "0"+denom, s.App.BankKeeper.GetBalance(s.Ctx, badGuy, denom).String())

	_, err = s.MsgServer.ClaimVoucher(s.Ctx, &claimVoucherMsg)
	require.True(t, errors.IsOf(err, types.ErrRestrictedAction), err)
	require.Equal(t, "10"+denom, s.App.BankKeeper.GetBalance(s.Ctx, permissionsModuleAddress, denom).String())
	require.Equal(t, "0"+denom, s.App.BankKeeper.GetBalance(s.Ctx, badGuy, denom).String())
	// now we add badGuy address to whitelisted and they should be able to claim
	revokeNamespaceRolesMsg := types.MsgRevokeNamespaceRoles{
		Sender:         sender.String(),
		NamespaceDenom: denom,
		AddressRolesToRevoke: []*types.AddressRoles{
			{
				Address: badGuy.String(),
				Roles:   []string{"blacklisted"},
			},
		},
	}
	_, err = s.MsgServer.RevokeNamespaceRoles(s.Ctx, &revokeNamespaceRolesMsg)
	require.NoError(t, err)
	_, err = s.MsgServer.ClaimVoucher(s.Ctx, &claimVoucherMsg)
	require.NoError(t, err)
	require.Equal(t, "0"+denom, s.App.BankKeeper.GetBalance(s.Ctx, permissionsModuleAddress, denom).String())
	require.Equal(t, "10"+denom, s.App.BankKeeper.GetBalance(s.Ctx, badGuy, denom).String())
	// claiming again should fail
	_, err = s.MsgServer.ClaimVoucher(s.Ctx, &claimVoucherMsg)
	require.True(t, errors.IsOf(err, types.ErrVoucherNotFound), err)
}
