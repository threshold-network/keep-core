//go:build integration

package electrum_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slices"

	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/bitcoin/electrum"

	testData "github.com/keep-network/keep-core/internal/testdata/bitcoin"

	_ "unsafe"

	_ "github.com/keep-network/keep-core/config"
)

const requestTimeout = 5 * time.Second
const requestRetryTimeout = requestTimeout * 2

const blockDelta = 2

type testConfig struct {
	clientConfig electrum.Config
	network      bitcoin.Network
}

// Servers details were taken from a public Electrum servers list published
// at https://1209k.com/bitcoin-eye/ele.php?chain=tbtc.
var testConfigs = map[string]testConfig{
	"electrs-esplora tcp": {
		clientConfig: electrum.Config{
			URL:                 "tcp://electrum.blockstream.info:60001",
			RequestTimeout:      requestTimeout * 2,
			RequestRetryTimeout: requestRetryTimeout * 2,
		},
		network: bitcoin.Testnet,
	},
	"electrs-esplora ssl": {
		clientConfig: electrum.Config{
			URL:                 "ssl://electrum.blockstream.info:60002",
			RequestTimeout:      requestTimeout * 2,
			RequestRetryTimeout: requestRetryTimeout * 2,
		},
		network: bitcoin.Testnet,
	},
	"electrumx wss": {
		clientConfig: electrum.Config{
			URL:                 "wss://electrumx-server.test.tbtc.network:8443",
			RequestTimeout:      requestTimeout,
			RequestRetryTimeout: requestRetryTimeout,
		},
		network: bitcoin.Testnet,
	},
	"fulcrum tcp": {
		clientConfig: electrum.Config{
			URL:                 "tcp://v22019051929289916.bestsrv.de:50001",
			RequestTimeout:      requestTimeout * 2,
			RequestRetryTimeout: requestRetryTimeout * 2,
		},
		network: bitcoin.Testnet,
	},
}

var invalidTxID bitcoin.Hash

//go:linkname readEmbeddedServers github.com/keep-network/keep-core/config.readElectrumUrls
func readEmbeddedServers(network bitcoin.Network) ([]string, error)

func init() {
	var err error

	readServers := func(network bitcoin.Network) error {
		servers, err := readEmbeddedServers(network)
		if err != nil {
			return err
		}

		for _, server := range servers {
			serverName := fmt.Sprintf("embedded/%s/%s", network.String(), server)
			testConfigs[serverName] = testConfig{
				clientConfig: electrum.Config{
					URL:                 server,
					RequestTimeout:      requestTimeout,
					RequestRetryTimeout: requestRetryTimeout,
				},
				network: network,
			}
		}
		return nil
	}

	if err := readServers(bitcoin.Testnet); err != nil {
		panic(err)
	}

	if err := readServers(bitcoin.Mainnet); err != nil {
		panic(err)
	}

	// Remove duplicates
	urls := make(map[string]string)
	for key, server := range testConfigs {
		firstName, ok := urls[server.clientConfig.URL]
		if ok {
			delete(testConfigs, key)
			fmt.Printf(
				"removed server [%s] as a server with the same URL [%s] is already registered under [%s] name\n",
				key,
				server.clientConfig.URL,
				firstName,
			)
			continue
		}
		urls[server.clientConfig.URL] = key
	}

	invalidTxID, err = bitcoin.NewHashFromString(
		"9489457dc2c5a461a0b86394741ef57731605f2c628102de9f4d90afee9ac794",
		bitcoin.ReversedByteOrder,
	)
	if err != nil {
		panic(err)
	}
}

func TestConnect_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		_, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()
	})
}

func TestGetTransaction_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		for txName, tx := range testData.Transactions[testConfig.network] {
			t.Run(txName, func(t *testing.T) {
				result, err := electrum.GetTransaction(tx.TxHash)
				if err != nil {
					t.Fatal(err)
				}

				expectedResult := &tx.BitcoinTx
				if diff := deep.Equal(result, expectedResult); diff != nil {
					t.Errorf(
						"compare failed: %v\nactual: %s\nexpected: %s",
						diff,
						toJson(result),
						toJson(expectedResult),
					)
				}
			})
		}
	})
}

func TestGetTransaction_Negative_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		_, err := electrum.GetTransaction(invalidTxID)

		expectedErr := fmt.Errorf(
			"failed to get raw transaction with ID [%s]: [not found]",
			invalidTxID.Hex(bitcoin.ReversedByteOrder),
		)
		if !reflect.DeepEqual(expectedErr, err) {
			t.Errorf(
				"unexpected error\n"+
					"expected: %v\n"+
					"actual:   %v\n",
				expectedErr,
				err,
			)
		}
	})
}

func TestGetTransactionConfirmations_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		for txName, tx := range testData.Transactions[testConfig.network] {
			t.Run(txName, func(t *testing.T) {
				latestBlockHeight, err := electrum.GetLatestBlockHeight()
				if err != nil {
					t.Fatalf("failed to get the latest block height: %s", err)
				}
				expectedConfirmations := latestBlockHeight - tx.BlockHeight

				result, err := electrum.GetTransactionConfirmations(tx.TxHash)
				if err != nil {
					t.Fatal(err)
				}

				assertNumberCloseTo(t, expectedConfirmations, result, blockDelta)
			})
		}

		// We add sleep as a workaround for https://github.com/checksum0/go-electrum/issues/10
		time.Sleep(time.Second)
	})
}

func TestGetTransactionConfirmations_Negative_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		_, err := electrum.GetTransactionConfirmations(invalidTxID)

		expectedErr := fmt.Errorf(
			"failed to get raw transaction with ID [%s]: [not found]",
			invalidTxID.Hex(bitcoin.ReversedByteOrder),
		)
		if !reflect.DeepEqual(expectedErr, err) {
			t.Errorf(
				"unexpected error\n"+
					"expected: %v\n"+
					"actual:   %v\n",
				expectedErr,
				err,
			)
		}
	})
}

// TODO: We should uncomment this test once https://github.com/checksum0/go-electrum/issues/10
// is fixed. This test was added to validate the fix of the following issue
// https://github.com/keep-network/keep-core/issues/3699 but at the same time
// made `panic: assignment to entry in nil map` happen very frequently which is
// disturbing during the development and running the existing integration tests.

// func TestGetLatestBlockHeightConcurrently_Integration(t *testing.T) {
// 	goroutines := 20

// 	for testName, testConfig := range testConfigs {
// 		t.Run(testName+"_get", func(t *testing.T) {
// 			electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
// 			defer cancelCtx()

// 			var wg sync.WaitGroup

// 			for i := 0; i < goroutines; i++ {
// 				wg.Add(1)

// 				go func() {
// 					result, err := electrum.GetLatestBlockHeight()

// 					if err != nil {
// 						t.Fatal(err)
// 					}

// 					if result == 0 {
// 						t.Errorf(
// 							"returned block height is 0",
// 						)
// 					}

// 					wg.Done()
// 				}()
// 			}

// 			wg.Wait()
// 		})

// 		// Passed if no "panic: concurrent write to websocket connection"
// 	}
// }

func TestGetLatestBlockHeight_Integration(t *testing.T) {
	expectedBlockHeightRef := map[string]uint{}
	results := map[string]map[string]uint{}

	for testName, testConfig := range testConfigs {
		t.Run(testName+"_get", func(t *testing.T) {
			skipTestnetWSS(t, testName, testConfig)

			electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
			defer cancelCtx()

			result, err := electrum.GetLatestBlockHeight()
			if err != nil {
				t.Fatal(err)
			}

			if result == 0 {
				t.Errorf(
					"returned block height is 0",
				)
			}

			if _, ok := results[testConfig.network.String()]; !ok {
				results[testConfig.network.String()] = map[string]uint{}
			}
			results[testConfig.network.String()][testName] = result

			ref := expectedBlockHeightRef[testConfig.network.String()]
			// Store the highest value as a reference.
			if result > ref {
				expectedBlockHeightRef[testConfig.network.String()] = result
			}
		})
	}

	for testName, config := range testConfigs {
		t.Run(testName+"_compare", func(t *testing.T) {
			skipTestnetWSS(t, testName, config)

			result := results[config.network.String()][testName]
			ref := expectedBlockHeightRef[config.network.String()]

			assertNumberCloseTo(t, ref, result, blockDelta)
		})
	}
}

func TestGetBlockHeader_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		blockData, ok := testData.Blocks[testConfig.network]
		if !ok {
			t.Fatalf("block test data not defined for network %s", testConfig.network)
		}

		result, err := electrum.GetBlockHeader(blockData.BlockHeight)
		if err != nil {
			t.Fatal(err)
		}

		if diff := deep.Equal(result, blockData.BlockHeader); diff != nil {
			t.Errorf("compare failed: %v", diff)
		}
	})
}

func TestGetBlockHeader_Negative_Integration(t *testing.T) {
	blockHeight := uint(math.MaxUint32)

	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		_, err := electrum.GetBlockHeader(blockHeight)

		assertMissingBlockHeaderError(
			t,
			testConfig.clientConfig,
			"failed to get block header",
			err,
		)
	})
}

func TestGetTransactionMerkleProof_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		txMerkleProofData, ok := testData.TxMerkleProofs[testConfig.network]
		if !ok {
			t.Fatalf(
				"transaction merkle proof data not defined for network %s",
				testConfig.network,
			)
		}

		transactionHash := txMerkleProofData.TxHash
		blockHeight := txMerkleProofData.BlockHeight

		expectedResult := txMerkleProofData.MerkleProof

		result, err := electrum.GetTransactionMerkleProof(
			transactionHash,
			blockHeight,
		)
		if err != nil {
			t.Fatal(err)
		}

		if diff := deep.Equal(result, expectedResult); diff != nil {
			t.Errorf("compare failed: %v", diff)
		}
	})
}

func TestGetTransactionMerkleProof_Negative_Integration(t *testing.T) {
	blockHeight := uint(123456)

	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		_, err := electrum.GetTransactionMerkleProof(
			invalidTxID,
			blockHeight,
		)

		assertMissingTransactionInBlockError(
			t,
			testConfig.clientConfig,
			"failed to get merkle proof",
			err,
		)
	})
}

func TestGetTransactionsForPublicKeyHash_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		txMerkleProofData, ok := testData.TransactionsForPublicKeyHash[testConfig.network]
		if !ok {
			t.Fatalf(
				"transactions for public key hash data not defined for network %s",
				testConfig.network,
			)
		}

		publicKeyHash := (*[20]byte)(txMerkleProofData.PublicKeyHash)
		expectedHashes := txMerkleProofData.Transactions

		transactions, err := electrum.GetTransactionsForPublicKeyHash(*publicKeyHash, 5)
		if err != nil {
			t.Fatal(err)
		}

		actualHashes := make([]bitcoin.Hash, len(transactions))
		for i, transaction := range transactions {
			actualHashes[i] = transaction.Hash()
		}

		if diff := deep.Equal(actualHashes, expectedHashes); diff != nil {
			t.Errorf("compare failed: %v", diff)
		}
	})
}

func TestGetTxHashesForPublicKeyHash_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		data, ok := testData.TransactionsForPublicKeyHash[testConfig.network]
		if !ok {
			t.Fatalf(
				"transactions for public key hash data not defined for network %s",
				testConfig.network,
			)
		}

		publicKeyHash := (*[20]byte)(data.PublicKeyHash)
		expectedHashes := data.Transactions

		actualHashes, err := electrum.GetTxHashesForPublicKeyHash(*publicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		// If the actual hashes set is greater than the expected set, we need
		// to adjust them to the same length to make a comparison that makes sense.
		if len(actualHashes) > len(expectedHashes) {
			actualHashes = actualHashes[len(actualHashes)-len(expectedHashes):]
		}

		if diff := deep.Equal(actualHashes, expectedHashes); diff != nil {
			t.Errorf("compare failed: %v", diff)
		}
	})
}

func TestGetUtxosForPublicKeyHash_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		data, ok := testData.TransactionsForPublicKeyHash[testConfig.network]
		if !ok {
			t.Fatalf(
				"transactions for public key hash data not defined for network %s",
				testConfig.network,
			)
		}

		publicKeyHash := (*[20]byte)(data.PublicKeyHash)
		expectedUtxos := data.Utxos

		utxos, err := electrum.GetUtxosForPublicKeyHash(*publicKeyHash)
		if err != nil {
			t.Fatal(err)
		}

		actualUtxos := make([]string, len(utxos))
		for i, utxo := range utxos {
			actualUtxos[i] = fmt.Sprintf("%v:%v:%v",
				utxo.Outpoint.TransactionHash.Hex(bitcoin.ReversedByteOrder),
				utxo.Outpoint.OutputIndex,
				utxo.Value,
			)
		}

		// Some UTXOs in the test data come from the same block and their
		// position is sometimes switched. Let's use another sort criteria
		// to achieve a predictable order, i.e. sort the whole UTXO string
		// (txHash:outputIndex:value) in the ascending order.
		sort.SliceStable(
			actualUtxos,
			func(i, j int) bool {
				return actualUtxos[i] < actualUtxos[j]
			},
		)

		if diff := deep.Equal(actualUtxos, expectedUtxos); diff != nil {
			t.Errorf("compare failed: %v", diff)
		}
	})
}

func TestEstimateSatPerVByteFee_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		satPerVByteFee, err := electrum.EstimateSatPerVByteFee(1)
		if err != nil {
			t.Fatal(err)
		}

		// We expect the fee is always at least 1.
		if satPerVByteFee < 1 {
			t.Errorf("returned fee is below 1")
		}
	})
}

func TestGetCoinbaseTxHash_Integration(t *testing.T) {
	runParallel(t, func(t *testing.T, testConfig testConfig) {
		electrum, cancelCtx := newTestConnection(t, testConfig.clientConfig)
		defer cancelCtx()

		blockData, ok := testData.Blocks[testConfig.network]
		if !ok {
			t.Fatalf("block test data not defined for network %s", testConfig.network)
		}

		txHash, err := electrum.GetCoinbaseTxHash(blockData.BlockHeight)
		if err != nil {
			t.Fatal(err)
		}

		expectedTxHash := blockData.CoinbaseTxHash
		if expectedTxHash != txHash {
			t.Errorf(
				"unexpected coinbase transaction hash\n"+
					"expected: %s\n"+
					"actual:   %s",
				expectedTxHash,
				txHash,
			)
		}
	})
}

func runParallel(t *testing.T, runFunc func(t *testing.T, testConfig testConfig)) {
	for testName, testConfig := range testConfigs {
		// Capture range variables.
		testName := testName
		testConfig := testConfig

		t.Run(testName, func(t *testing.T) {
			skipTestnetWSS(t, testName, testConfig)

			t.Parallel()

			runFunc(t, testConfig)
		})
	}
}

func skipTestnetWSS(t *testing.T, testName string, testConfig testConfig) {
	if !strings.Contains(testConfig.clientConfig.URL, "electrumx-server.test.tbtc.network") &&
		!strings.Contains(testName, "electrumx wss") &&
		!strings.Contains(testName, "electrumx-server.test.tbtc.network") {
		return
	}

	// TODO(3843): The test WSS endpoint is currently offline/handshake-failing
	// (sslv3 alert handshake failure), so skip until a healthy host is restored
	// or replaced.
	t.Skip("skip electrumx wss tests: test endpoint TLS handshake fails")
}

func newTestConnection(t *testing.T, config electrum.Config) (bitcoin.Chain, context.CancelFunc) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	electrum, err := electrum.Connect(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	return electrum, cancelCtx
}

func assertNumberCloseTo(t *testing.T, expected uint, actual uint, delta uint) {
	min := expected - delta
	max := expected + delta

	if min > actual || actual > max {
		t.Errorf(
			"value %d is out of expected range: [%d,%d]",
			actual,
			min,
			max,
		)
	}
}

type expectedErrorMessages struct {
	missingBlockHeader        []string
	missingTransactionInBlock []string
}

var expectedServerErrorMessages = expectedErrorMessages{
	missingBlockHeader: []string{
		"errNo: 0, errMsg: missing header",
		"errNo: 1, errMsg: height 4,294,967,295 out of range",
		"errNo: 1, errMsg: Invalid height",
	},
	missingTransactionInBlock: []string{
		"errNo: 0, errMsg: tx not found or is unconfirmed",
		"errNo: 1, errMsg: tx 9489457dc2c5a461a0b86394741ef57731605f2c628102de9f4d90afee9ac794 not in block at height 123,456",
		"errNo: 1, errMsg: No transaction matching the requested hash found at height 123456"},
}

func assertMissingBlockHeaderError(
	t *testing.T,
	clientConfig electrum.Config,
	clientErrorPrefix string,
	actualError error,
) {
	assertServerError(
		t,
		clientConfig,
		clientErrorPrefix,
		expectedServerErrorMessages.missingBlockHeader,
		actualError,
	)
}

func assertMissingTransactionInBlockError(
	t *testing.T,
	clientConfig electrum.Config,
	clientErrorPrefix string,
	actualError error,
) {
	assertServerError(
		t,
		clientConfig,
		clientErrorPrefix,
		expectedServerErrorMessages.missingTransactionInBlock,
		actualError,
	)
}

func assertServerError(
	t *testing.T,
	clientConfig electrum.Config,
	clientErrorPrefix string,
	expectedServerErrors []string,
	actualError error,
) {
	expectedErrorMsgFormat := fmt.Sprintf(
		"%s: [retry timeout [%s] exceeded; most recent error: [request failed: [%%s]]]",
		clientErrorPrefix,
		clientConfig.RequestRetryTimeout,
	)

	expectedErrorMsgStrings := make([]string, len(expectedServerErrors))
	for i, serverError := range expectedServerErrors {
		expectedErrorMsgStrings[i] = fmt.Sprintf(expectedErrorMsgFormat, serverError)
	}

	if actualError == nil {
		t.Errorf("expected error, but actual error is nil")
		return
	}

	if !slices.Contains(expectedErrorMsgStrings, actualError.Error()) {
		t.Errorf(
			"unexpected error message\nactual:\n\t%v\nexpected one of:\n\t%s",
			actualError,
			strings.Join(expectedErrorMsgStrings, "\n\t"),
		)
		return
	}
}

func toJson(val interface{}) string {
	b, err := json.Marshal(val)
	if err != nil {
		panic(err)
	}

	return string(b)
}
