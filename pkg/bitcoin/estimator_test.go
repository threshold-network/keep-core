package bitcoin

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

func TestTransactionSizeEstimator_VirtualSize(t *testing.T) {
	taprootScript, err := PayToTaproot([32]byte{0x01})
	if err != nil {
		t.Fatal(err)
	}
	witnessPublicKeyHashScript, err := PayToWitnessPublicKeyHash([20]byte{0x02})
	if err != nil {
		t.Fatal(err)
	}

	var tests = map[string]struct {
		estimator           *TransactionSizeEstimator
		expectedVirtualSize int
		expectedError       error
	}{
		// https://live.blockcypher.com/btc-testnet/tx/5b6d040eb06b3de1a819890d55d251112e55c31db4a3f5eb7cfacf519fad7adb
		"1 P2WPKH input and 1 P2WSH input (92-byte redeem script) and 1 P2WPKH output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyHashInputs(1, true).
				AddScriptHashInputs(1, 92, true).
				AddPublicKeyHashOutputs(1, true),
			expectedVirtualSize: 201,
		},
		// https://live.blockcypher.com/btc-testnet/tx/ef96519c30906e9a3aae45a1945777486c3bfc0426bf234704c688a60518425f
		"1 P2WPKH input and 3 P2WSH inputs (92-byte redeem script) and 1 P2WPKH output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyHashInputs(1, true).
				AddScriptHashInputs(3, 92, true).
				AddPublicKeyHashOutputs(1, true),
			expectedVirtualSize: 384,
		},
		// https://live.blockcypher.com/btc-testnet/tx/dc8e04cf284e1fd32d30d2874aba1227e82bb5678ee7c12dbb378442ce8dba9e
		"1 P2WPKH input and 5 P2WSH inputs (92-byte redeem script) and 1 P2WPKH output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyHashInputs(1, true).
				AddScriptHashInputs(5, 92, true).
				AddPublicKeyHashOutputs(1, true),
			expectedVirtualSize: 566,
		},
		// https://live.blockcypher.com/btc-testnet/tx/8751b14f847fe4b2b571b4b925f0faad09686b12a6da3fbadea9dcc9f2e299dc
		"1 P2WPKH input and 10 P2WSH inputs (92-byte redeem script) and 1 P2WPKH output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyHashInputs(1, true).
				AddScriptHashInputs(10, 92, true).
				AddPublicKeyHashOutputs(1, true),
			expectedVirtualSize: 1022,
		},
		// https://live.blockcypher.com/btc-testnet/tx/2a5d5f472e376dc28964e1b597b1ca5ee5ac042101b5199a3ca8dae2deec3538
		"3 P2WSH inputs (92-byte redeem script) and 2 P2SH inputs (92-byte redeem script) and 1 P2WPKH output": {
			estimator: NewTransactionSizeEstimator().
				AddScriptHashInputs(3, 92, true).
				AddScriptHashInputs(2, 92, false).
				AddPublicKeyHashOutputs(1, true),
			expectedVirtualSize: 800,
		},
		// https://live.blockcypher.com/btc-testnet/tx/2724545276df61f43f1e92c4b9f1dd3c9109595c022dbd9dc003efbad8ded38b
		"1 P2WPKH input and 5 outputs (1 P2PKH, 2 P2WPKH, 1 P2SH, 1 P2WSH)": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyHashInputs(1, true).
				AddPublicKeyHashOutputs(1, false).
				AddPublicKeyHashOutputs(2, true).
				AddScriptHashOutputs(1, false).
				AddScriptHashOutputs(1, true),
			expectedVirtualSize: 250,
		},
		"1 P2TR key-path input and 1 P2TR output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyScriptInput(taprootScript).
				AddOutputScript(taprootScript),
			expectedVirtualSize: 111,
		},
		"2 P2TR key-path inputs and 1 P2TR output": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyScriptInput(taprootScript).
				AddPublicKeyScriptInput(taprootScript).
				AddOutputScript(taprootScript),
			expectedVirtualSize: 169,
		},
		"1 P2TR input and mixed P2TR and P2WPKH outputs": {
			estimator: NewTransactionSizeEstimator().
				AddPublicKeyScriptInput(taprootScript).
				AddOutputScript(taprootScript).
				AddOutputScript(taprootScript).
				AddOutputScript(witnessPublicKeyHashScript).
				AddOutputScript(witnessPublicKeyHashScript),
			expectedVirtualSize: 216,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			virtualSize, err := test.estimator.VirtualSize()

			if !reflect.DeepEqual(err, test.expectedError) {
				t.Errorf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]\n",
					test.expectedError,
					err,
				)
			}

			testutils.AssertIntsEqual(
				t,
				"virtual size",
				test.expectedVirtualSize,
				int(virtualSize),
			)
		})
	}
}

func TestTransactionFeeEstimator_EstimateFee(t *testing.T) {
	chain := newLocalChain()

	chain.setSatPerVByteFee(50)

	estimator := NewTransactionFeeEstimator(chain)

	fee, err := estimator.EstimateFee(250)
	if err != nil {
		t.Fatal(err)
	}

	testutils.AssertIntsEqual(
		t,
		"estimated fee",
		12500,
		int(fee),
	)
}
