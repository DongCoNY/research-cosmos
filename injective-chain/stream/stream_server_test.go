package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/test"
	"github.com/InjectiveLabs/injective-core/injective-chain/stream/types"
	"github.com/cometbft/cometbft/libs/pubsub"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"sync"
	"testing"
	"time"
)

func TestGracefulShutdown(t *testing.T) {
	var latestSentBlock uint64
	var receivedBlocks = make(map[int]uint64)
	latestBlockCh := make(chan uint64)
	mockInABCIEvents := make(chan baseapp.StreamEvents)

	// chain emits events in mockInABCIEvents channel, but a timeout is set to simulate a shutdown with a termination signal sent in latestBlockCh
	go func() {
		var streamEventsHave []baseapp.StreamEvents
		err := json.Unmarshal(
			test.JSONEventsSet1Have,
			&streamEventsHave)
		if err != nil {
			t.Fatal(err)
		}
		//simulate shutdown
		timeout := time.NewTimer(150 * time.Millisecond)
		defer timeout.Stop()
		var height uint64
		for _, event := range streamEventsHave {
			select {
			case <-timeout.C:
				latestBlockCh <- height
				return
			case mockInABCIEvents <- event:
				time.Sleep(10 * time.Millisecond)
			}
			height = event.Height
		}
	}()

	// stream server and publisher process events from mockInABCIEvents channel
	bus := pubsub.NewServer()
	eventPublisher := NewPublisher(mockInABCIEvents, bus)
	streamServer := NewChainStreamServer(bus)

	// Run the publisher
	err := eventPublisher.Run(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	// Run the stream server
	err = streamServer.Serve("localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	//  when a term signal is received on latestBlockCh, the stream server and publisher stops. Latest sent block is recorded
	go func() {
		latestSentBlock = <-latestBlockCh
		if err = eventPublisher.Stop(); err != nil {
			t.Fatal(err)
		}
		streamServer.Stop()
	}()

	// CLIENTS
	// multiple clients connect to the stream server and receive blocks. The last block received is recorded in receivedBlocks map
	wg := new(sync.WaitGroup)
	mu := new(sync.Mutex)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(clientID int) {
			cc, err := grpc.Dial(fmt.Sprintf("localhost:%d", streamServer.GetCurrentServerPort()), grpc.WithTransportCredentials(insecure.NewCredentials()))
			// nolint:staticcheck //ignored on purpose
			defer cc.Close()
			if err != nil {
				panic(err)
			}
			client := types.NewStreamClient(cc)
			ctx := context.Background()
			stream, err := client.Stream(ctx, types.NewFullStreamRequest())
			if err != nil {
				t.Fatal(err)
			}

			var streamResponse *types.StreamResponse
			for {
				streamResponse, err = stream.Recv()
				if err != nil {
					s, ok := status.FromError(err)
					if ok {
						require.Equal(t, codes.Unavailable, s.Code())
					}
					break
				}
				mu.Lock()
				receivedBlocks[clientID] = streamResponse.BlockHeight
				mu.Unlock()
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	// check that all clients received the recorded latest block
	for _, receivedBlock := range receivedBlocks {
		require.Equal(t, int(latestSentBlock), int(receivedBlock))
	}
}
