package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/test"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types"
	"github.com/cometbft/cometbft/libs/pubsub"
	"github.com/cometbft/cometbft/libs/pubsub/query"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestEventPublisher(t *testing.T) {
	testCases := []struct {
		name             string
		streamEventsHave []byte
		responsesWant    []byte
		err              error
		buffCap          uint
	}{
		{
			name:             "Event publisher normal event processing",
			streamEventsHave: test.JSONEventsSet1Have,
			buffCap:          100,
			responsesWant:    test.JSONResponsesSet1Want,
		},
		{
			name:             "Event publisher error on abci to stream message conversion",
			streamEventsHave: test.JSONEventsSet2ErrHave,
			buffCap:          100,
			err:              fmt.Errorf("error converting ABCI event to BankBalance: failed to unmarshal ABCI event to BankBalance: math/big: cannot unmarshal \"WRONG\" into a *big.Int"),
		},
		{
			name:             "Event publisher buffer overflow",
			streamEventsHave: test.JSONEventsSet1Have,
			buffCap:          1,
			err:              fmt.Errorf("chain stream event buffer overflow"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var streamEventsHave []baseapp.StreamEvents
			err := json.Unmarshal(
				tc.streamEventsHave,
				&streamEventsHave)
			if err != nil {
				t.Fatal(err)
			}
			responsesWant := []*types.StreamResponseMap{}
			if tc.responsesWant != nil {
				err = json.Unmarshal(
					tc.responsesWant,
					&responsesWant)
				if err != nil {
					t.Fatal(err)
				}
			}

			inABCIEvents := make(chan baseapp.StreamEvents)
			bus := pubsub.NewServer()
			publisher := NewPublisher(inABCIEvents, bus).WithBufferCapacity(tc.buffCap)

			// Run the publisher
			publisher.Run(context.TODO())

			// Build a map of events by height for easy assertions
			var streamEventsHaveMap = make(map[uint64]baseapp.StreamEvents)
			for _, events := range streamEventsHave {
				streamEventsHaveMap[events.Height] = events
			}

			// Simulate receiving events
			wgIn := sync.WaitGroup{}
			wgIn.Add(1)
			go func() {
				for _, events := range streamEventsHave {
					inABCIEvents <- events
				}
				wgIn.Done()
			}()

			go func() {
				client_id := uuid.New().String()
				sub, err := bus.Subscribe(context.Background(), client_id, query.Empty{})
				if err != nil {
					t.Fatal(err)
				}
				ch := sub.Out()
				defer bus.Unsubscribe(context.Background(), client_id, query.Empty{})

				var getHeight uint64
				var wantHeight uint64
				for {
					select {
					case message := <-ch:
						if err, ok := message.Data().(error); ok {
							require.Equal(t, tc.err.Error(), err.Error())
							return
						}
						inResp, ok := message.Data().(*types.StreamResponseMap)
						if !ok {
							t.Fatal("unexpected message type")
						}
						if wantHeight == 0 {
							wantHeight = inResp.BlockHeight
						}
						getHeight = inResp.BlockHeight
						require.Equal(t, wantHeight, getHeight)
						wantHeight++
					}
				}
			}()

			wgIn.Wait()
			// Stop the publisher
			publisher.Stop()
		})
	}
}
