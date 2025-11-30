package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/hashicorp/go-multierror"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/rate"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/ethereum/threshold/gen/contract"
	"github.com/keep-network/keep-core/pkg/maintainer"
	"github.com/keep-network/keep-core/pkg/operator"
)

// Definitions of contract names.
const (
	TokenStakingContractName = "TokenStaking"
)

var logger = log.Logger("keep-ethereum")

// baseChain represents a base, non-application-specific chain handle. It
// provides the implementation of generic features like balance monitor,
// block counter and similar.
type baseChain struct {
	key     *keystore.Key
	client  ethutil.EthereumClient
	chainID *big.Int

	blockCounter *ethereum.BlockCounter
	nonceManager *ethereum.NonceManager
	miningWaiter *ethutil.MiningWaiter

	// transactionMutex allows interested parties to forcibly serialize
	// transaction submission.
	//
	// When transactions are submitted, they require a valid nonce. The nonce is
	// equal to the count of transactions the account has submitted so far, and
	// for a transaction to be accepted it should be monotonically greater than
	// any previous submitted transaction. To do this, transaction submission
	// asks the Ethereum client it is connected to for the next pending nonce,
	// and uses that value for the transaction. Unfortunately, if multiple
	// transactions are submitted in short order, they may all get the same
	// nonce. Serializing submission ensures that each nonce is requested after
	// a previous transaction has been submitted.
	transactionMutex *sync.Mutex

	tokenStaking *contract.TokenStaking
}

// Connect creates Random Beacon and TBTC Ethereum chain handles.
func Connect(
	ctx context.Context,
	config ethereum.Config,
) (
	*BeaconChain,
	*TbtcChain,
	chain.BlockCounter,
	chain.Signing,
	*operator.PrivateKey,
	error,
) {
	client, err := ethclient.Dial(config.URL)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"error Connecting to Ethereum Server: %s [%v]",
			config.URL,
			err,
		)
	}

	baseChain, err := newBaseChain(ctx, config, client)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"could not create base chain handle: [%v]",
			err,
		)
	}

	beaconChain, err := newBeaconChain(config, baseChain)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"could not create beacon chain handle: [%v]",
			err,
		)
	}

	tbtcChain, err := newTbtcChain(config, baseChain)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"could not create TBTC chain handle: [%v]",
			err,
		)
	}

	operatorPrivateKey, _, err := baseChain.OperatorKeyPair()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"could not get operator key pair: [%v]",
			err,
		)
	}

	if err := validateContractsAddresses(config, beaconChain, tbtcChain); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"contracts addresses validation failed: [%w]", err,
		)
	}

	return beaconChain,
		tbtcChain,
		baseChain.blockCounter,
		baseChain.Signing(),
		operatorPrivateKey,
		nil
}

// ConnectBitcoinDifficulty creates Bitcoin difficulty chain handle.
func ConnectBitcoinDifficulty(
	ctx context.Context,
	ethereumConfig ethereum.Config,
	maintainerConfig maintainer.Config,
) (
	*BitcoinDifficultyChain,
	error,
) {
	client, err := ethclient.Dial(ethereumConfig.URL)
	if err != nil {
		return nil, fmt.Errorf(
			"error Connecting to Ethereum Server: %s [%v]",
			ethereumConfig.URL,
			err,
		)
	}

	baseChain, err := newBaseChain(ctx, ethereumConfig, client)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create base chain handle: [%v]",
			err,
		)
	}

	bitcoinDifficultyChain, err := NewBitcoinDifficultyChain(
		ethereumConfig,
		maintainerConfig,
		baseChain,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"could not create Bitcoin difficulty chain handle: [%v]",
			err,
		)
	}

	return bitcoinDifficultyChain, nil
}

func validateContractsAddresses(
	config ethereum.Config,
	beaconChain *BeaconChain,
	tbtcChain *TbtcChain,
) error {
	baseStakingAddress, err := config.ContractAddress(TokenStakingContractName)
	if err != nil {
		return fmt.Errorf(
			"failed to get %s address from config: [%w]",
			TokenStakingContractName,
			err,
		)
	}

	beaconStakingAddress, err := beaconChain.Staking()
	if err != nil {
		return fmt.Errorf("failed to get staking address for beacon: [%w]", err)
	}

	tbtcStakingAddress, err := tbtcChain.Staking()
	if err != nil {
		return fmt.Errorf("failed to get staking address for tbtc: [%w]", err)
	}

	var result *multierror.Error

	if beaconStakingAddress.String() != baseStakingAddress.String() {
		result = multierror.Append(result, fmt.Errorf(
			"staking address for beacon [%s] doesn't match the base token staking address: [%s]",
			beaconStakingAddress,
			baseStakingAddress,
		))
	}

	if tbtcStakingAddress.String() != baseStakingAddress.String() {
		result = multierror.Append(result, fmt.Errorf(
			"staking address for tbtc [%s] doesn't match the base token staking address: [%s]",
			tbtcStakingAddress,
			baseStakingAddress,
		))
	}

	return result.ErrorOrNil()
}

// newChain construct a new instance of the Ethereum chain handle.
func newBaseChain(
	ctx context.Context,
	config ethereum.Config,
	client *ethclient.Client,
) (*baseChain, error) {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve Ethereum chain id: [%v]",
			err,
		)
	}

	if config.Network != ethereum.Developer &&
		big.NewInt(config.Network.ChainID()).Cmp(chainID) != 0 {
		return nil, fmt.Errorf(
			"chain id returned from ethereum api [%s] "+
				"doesn't match the expected chain id [%d] for [%s] network; "+
				"please verify the configured ethereum.url",
			chainID.String(),
			config.Network.ChainID(),
			config.Network,
		)
	}

	key, err := decryptKey(config)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decrypt Ethereum key: [%v]",
			err,
		)
	}

	clientWithAddons := wrapClientAddons(config, client)

	blockCounter, err := ethutil.NewBlockCounter(clientWithAddons)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create Ethereum blockcounter: [%v]",
			err,
		)
	}

	nonceManager := ethutil.NewNonceManager(
		clientWithAddons,
		key.Address,
	)

	miningWaiter := ethutil.NewMiningWaiter(clientWithAddons, config)

	transactionMutex := &sync.Mutex{}

	// TODO: Consider adding the balance monitoring.

	tokenStakingAddress, err := config.ContractAddress(TokenStakingContractName)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve %s contract address: [%v]",
			TokenStakingContractName,
			err,
		)
	}

	tokenStaking, err :=
		contract.NewTokenStaking(
			tokenStakingAddress,
			chainID,
			key,
			client,
			nonceManager,
			miningWaiter,
			blockCounter,
			transactionMutex,
		)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to attach to TokenStaking contract: [%v]",
			err,
		)
	}

	return &baseChain{
		key:              key,
		client:           clientWithAddons,
		chainID:          chainID,
		blockCounter:     blockCounter,
		nonceManager:     nonceManager,
		miningWaiter:     miningWaiter,
		transactionMutex: transactionMutex,
		tokenStaking:     tokenStaking,
	}, nil
}

// OperatorKeyPair returns the key pair of the operator assigned to this
// chain handle.
func (bc *baseChain) OperatorKeyPair() (
	*operator.PrivateKey,
	*operator.PublicKey,
	error,
) {
	privateKey, publicKey, err := ChainPrivateKeyToOperatorKeyPair(
		bc.key.PrivateKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot convert chain private key to operator key pair: [%v]",
			err,
		)
	}

	return privateKey, publicKey, nil
}

// GetBlockNumberByTimestamp gets the block number for the given timestamp.
// In the best case, the block with the exact same timestamp is returned.
// If the aforementioned is not possible, it tries to return the closest
// possible block.
//
// WARNING: THIS FUNCTION MAY NOT BE PERFORMANT FOR BLOCKS EARLIER THAN 15537393
// (SEPTEMBER 15, 2022, AT 1:42:42 EST) BEFORE THE ETH2 MERGE. PRE-MERGE
// AVERAGE BLOCK TIME WAS HIGHER THAN THE VALUE ASSUMED WITHIN THIS FUNCTION
// SO MORE OVERSHOOTS WILL BE DONE DURING THE BLOCK PREDICTION. OVERSHOOTS
// MUST BE COMPENSATED BY ADDITIONAL CLIENT CALLS THAT TAKE TIME.
func (bc *baseChain) GetBlockNumberByTimestamp(
	timestamp uint64,
) (uint64, error) {
	block, err := bc.currentBlock()
	if err != nil {
		return 0, fmt.Errorf("cannot get current block: [%v]", err)
	}

	if block.Time() < timestamp {
		return 0, fmt.Errorf("requested timestamp is in the future")
	}

	// Corner case shortcut.
	if block.Time() == timestamp {
		return block.NumberU64(), nil
	}

	// The Ethereum average block time (https://etherscan.io/chart/blocktime)
	// is slightly over 12s most of the time. If we assume it to be 12s here,
	// the below for-loop will often overshoot and hit blocks that are before
	// the requested timestamp. That means we will often fall into the second
	// compensation for-loop that walks forward block by block thus more
	// client calls will be made and more time will be needed. Instead of that,
	// we are assuming a slightly greater average block time than the actual
	// one. In this case, the below for-loop will often end with blocks
	// that are slightly after the requested timestamp. To compensate this case,
	// we can just compare the obtained block with the previous one and chose
	// the better one.
	const averageBlockTime = 13

	for block.Time() > timestamp {
		// timeDiff is always >0 due to the for-loop condition.
		timeDiff := block.Time() - timestamp
		// blockDiff is an integer whose value can be:
		// - >=1 if timeDiff >= averageBlockTime
		// - ==0 if timeDiff < averageBlockTime
		// It cannot be <0 as timeDiff is always >0.
		blockDiff := timeDiff / averageBlockTime
		// blockDiff equal to 0 would cause an infinite loop as it would always
		// fetch the same block so, we must break.
		if blockDiff < 1 {
			break
		}

		block, err = bc.blockByNumber(block.NumberU64() - blockDiff)
		if err != nil {
			return 0, fmt.Errorf("cannot get block: [%v]", err)
		}
	}

	// Once we quit the above for-loop, the following cases are possible:
	// - Case 1: block.Time() < timestamp
	// - Case 2: block.Time() > timestamp (difference is < averageBlockTime)
	// - Case 3: block.Time() == timestamp
	//
	// First, try to reduce Case 1 by walking forward block by block until
	// we achieve Case 2 or 3.
	for block.Time() < timestamp {
		block, err = bc.blockByNumber(block.NumberU64() + 1)
		if err != nil {
			return 0, fmt.Errorf("cannot get block: [%v]", err)
		}
	}
	// At this point, only Case 2 or 3 are possible. If we have Case 2,
	// just get the previous block and compare which one lies closer to
	// the requested timestamp.
	if block.Time() > timestamp {
		previousBlock, err := bc.blockByNumber(block.NumberU64() - 1)
		if err != nil {
			return 0, fmt.Errorf("cannot get block: [%v]", err)
		}

		return closerBlock(timestamp, previousBlock, block).NumberU64(), nil
	}

	return block.NumberU64(), nil
}

// GetBlockHashByNumber gets the block hash for the given block number.
func (bc *baseChain) GetBlockHashByNumber(blockNumber uint64) (
	[32]byte,
	error,
) {
	header, err := bc.headerByNumber(blockNumber)
	if err != nil {
		return [32]byte{}, fmt.Errorf("cannot get block header: [%v]", err)
	}

	return header.Hash(), nil
}

// currentBlock fetches the current block.
func (bc *baseChain) currentBlock() (*types.Block, error) {
	currentBlockNumber, err := bc.blockCounter.CurrentBlock()
	if err != nil {
		return nil, err
	}

	currentBlock, err := bc.blockByNumber(currentBlockNumber)
	if err != nil {
		return nil, err
	}

	return currentBlock, nil
}

// blockByNumber returns the block for the given block number. Times out
// if the underlying client call takes more than 30 seconds.
func (bc *baseChain) blockByNumber(number uint64) (*types.Block, error) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCtx()

	// Fetch the header to avoid decoding full transactions (some providers
	// may return transaction types the client library does not support yet).
	header, err := bc.client.HeaderByNumber(ctx, big.NewInt(int64(number)))
	if err != nil {
		return nil, err
	}

	return types.NewBlockWithHeader(header), nil
}

// headerByNumber returns the header for the given block number. Times out
// if the underlying client call takes more than 30 seconds.
func (bc *baseChain) headerByNumber(number uint64) (*types.Header, error) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCtx()

	return bc.client.HeaderByNumber(ctx, big.NewInt(int64(number)))
}

// closerBlock check timestamps of blocks b1 and b2 and returns the block
// whose timestamp lies closer to the requested timestamp. If the distance
// is same for both blocks, the block with greater block number is returned.
func closerBlock(timestamp uint64, b1, b2 *types.Block) *types.Block {
	abs := func(x int64) int64 {
		if x < 0 {
			return -x
		}
		return x
	}

	b1Diff := abs(int64(b1.Time() - timestamp))
	b2Diff := abs(int64(b2.Time() - timestamp))

	// If the differences are same, return the block with greater number.
	if b1Diff == b2Diff {
		if b2.NumberU64() > b1.NumberU64() {
			return b2
		}
		return b1
	}

	// Return the block whose timestamp is closer to the requested timestamp.
	if b1Diff < b2Diff {
		return b1
	}
	return b2
}

func (bc *baseChain) AverageBlockTime() time.Duration {
	return 12 * time.Second
}

// wrapClientAddons wraps the client instance with add-ons like logging, rate
// limiting and so on.
func wrapClientAddons(
	config ethereum.Config,
	client ethutil.EthereumClient,
) ethutil.EthereumClient {
	loggingClient := ethutil.WrapCallLogging(logger, client)

	if config.RequestsPerSecondLimit > 0 || config.ConcurrencyLimit > 0 {
		logger.Infof(
			"enabled ethereum client request rate limiter; "+
				"rps limit [%v]; "+
				"concurrency limit [%v]",
			config.RequestsPerSecondLimit,
			config.ConcurrencyLimit,
		)

		return ethutil.WrapRateLimiting(
			loggingClient,
			&rate.LimiterConfig{
				RequestsPerSecondLimit: config.RequestsPerSecondLimit,
				ConcurrencyLimit:       config.ConcurrencyLimit,
			},
		)
	}

	return loggingClient
}

// decryptKey decrypts the chain key pointed by the config.
func decryptKey(config ethereum.Config) (*keystore.Key, error) {
	return ethutil.DecryptKeyFile(
		config.Account.KeyFile,
		config.Account.KeyFilePassword,
	)
}
