package testexchange

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/onsi/gomega/types"

	"fmt"
)

func BeApproximately(expected sdk.Dec) types.GomegaMatcher {
	return &approximatelyDecMatcher{
		expected: expected,
		maxDiff:  sdk.SmallestDec().MulInt64(7000),
	}
}

func BeApproximatelyWithMaxDiff(expected, maxDiff sdk.Dec) types.GomegaMatcher {
	return &approximatelyDecMatcher{
		expected: expected,
		maxDiff:  maxDiff,
	}
}

type approximatelyDecMatcher struct {
	expected sdk.Dec
	maxDiff  sdk.Dec
}

func (matcher *approximatelyDecMatcher) Match(actual interface{}) (success bool, err error) {
	actualNumber, ok := actual.(sdk.Dec)

	if !ok {
		return false, fmt.Errorf("approximatelyDecMatcher matcher expects an sdk.Dec")
	}

	diff := matcher.expected.Sub(actualNumber).Abs()
	return diff.LTE(matcher.maxDiff), nil
}

func (matcher *approximatelyDecMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected: %#v to be within %#v of the value \nReceived: %#v", matcher.expected, matcher.maxDiff, actual)
}

func (matcher *approximatelyDecMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected: %#v to not be within %#v of the value \nReceived: %#v", matcher.expected, matcher.maxDiff, actual)
}
