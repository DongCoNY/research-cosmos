package fuzztesting

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func f2d(f64Value float64) sdk.Dec {
	return sdk.MustNewDecFromStr(fmt.Sprintf("%f", f64Value))
}

func fn2d(nullableFloat *float64) *sdk.Dec {
	if nullableFloat != nil {
		dec := f2d(*nullableFloat)
		return &dec
	} else {
		return nil
	}
}

func d2nf(decValue sdk.Dec) *float64 {
	f := decValue.MustFloat64()
	return &f
}

func nd2nf(decValue *sdk.Dec) *float64 {
	if decValue == nil {
		return nil
	}
	f := decValue.MustFloat64()
	return &f
}
