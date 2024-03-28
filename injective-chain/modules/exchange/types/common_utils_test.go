package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Min Tick size", func() {
	It("BreachesMinimumTickSize 0.001", func() {
		number, _ := sdk.NewDecFromStr("413416.659")
		minTick, _ := sdk.NewDecFromStr("0.001")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeFalse())
	})

	It("BreachesMinimumTickSize 0.00001", func() {
		number, _ := sdk.NewDecFromStr("45.32528900")
		minTick, _ := sdk.NewDecFromStr("0.000001")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeFalse())
	})

	It("BreachesMinimumTickSize 0.00001", func() {
		number, _ := sdk.NewDecFromStr("16143.19990")
		minTick, _ := sdk.NewDecFromStr("0.0001")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeFalse())
	})

	It("BreachesMinimumTickSize 0.00001", func() {
		number, _ := sdk.NewDecFromStr("16143.19990")
		minTick, _ := sdk.NewDecFromStr("0.001")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeTrue())
	})

	It("BreachesMinimumTickSize 0.000001", func() {
		number, _ := sdk.NewDecFromStr("32072.059684000")
		minTick, _ := sdk.NewDecFromStr("0.0000010000")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeFalse())
	})

	It("BreachesMinimumTickSize 0.1", func() {
		number, _ := sdk.NewDecFromStr("27489.7000000")
		minTick, _ := sdk.NewDecFromStr("0.1000")
		breaches := BreachesMinimumTickSize(number, minTick)
		Expect(breaches).To(BeFalse())
	})
})

var _ = Describe("SubaccountID", func() {
	It("checks default subaccountID correctly", func() {
		subaccountID := EthAddressToSubaccountID(common.HexToAddress("0x199d5ed7f45f4ee35960cf22eade2076e95b253f"))
		isDefault := IsDefaultSubaccountID(subaccountID)
		Expect(isDefault).To(BeTrue())

		subaccountID[19] = 4
		isDefault = IsDefaultSubaccountID(subaccountID)
		Expect(isDefault).To(BeTrue())

		subaccountID = EthAddressToSubaccountID(common.HexToAddress("0x199d5ed7f45f4ee35960cf22eade2076e95b253f"))
		subaccountID[20] = 4
		isDefault = IsDefaultSubaccountID(subaccountID)
		Expect(isDefault).To(BeFalse())

		subaccountID = EthAddressToSubaccountID(common.HexToAddress("0x199d5ed7f45f4ee35960cf22eade2076e95b253f"))
		subaccountID[31] = 4
		isDefault = IsDefaultSubaccountID(subaccountID)
		Expect(isDefault).To(BeFalse())
	})
})

var _ = Describe("Simplified subaccountID", func() {
	var address = sdk.MustAccAddressFromBech32("inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz")
	It("handles empty subaccount id", func() {
		input := "0"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(BeNil(), "error returned for empty subaccount id")

		subaccountId, err := GetNonceDerivedSubaccountID(address, input)
		Expect(err).To(BeNil(), "error returned for empty subaccount id")

		expectedSubaccountId := MustSdkAddressWithNonceToSubaccountID(address, 0)
		Expect(subaccountId.Bytes()).To(Equal(expectedSubaccountId.Bytes()), "wrong subaccount id returned")
	})

	It("handles simplfied default subaccount id", func() {
		input := "0"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(BeNil(), "error returned for simplfied default subaccount id")

		subaccountId, err := GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(BeNil(), "error returned for simplfied default subaccount id")

		expectedSubaccountId := MustSdkAddressWithNonceToSubaccountID(address, 0)
		Expect(subaccountId.Bytes()).To(Equal(expectedSubaccountId.Bytes()), "wrong subaccount id returned")
	})

	It("handles simplfied non-default subaccount id", func() {
		input := "999"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(BeNil(), "error returned for simplfied non-default subaccount id")

		subaccountId, err := GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(BeNil(), "error returned for simplfied non-default subaccount id")

		expectedSubaccountId := MustSdkAddressWithNonceToSubaccountID(address, 999)
		Expect(subaccountId.Bytes()).To(Equal(expectedSubaccountId.Bytes()), "wrong subaccount id returned")
	})

	It("handles full default subaccount id", func() {
		input := MustSdkAddressWithNonceToSubaccountID(address, 0).String()
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(BeNil(), "error returned for full subaccount id")

		subaccountId, err := GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(BeNil(), "error returned for full subaccount id")

		expectedSubaccountId := MustSdkAddressWithNonceToSubaccountID(address, 0)
		Expect(subaccountId.Bytes()).To(Equal(expectedSubaccountId.Bytes()), "wrong subaccount id returned")
	})

	It("returns error for too long simplified subaccount id", func() {
		input := "9999"
		expectedError := input + ": " + ErrBadSubaccountID.Error()
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for too long subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")

		_, err = GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for too long subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")
	})

	It("returns error for non-numeric simplified subaccount id", func() {
		input := "abc"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for non-numeric subaccount id")

		_, err = GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for non-numericsubaccount id")
	})

	It("returns error for mixed simplified subaccount id", func() {
		input := "a2"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for non-numeric subaccount id")

		_, err = GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for non-numeric subaccount id")
	})

	It("returns error for negative simplified subaccount id", func() {
		input := "-1"
		err := CheckValidSubaccountIDOrNonce(address, input)
		expectedError := input + ": " + ErrBadSubaccountID.Error()
		Expect(err).To(Not(BeNil()), "no error returned for negative subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")

		expectedError = ErrBadSubaccountNonce.Error()
		_, err = GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for negative subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")
	})

	It("returns error for simplified subaccount id with decimal fraction", func() {
		input := "1.2"
		err := CheckValidSubaccountIDOrNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for subaccount id with decimal fraction")

		_, err = GetSubaccountIDOrDeriveFromNonce(address, input)
		Expect(err).To(Not(BeNil()), "no error returned for subaccount id with decimal fraction")
	})

	It("returns error for too long subaccount id", func() {
		subaccountId := MustSdkAddressWithNonceToSubaccountID(address, 0).String() + "0"
		err := CheckValidSubaccountIDOrNonce(address, subaccountId)
		expectedError := subaccountId + ": " + ErrBadSubaccountID.Error()
		Expect(err).To(Not(BeNil()), "no error returned for too long subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")

		_, err = GetSubaccountIDOrDeriveFromNonce(address, subaccountId)
		Expect(err).To(Not(BeNil()), "no error returned for too long subaccount id")
		Expect(err.Error()).To(Equal(expectedError), "wrong error returned")
	})
})
