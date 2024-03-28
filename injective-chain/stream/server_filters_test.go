package stream

import (
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

var pos1 = &types.Position{MarketId: "market1", SubaccountId: "subaccount1"}
var pos2 = &types.Position{MarketId: "market2", SubaccountId: "subaccount2"}
var pos3 = &types.Position{MarketId: "market1", SubaccountId: "subaccount2"}
var pos4 = &types.Position{MarketId: "market2", SubaccountId: "subaccount1"}
var pos5 = &types.Position{MarketId: "market1", SubaccountId: "subaccount3"}

var testCases = []struct {
	name         string
	firstMap     map[string][]*types.Position
	secondMap    map[string][]*types.Position
	firstFilter  []string
	secondFilter []string
	expected     []*types.Position
	error        error
}{
	{
		name: "Both Filters Non-Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1},
			"market2": {pos2},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1},
			"subaccount2": {pos2},
		},
		firstFilter:  []string{"market1", "market2"},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     []*types.Position{pos1, pos2},
	},
	{
		name: "Both Filters Non-Empty with intersection",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{"market1"},
		secondFilter: []string{"subaccount2"},
		expected:     []*types.Position{pos3},
	},
	{
		name: "First Filter Non-Empty, Second Filter Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{"market1"},
		secondFilter: []string{},
		expected:     []*types.Position{pos1, pos3, pos5},
	},
	{
		name: "Second Filter Non-Empty, First Filter Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     []*types.Position{pos1, pos2, pos3, pos4},
	},
	{
		name: "First Filter Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1},
			"market2": {pos2},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1},
			"subaccount2": {pos2},
		},
		firstFilter:  []string{},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     []*types.Position{pos1, pos2},
	},
	{
		name: "Second Filter Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1},
			"market2": {pos2},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1},
			"subaccount2": {pos2},
		},
		firstFilter:  []string{"market1", "market2"},
		secondFilter: []string{},
		expected:     []*types.Position{pos1, pos2},
	},
	{
		name: "Both Filters Empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1},
			"market2": {pos2},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1},
			"subaccount2": {pos2},
		},
		firstFilter:  []string{},
		secondFilter: []string{},
		expected:     nil,
	},
	{
		name:         "Both Maps Empty",
		firstMap:     map[string][]*types.Position{},
		secondMap:    map[string][]*types.Position{},
		firstFilter:  []string{"market1", "market2"},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     nil,
	},
	{
		name:         "First Map Empty",
		firstMap:     map[string][]*types.Position{},
		secondMap:    map[string][]*types.Position{"subaccount1": {pos1}, "subaccount2": {pos2}},
		firstFilter:  []string{"market1", "market2"},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     nil,
		error:        ErrInvalidParameters,
	},
	{
		name:         "Second Map Empty",
		firstMap:     map[string][]*types.Position{"market1": {pos1}, "market2": {pos2}},
		secondMap:    map[string][]*types.Position{},
		firstFilter:  []string{"market1", "market2"},
		secondFilter: []string{"subaccount1", "subaccount2"},
		expected:     nil,
		error:        ErrInvalidParameters,
	},
	{
		name: "First filter is wildcard, second is filled",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{"*"},
		secondFilter: []string{"subaccount2"},
		expected:     []*types.Position{pos2, pos3},
	},
	{
		name: "Second filter is wildcard, first is filled",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{"market1"},
		secondFilter: []string{"*"},
		expected:     []*types.Position{pos1, pos3, pos5},
	},
	{
		name: "First filter is wildcard, second is empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{"*"},
		secondFilter: []string{},
		expected:     []*types.Position{pos1, pos2, pos3, pos4, pos5},
	},
	{
		name: "Second filter is wildcard, first is empty",
		firstMap: map[string][]*types.Position{
			"market1": {pos1, pos3, pos5},
			"market2": {pos2, pos4},
		},
		secondMap: map[string][]*types.Position{
			"subaccount1": {pos1, pos4},
			"subaccount2": {pos2, pos3},
			"subaccount3": {pos5},
		},
		firstFilter:  []string{},
		secondFilter: []string{"*"},
		expected:     []*types.Position{pos1, pos2, pos3, pos4, pos5},
	},
}

func TestFilterMulti(t *testing.T) {
	// Create shared instances of types.Position
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := FilterMulti(tc.firstMap, tc.secondMap, tc.firstFilter, tc.secondFilter)
			assert.ElementsMatch(t, tc.expected, result)
			assert.Equal(t, tc.error, err)
		})
	}
}
