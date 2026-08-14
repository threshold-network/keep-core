package sortition

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/sortition/internal/local"
)

const (
	testOperatorAddress        = "0x3c5eBAcFe5aE12D82d43602a12b8bBb76b893CfA"
	testStakingProviderAddress = "0x80C63B577DC79B2432357BECC5b431dfb8E181DD"

	statusCheckTick = 10 * time.Millisecond
)

// If environment variable `PRINT_LOGS_IN_TEST` is set to `true`, logger in
// the code called by unit tests prints to the console.
func TestMain(m *testing.M) {
	if os.Getenv("PRINT_LOGS_IN_TEST") == "true" {
		err := log.SetLogLevel("*", "DEBUG")
		if err != nil {
			fmt.Fprintf(os.Stderr, "logger initialization failed: [%v]\n", err)
			os.Exit(-1)
		}
	}

	os.Exit(m.Run())
}

func TestMonitorPool_NotRegisteredOperator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	testutils.AssertErrorsSame(t, ErrOperatorUnknown, err)
}

func TestMonitorPool_NoStake(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	isOperatorInPool, err := localChain.IsOperatorInPool()
	if err != nil {
		t.Fatal(err)
	}

	if isOperatorInPool {
		t.Fatal("expected the operator not to be in the pool")
	}
}

func TestMonitor_JoinPool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	isOperatorInPool, err := localChain.IsOperatorInPool()
	if err != nil {
		t.Fatal(err)
	}

	if !isOperatorInPool {
		t.Fatal("expected the operator to join the pool")
	}
}

func TestMonitor_JoinPool_PolicyNotSatisfied(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		&neverJoinPolicy{},
	)
	if err != nil {
		t.Fatal(err)
	}

	isOperatorInPool, err := localChain.IsOperatorInPool()
	if err != nil {
		t.Fatal(err)
	}

	if isOperatorInPool {
		t.Fatal("expected the operator to not join the pool")
	}
}

func TestMonitor_UpdatePool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))
	localChain.JoinSortitionPool()

	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(101))

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	isOperatorUpToDate, err := localChain.IsOperatorUpToDate()
	if err != nil {
		t.Fatal(err)
	}
	if !isOperatorUpToDate {
		t.Fatal("expected the operator to be up to date")
	}
}

func TestMonitor_JoinPool_WithDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	isOperatorInPool, err := localChain.IsOperatorInPool()
	if err != nil {
		t.Fatal(err)
	}
	if isOperatorInPool {
		t.Fatal("expected the operator not to be in the pool")
	}

	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))

	// Let's give some time for the monitoring loop to react...
	time.Sleep(50 * time.Millisecond)

	isOperatorInPool, err = localChain.IsOperatorInPool()
	if err != nil {
		t.Fatal(err)
	}
	if !isOperatorInPool {
		t.Fatal("expected the operator to join the pool")
	}
}

func TestMonitor_UpdatePool_WithDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))
	localChain.JoinSortitionPool()

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(101))
	isOperatorUpToDate, err := localChain.IsOperatorUpToDate()
	if err != nil {
		t.Fatal(err)
	}
	if isOperatorUpToDate {
		t.Fatal("expected the operator not to be up to date")
	}

	// Let's give some time for the monitoring loop to react...
	time.Sleep(50 * time.Millisecond)

	isOperatorUpToDate, err = localChain.IsOperatorUpToDate()
	if err != nil {
		t.Fatal(err)
	}
	if !isOperatorUpToDate {
		t.Fatal("expected the operator to be up to date")
	}
}

func TestMonitor_CannotRestoreRewardsEligibility_TimeNotPassed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))
	localChain.JoinSortitionPool()

	// Operator is ineligible for rewards and eligibility can
	// not be restored yet
	localChain.SetRewardIneligibility(big.NewInt(1))
	localChain.SetCurrentTimestamp(big.NewInt(0))

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	isEligibleForRewards, err := localChain.IsEligibleForRewards()
	if err != nil {
		t.Fatal(err)
	}
	if isEligibleForRewards {
		t.Fatal("expected the operator not to be eligible for rewards")
	}
}

func TestMonitor_CanRestoreRewardsEligibility(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))
	localChain.JoinSortitionPool()

	// Operator is ineligible for rewards and eligibility can
	// be restored at this point
	localChain.SetRewardIneligibility(big.NewInt(1))
	localChain.SetCurrentTimestamp(big.NewInt(2))

	err := MonitorPool(
		ctx, &testutils.MockLogger{}, localChain, statusCheckTick, UnconditionalJoinPolicy)
	if err != nil {
		t.Fatal(err)
	}

	isEligibleForRewards, err := localChain.IsEligibleForRewards()
	if err != nil {
		t.Fatal(err)
	}
	if !isEligibleForRewards {
		t.Fatal("expected the operator to be restored for rewards")
	}
}

func TestMonitor_CanRestoreRewardsEligibility_WithDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localChain := local.Connect(testOperatorAddress)
	localChain.RegisterOperator(testStakingProviderAddress, testOperatorAddress)
	localChain.SetEligibleStake(testStakingProviderAddress, big.NewInt(100))
	localChain.JoinSortitionPool()

	// Operator is ineligible for rewards and eligibility can
	// not be restored yet
	localChain.SetRewardIneligibility(big.NewInt(1))
	localChain.SetCurrentTimestamp(big.NewInt(0))

	err := MonitorPool(
		ctx,
		&testutils.MockLogger{},
		localChain,
		statusCheckTick,
		UnconditionalJoinPolicy,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Eligibility can now be restored
	localChain.SetCurrentTimestamp(big.NewInt(2))
	// Let's give some time for the monitoring loop to react...
	time.Sleep(50 * time.Millisecond)

	isEligibleForRewards, err := localChain.IsEligibleForRewards()
	if err != nil {
		t.Fatal(err)
	}
	if !isEligibleForRewards {
		t.Fatal("expected the operator to be restored for rewards")
	}
}

type neverJoinPolicy struct{}

func (njp *neverJoinPolicy) ShouldJoin() bool {
	return false
}
