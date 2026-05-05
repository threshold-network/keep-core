package btcdiff

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
)

func TestVerifySubmissionEligibility(t *testing.T) {
	tests := map[string]struct {
		ready                       bool
		operatorAuthorized          bool
		operatorAuthorizedForRefund bool
		disableProxy                bool
		expectedError               error
	}{
		"chain not ready, enabled proxy": {
			ready:                       false,
			operatorAuthorized:          false,
			operatorAuthorizedForRefund: false,
			disableProxy:                false,
			expectedError:               errNoGenesis,
		},
		"chain not ready, disabled proxy": {
			ready:                       false,
			operatorAuthorized:          false,
			operatorAuthorizedForRefund: false,
			disableProxy:                true,
			expectedError:               errNoGenesis,
		},
		"operator not authorized, enabled proxy": {
			ready:                       true,
			operatorAuthorized:          false,
			operatorAuthorizedForRefund: false,
			disableProxy:                false,
			expectedError:               errNotAuthorized,
		},
		"operator not authorized, disabled proxy": {
			ready:                       true,
			operatorAuthorized:          false,
			operatorAuthorizedForRefund: false,
			disableProxy:                true,
			expectedError:               errNotAuthorized,
		},
		"operator authorized, enabled proxy": {
			ready:                       true,
			operatorAuthorized:          false,
			operatorAuthorizedForRefund: true,
			disableProxy:                false,
			expectedError:               nil,
		},
		"operator authorized, disabled proxy": {
			ready:                       true,
			operatorAuthorized:          true,
			operatorAuthorizedForRefund: false,
			disableProxy:                true,
			expectedError:               nil,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			difficultyChain := connectLocalBitcoinDifficultyChain()
			operatorAddress := difficultyChain.Signing().Address()

			difficultyChain.SetReady(test.ready)
			difficultyChain.SetAuthorizedOperator(
				operatorAddress,
				test.operatorAuthorized,
			)
			difficultyChain.SetAuthorizedForRefundOperator(
				operatorAddress,
				test.operatorAuthorizedForRefund,
			)

			bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
				config: Config{
					DisableProxy:       test.disableProxy,
					IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
					RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
				},
				btcChain: nil,
				chain:    difficultyChain,
			}

			err := bitcoinDifficultyMaintainer.verifySubmissionEligibility()
			testutils.AssertAnyErrorInChainMatchesTarget(
				t,
				test.expectedError,
				err,
			)
		})
	}
}

func TestProveNextEpoch(t *testing.T) {
	tests := []struct {
		name         string
		disableProxy bool
	}{
		{
			name:         "proxy disabled",
			disableProxy: true,
		},
		{
			name:         "proxy enabled",
			disableProxy: false,
		},
	}

	createBlockHeaders := func() map[uint]*bitcoin.BlockHeader {
		return map[uint]*bitcoin.BlockHeader{
			602784: { // First block of epoch 299 (anchor for uniform nBits)
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    999000,
				Bits:                    1111111,
				Nonce:                   1,
			},
			604797: {
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000000,
				Bits:                    1111111,
				Nonce:                   10,
			},
			604798: {
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000100,
				Bits:                    1111111,
				Nonce:                   20,
			},
			604799: { // Last block of the old epoch (epoch 299)
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000200,
				Bits:                    1111111,
				Nonce:                   30,
			},
			604800: { // First block of the new epoch (epoch 300)
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000300,
				Bits:                    2222222,
				Nonce:                   40,
			},
			604801: {
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000400,
				Bits:                    2222222,
				Nonce:                   50,
			},
			604802: {
				Version:                 0,
				PreviousBlockHeaderHash: bitcoin.Hash{},
				MerkleRootHash:          bitcoin.Hash{},
				Time:                    1000500,
				Bits:                    2222222,
				Nonce:                   60,
			},
		}
	}

	runProveNextEpochAssertions := func(
		t *testing.T,
		ctx context.Context,
		bitcoinDifficultyMaintainer *bitcoinDifficultyMaintainer,
		blockHeaders map[uint]*bitcoin.BlockHeader,
		difficultyChain *localBitcoinDifficultyChain,
	) {
		result, err := bitcoinDifficultyMaintainer.proveNextEpoch(ctx)
		if err != nil {
			t.Fatal(err)
		}

		expectedResult := true
		if result != expectedResult {
			t.Fatalf(
				"unexpected result returned\nexpected: %v\nactual:   %v\n",
				expectedResult,
				result,
			)
		}

		var retargetEvents []*RetargetEvent
		if bitcoinDifficultyMaintainer.config.DisableProxy {
			retargetEvents = difficultyChain.RetargetEvents()
		} else {
			retargetEvents = difficultyChain.RetargetWithRefundEvents()
		}

		expectedNumberOfRetargetEvents := 1
		if len(retargetEvents) != expectedNumberOfRetargetEvents {
			t.Fatalf(
				"unexpected number of retarget events\nexpected: %v\nactual:   %v\n",
				expectedNumberOfRetargetEvents,
				len(retargetEvents),
			)
		}

		eventsOldDifficulty := retargetEvents[0].oldDifficulty
		expectedOldDifficulty := blockHeaders[604799].Bits
		if eventsOldDifficulty != expectedOldDifficulty {
			t.Fatalf(
				"unexpected old difficulty of the retarget event \n"+
					"expected: %v\nactual:   %v\n",
				expectedOldDifficulty,
				eventsOldDifficulty,
			)
		}

		eventsNewDifficulty := retargetEvents[0].newDifficulty
		expectedNewDifficulty := blockHeaders[604800].Bits
		if eventsNewDifficulty != expectedNewDifficulty {
			t.Fatalf(
				"unexpected new difficulty of the retarget event \n"+
					"expected: %v\nactual:   %v\n",
				expectedNewDifficulty,
				eventsNewDifficulty,
			)
		}

		// Call once more, this time without any new epoch to prove
		result, err = bitcoinDifficultyMaintainer.proveNextEpoch(ctx)
		if err != nil {
			t.Fatal(err)
		}

		expectedResult = false
		if result != expectedResult {
			t.Fatalf(
				"unexpected result returned\nexpected: %v\nactual:   %v\n",
				expectedResult,
				result,
			)
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			defer cancelCtx()

			btcChain := connectLocalBitcoinChain()
			blockHeaders := createBlockHeaders()
			btcChain.SetBlockHeaders(blockHeaders)

			difficultyChain := connectLocalBitcoinDifficultyChain()
			difficultyChain.SetCurrentEpoch(299)
			difficultyChain.SetProofLength(3)

			bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
				config: Config{
					DisableProxy:       test.disableProxy,
					IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
					RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
				},
				btcChain: btcChain,
				chain:    difficultyChain,
			}

			runProveNextEpochAssertions(
				t,
				ctx,
				bitcoinDifficultyMaintainer,
				blockHeaders,
				difficultyChain,
			)
		})
	}
}

func TestGetBlockHeaders(t *testing.T) {
	btcChain := connectLocalBitcoinChain()

	blockHeaders := map[uint]*bitcoin.BlockHeader{
		700000: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000000,
			Bits:                    1111111,
			Nonce:                   30,
		},
		700001: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000100,
			Bits:                    1111111,
			Nonce:                   40,
		},
		700002: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000200,
			Bits:                    2222222,
			Nonce:                   50,
		},
	}
	btcChain.SetBlockHeaders(blockHeaders)

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    nil,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	headers, err := bitcoinDifficultyMaintainer.getBlockHeaders(700000, 700002)
	if err != nil {
		t.Fatal(err)
	}

	expectedHeaders := []*bitcoin.BlockHeader{
		blockHeaders[700000], blockHeaders[700001], blockHeaders[700002],
	}

	if !reflect.DeepEqual(expectedHeaders, headers) {
		t.Errorf("\nexpected: %v\nactual:   %v", expectedHeaders, headers)
	}
}

func TestWaitForCurrentEpochUpdate_Successful(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	currentEpoch := uint64(299)
	targetEpoch := uint64(300)

	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetCurrentEpoch(currentEpoch)

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: nil,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    2 * time.Second,
			RestartBackOffTime: 2 * time.Second,
		},
	}

	// Run function on a goroutine. The function should wait until the current
	// epoch is set to the target epoch.
	errChan := make(chan error, 1)
	go func() {
		err := bitcoinDifficultyMaintainer.waitForCurrentEpochUpdate(
			ctx,
			targetEpoch,
		)

		errChan <- err
	}()

	// Make sure the function keeps waiting for the current epoch to be updated.
	select {
	case <-time.After(1500 * time.Millisecond):
	case <-errChan:
		t.Fatal("Unexpected return from function")
	}

	// Update the current epoch to allow the waiting function to return.
	difficultyChain.SetCurrentEpoch(targetEpoch)
	select {
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("Function did not return on time")
	case err := <-errChan:
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWaitForCurrentEpochUpdate_Cancelled(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	currentEpoch := uint64(299)
	targetEpoch := uint64(300)

	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetCurrentEpoch(currentEpoch)

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: nil,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    2 * time.Second,
			RestartBackOffTime: 2 * time.Second,
		},
	}

	// Run function on a goroutine. The function should wait until the current
	// epoch is set to the target epoch.
	errChan := make(chan error, 1)
	go func() {
		err := bitcoinDifficultyMaintainer.waitForCurrentEpochUpdate(
			ctx,
			targetEpoch,
		)

		errChan <- err
	}()

	// Cancel context while the function is waiting for current epoch to be
	// updated.
	cancelCtx()

	var err error
	select {
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("Function did not return on time")
	case err = <-errChan:
	}

	testutils.AssertAnyErrorInChainMatchesTarget(t, context.Canceled, err)
}

func TestProveEpochs_ErrorVerifyingSubmissionEligibility(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// Do not authorize the maintainer to trigger an error.
	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetReady(true)

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: nil,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	err := bitcoinDifficultyMaintainer.proveEpochs(ctx)
	testutils.AssertAnyErrorInChainMatchesTarget(t, errNotAuthorized, err)
}

func TestProveEpochs_ErrorProvingSingleEpoch(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	difficultyChain := connectLocalBitcoinDifficultyChain()
	maintainerAddress := difficultyChain.Signing().Address()

	difficultyChain.SetReady(true)
	difficultyChain.SetAuthorizedOperator(
		maintainerAddress,
		true,
	)

	// Do not set block headers in the Bitcoin chain to trigger an error.
	btcChain := connectLocalBitcoinChain()

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    difficultyChain,

		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	err := bitcoinDifficultyMaintainer.proveEpochs(ctx)
	testutils.AssertAnyErrorInChainMatchesTarget(t, errNoBlocksSet, err)
}

func TestProveEpochs_Successful(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	difficultyChain := connectLocalBitcoinDifficultyChain()
	maintainerAddress := difficultyChain.Signing().Address()

	difficultyChain.SetReady(true)
	difficultyChain.SetAuthorizedOperator(
		maintainerAddress,
		true,
	)
	difficultyChain.SetProofLength(1)
	difficultyChain.SetCurrentEpoch(299)

	btcChain := connectLocalBitcoinChain()

	// Set one block header on each side of the retarget. The old epoch number
	// is 299, the new epoch number is 300.
	blockHeaders := map[uint]*bitcoin.BlockHeader{
		602784: { // First block of epoch 299
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    999000,
			Bits:                    1111111,
			Nonce:                   1,
		},
		604799: { // Last block of the old epoch (epoch 299)
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000200,
			Bits:                    1111111,
			Nonce:                   30,
		},
		604800: { // First block of the new epoch (epoch 300)
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000300,
			Bits:                    2222222,
			Nonce:                   40,
		},
	}
	btcChain.SetBlockHeaders(blockHeaders)

	bitcoinDifficultyMaintainer := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    2 * time.Second,
			RestartBackOffTime: 2 * time.Second,
		},
	}

	// Run a goroutine that will cancel the context while the maintainer is
	// proving epochs. Maintainer should prove a single epoch and quit.
	go func() {
		time.Sleep(time.Second)
		cancelCtx()
	}()

	err := bitcoinDifficultyMaintainer.proveEpochs(ctx)
	testutils.AssertAnyErrorInChainMatchesTarget(t, context.Canceled, err)
}

func TestBitcoinDifficultyMaintainer_Integration(t *testing.T) {
	type authorizationFunc func(
		difficultyChain *localBitcoinDifficultyChain,
		address chain.Address,
		authorized bool,
	)

	type retargetEventsFunc func(
		difficultyChain *localBitcoinDifficultyChain,
	) []*RetargetEvent

	tests := []struct {
		name              string
		disableProxy      bool
		setAuthorization  authorizationFunc
		getRetargetEvents retargetEventsFunc
	}{
		{
			name:         "proxy disabled",
			disableProxy: true,
			setAuthorization: func(
				difficultyChain *localBitcoinDifficultyChain,
				addr chain.Address,
				authorized bool,
			) {
				difficultyChain.SetAuthorizedOperator(addr, authorized)
			},
			getRetargetEvents: func(
				difficultyChain *localBitcoinDifficultyChain,
			) []*RetargetEvent {
				return difficultyChain.RetargetEvents()
			},
		},
		{
			name:         "proxy enabled",
			disableProxy: false,
			setAuthorization: func(
				difficultyChain *localBitcoinDifficultyChain,
				addr chain.Address,
				authorized bool,
			) {
				difficultyChain.SetAuthorizedForRefundOperator(addr, authorized)
			},
			getRetargetEvents: func(
				difficultyChain *localBitcoinDifficultyChain,
			) []*RetargetEvent {
				return difficultyChain.RetargetWithRefundEvents()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			defer cancelCtx()

			difficultyChain := connectLocalBitcoinDifficultyChain()
			maintainerAddress := difficultyChain.Signing().Address()

			test.setAuthorization(difficultyChain, maintainerAddress, true)
			difficultyChain.SetReady(true)

			difficultyChain.SetProofLength(1)
			difficultyChain.SetCurrentEpoch(299)

			btcChain := connectLocalBitcoinChain()

			idleBackOffTime := 500 * time.Millisecond
			restartBackOffTime := 1 * time.Second

			config := Config{
				DisableProxy:       test.disableProxy,
				IdleBackOffTime:    idleBackOffTime,
				RestartBackOffTime: restartBackOffTime,
			}

			Initialize(
				ctx,
				config,
				btcChain,
				difficultyChain,
			)

			//************ Loop restart on error ************
			// Do not set any headers in the Bitcoin chain, so that an error is
			// triggered. Wait for a moment to make sure the Bitcoin difficulty
			// maintainer started processing headers.
			time.Sleep(100 * time.Millisecond)

			//************ Prove two epochs ************
			// Set block headers for epochs 300 and 301 in the Bitcoin chain.
			blockHeaders := map[uint]*bitcoin.BlockHeader{
				602784: { // First block of epoch 299
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    999000,
					Bits:                    1111111,
					Nonce:                   1,
				},
				604799: { // Last block of the epoch 299
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000200,
					Bits:                    1111111,
					Nonce:                   30,
				},
				604800: { // First block of the epoch 300
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000300,
					Bits:                    2222222,
					Nonce:                   40,
				},
				606815: { // Last block of the epoch 300
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000400,
					Bits:                    2222222,
					Nonce:                   50,
				},
				606816: { // First block of the epoch 301
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000500,
					Bits:                    3333333,
					Nonce:                   60,
				},
			}
			btcChain.SetBlockHeaders(blockHeaders)

			// Wait for the Bitcoin difficulty maintainer to try processing headers
			// again after the previous attempt that ended in an error.
			time.Sleep(restartBackOffTime)

			// Make sure the first new epoch has been proven.
			expectedNumberOfRetargetEvents := 2

			retargetEvents := test.getRetargetEvents(difficultyChain)
			if len(retargetEvents) != expectedNumberOfRetargetEvents {
				t.Fatalf(
					"unexpected number of retarget events\nexpected: %v\nactual:   %v\n",
					expectedNumberOfRetargetEvents,
					len(retargetEvents),
				)
			}

			eventsOldDifficulty := retargetEvents[0].oldDifficulty
			expectedOldDifficulty := blockHeaders[604799].Bits
			if eventsOldDifficulty != expectedOldDifficulty {
				t.Fatalf(
					"unexpected old difficulty of the retarget event \n"+
						"expected: %v\nactual:   %v\n",
					expectedOldDifficulty,
					eventsOldDifficulty,
				)
			}

			eventsNewDifficulty := retargetEvents[0].newDifficulty
			expectedNewDifficulty := blockHeaders[604800].Bits
			if eventsNewDifficulty != expectedNewDifficulty {
				t.Fatalf(
					"unexpected new difficulty of the retarget event \n"+
						"expected: %v\nactual:   %v\n",
					expectedNewDifficulty,
					eventsNewDifficulty,
				)
			}

			eventsOldDifficulty = retargetEvents[1].oldDifficulty
			expectedOldDifficulty = blockHeaders[606815].Bits
			if eventsOldDifficulty != expectedOldDifficulty {
				t.Fatalf(
					"unexpected old difficulty of the retarget event \n"+
						"expected: %v\nactual:   %v\n",
					expectedOldDifficulty,
					eventsOldDifficulty,
				)
			}

			eventsNewDifficulty = retargetEvents[1].newDifficulty
			expectedNewDifficulty = blockHeaders[606816].Bits
			if eventsNewDifficulty != expectedNewDifficulty {
				t.Fatalf(
					"unexpected new difficulty of the retarget event \n"+
						"expected: %v\nactual:   %v\n",
					expectedNewDifficulty,
					eventsNewDifficulty,
				)
			}

			//************ Cancel context ************
			// Cancel the context to force the Bitcoin difficulty maintainer to stop.
			cancelCtx()

			// Set block headers for epoch 302 in the Bitcoin chain.
			blockHeaders = map[uint]*bitcoin.BlockHeader{
				608831: { // Last block of the epoch 301
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000600,
					Bits:                    3333333,
					Nonce:                   70,
				},
				608832: { // First block of the epoch 302
					Version:                 0,
					PreviousBlockHeaderHash: bitcoin.Hash{},
					MerkleRootHash:          bitcoin.Hash{},
					Time:                    1000700,
					Bits:                    4444444,
					Nonce:                   80,
				},
			}
			btcChain.SetBlockHeaders(blockHeaders)

			// Wait before proceeding with testing. If the Bitcoin difficulty maintainer
			// has not stopped, it will prove another epoch.
			time.Sleep(restartBackOffTime)

			// Make sure the Bitcoin difficulty maintainer has stopped and the number
			// of proven epochs has not changed.
			expectedNumberOfRetargetEvents = 2

			retargetEvents = test.getRetargetEvents(difficultyChain)
			if len(retargetEvents) != expectedNumberOfRetargetEvents {
				t.Fatalf(
					"unexpected number of retarget events\nexpected: %v\nactual:   %v\n",
					expectedNumberOfRetargetEvents,
					len(retargetEvents),
				)
			}
		})
	}
}

func TestProveNextEpoch_RejectsNonUniformPreRetargetDifficulty(t *testing.T) {
	ctx := context.Background()

	btcChain := connectLocalBitcoinChain()
	btcChain.SetBlockHeaders(map[uint]*bitcoin.BlockHeader{
		602784: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1,
			Bits:                    0x1a0082a5,
			Nonce:                   1,
		},
		604799: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    2,
			// Valid compact bits whose expanded target differs from the anchor
			// and from minimum-difficulty (invalid fake values like 9999999 can
			// expand to the same target as other invalid compacts).
			Bits:  0x1b0404cb,
			Nonce: 2,
		},
		604800: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    3,
			Bits:                    0x1c05aaef,
			Nonce:                   3,
		},
	})

	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetReady(true)
	difficultyChain.SetAuthorizedOperator(difficultyChain.Signing().Address(), true)
	difficultyChain.SetProofLength(1)
	difficultyChain.SetCurrentEpoch(299)

	bdm := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	_, err := bdm.proveNextEpoch(ctx)
	if err == nil {
		t.Fatal("expected non-uniform pre-retarget difficulty to be rejected")
	}
	if !errors.Is(err, ErrUniformPreRetargetDifficulty) {
		t.Fatalf("expected wrapped ErrUniformPreRetargetDifficulty; got [%v]", err)
	}
}

func TestProveNextEpoch_IdlesWhenIdleOnPreflightFailure(t *testing.T) {
	ctx := context.Background()

	btcChain := connectLocalBitcoinChain()
	btcChain.SetBlockHeaders(map[uint]*bitcoin.BlockHeader{
		602784: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1,
			Bits:                    0x1a0082a5,
			Nonce:                   1,
		},
		604799: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    2,
			Bits:                    0x1b0404cb,
			Nonce:                   2,
		},
		604800: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    3,
			Bits:                    0x1c05aaef,
			Nonce:                   3,
		},
	})

	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetReady(true)
	difficultyChain.SetAuthorizedOperator(difficultyChain.Signing().Address(), true)
	difficultyChain.SetProofLength(1)
	difficultyChain.SetCurrentEpoch(299)

	bdm := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:           true,
			IdleOnPreflightFailure: true,
			IdleBackOffTime:        bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime:     bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	proven, err := bdm.proveNextEpoch(ctx)
	if err != nil {
		t.Fatalf("expected idle skip without error; got [%v]", err)
	}
	if proven {
		t.Fatal("expected epoch not proven when preflight mismatch is skipped")
	}
}

// TestProveNextEpoch_PreflightAcceptsMinDifficultyPreRetarget ensures the
// maintainer does not reject honest testnet4-style proofs where the last
// block(s) before a retarget use compact bits 0x1d00ffff (DIFF1), matching
// LightRelay.sol (MIN_DIFFICULTY_TARGET exception).
func TestProveNextEpoch_PreflightAcceptsMinDifficultyPreRetarget(t *testing.T) {
	ctx := context.Background()

	btcChain := connectLocalBitcoinChain()
	btcChain.SetBlockHeaders(map[uint]*bitcoin.BlockHeader{
		602784: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    999000,
			Bits:                    1111111,
			Nonce:                   1,
		},
		604799: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000200,
			Bits:                    0x1d00ffff,
			Nonce:                   30,
		},
		604800: {
			Version:                 0,
			PreviousBlockHeaderHash: bitcoin.Hash{},
			MerkleRootHash:          bitcoin.Hash{},
			Time:                    1000300,
			Bits:                    2222222,
			Nonce:                   40,
		},
	})

	difficultyChain := connectLocalBitcoinDifficultyChain()
	difficultyChain.SetReady(true)
	difficultyChain.SetAuthorizedOperator(difficultyChain.Signing().Address(), true)
	difficultyChain.SetProofLength(1)
	difficultyChain.SetCurrentEpoch(299)

	bdm := &bitcoinDifficultyMaintainer{
		btcChain: btcChain,
		chain:    difficultyChain,
		config: Config{
			DisableProxy:       true,
			IdleBackOffTime:    bitcoinDifficultyDefaultIdleBackOffTime,
			RestartBackOffTime: bitcoinDifficultyDefaultRestartBackoffTime,
		},
	}

	proven, err := bdm.proveNextEpoch(ctx)
	if err != nil {
		t.Fatalf("expected min-diff pre-retarget header to pass preflight: %v", err)
	}
	if !proven {
		t.Fatal("expected epoch to be proven")
	}

	retargetEvents := difficultyChain.RetargetEvents()
	if len(retargetEvents) != 1 {
		t.Fatalf("expected 1 retarget event, got %d", len(retargetEvents))
	}
	if retargetEvents[0].oldDifficulty != 0x1d00ffff {
		t.Fatalf("unexpected old difficulty bits in event: %#x", retargetEvents[0].oldDifficulty)
	}
}

func TestRecordIdleTick_EscalatesAtThreshold(t *testing.T) {
	bdm := &bitcoinDifficultyMaintainer{}

	for i := 1; i < idleEscalationThreshold; i++ {
		if bdm.recordIdleTick() {
			t.Fatalf("expected false before threshold (tick %d)", i)
		}
	}
	if !bdm.recordIdleTick() {
		t.Fatalf("expected true at threshold tick %d", idleEscalationThreshold)
	}
	if bdm.consecutiveIdles != idleEscalationThreshold {
		t.Fatalf(
			"unexpected consecutiveIdles: expected %d, got %d",
			idleEscalationThreshold,
			bdm.consecutiveIdles,
		)
	}
}

func TestResetIdleTicks_ZerosCounter(t *testing.T) {
	bdm := &bitcoinDifficultyMaintainer{consecutiveIdles: 15}
	bdm.resetIdleTicks()
	if bdm.consecutiveIdles != 0 {
		t.Fatalf("expected consecutiveIdles=0, got %d", bdm.consecutiveIdles)
	}
}

func TestRecordIdleTick_ResetAndReescalates(t *testing.T) {
	bdm := &bitcoinDifficultyMaintainer{}

	for i := 0; i < idleEscalationThreshold; i++ {
		bdm.recordIdleTick()
	}
	bdm.resetIdleTicks()
	if bdm.consecutiveIdles != 0 {
		t.Fatalf("counter should be zero after reset, got %d", bdm.consecutiveIdles)
	}

	// After reset, next idleEscalationThreshold ticks should escalate again.
	for i := 1; i < idleEscalationThreshold; i++ {
		if bdm.recordIdleTick() {
			t.Fatalf("expected false before threshold after reset (tick %d)", i)
		}
	}
	if !bdm.recordIdleTick() {
		t.Fatal("expected true at threshold tick after reset")
	}
}
