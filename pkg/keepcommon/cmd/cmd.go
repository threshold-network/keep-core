// Package cmd contains useful utilities for commands that interact with the
// Ethereum blockchain.
package cmd

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-common/pkg/cmd/flag"
	"github.com/spf13/cobra"
)

const (
	blockFlag  string = "block"
	blockShort string = "b"
	// SubmitFlag allows for definition and lookup of a boolean `--submit`
	// command-line flag indicating that a given contract interaction should be
	// submitted as a paid, mutating interaction to the configured Ethereum chain.
	SubmitFlag  string = "submit"
	submitShort string = "s"
	valueFlag   string = "value"
	valueShort  string = "v"
)

var (
	// BlockFlagValue allows for reading the block flag included in ConstFlags,
	// which represents the block at which to execute a contract interaction.
	// The value, if that flag is passed on the command line, is stored in this
	// variable.
	BlockFlagValue flag.BigIntFlagValue
	// ValueFlagValue allows for reading the value flag included in
	// PayableFlags, which represents an amount of ETH to send with a contract
	// interaction. The value, if that flag is passed on the command line, is
	// stored in this variable.
	ValueFlagValue ethereum.Wei
)

// InitConstFlags initializes flags useful for constant contract interactions,
// meaning contract interactions that do not require transaction submission and
// are used for inspecting chain state. These flags include:
//   --block flag to check an interaction's result value at a specific block.
func InitConstFlags(cmd *cobra.Command) {
	flag.BigIntVarPFlag(
		cmd.Flags(),
		&BlockFlagValue,
		blockFlag,
		blockShort,
		nil,
		"Retrieve the result of calling this method on `BLOCK`.",
	)
}

// InitNonConstFlags initializes flags useful for non-constant contract interactions,
// meaning contract interactions that can be submitted as transactions and are
// used for modifying chain state. These flags include:
//   --submit flag to submit an interaction as a transaction,
// as well as all flags in ConstFlags.
func InitNonConstFlags(cmd *cobra.Command) {
	InitConstFlags(cmd)

	cmd.Flags().BoolP(
		SubmitFlag,
		submitShort,
		false,
		"Submit this call as a gas-spending network transaction.",
	)
}

// InitPayableFlags initializes flags useful for payable contract interactions,
// meaning contract interactions that can be submitted as transactions and are
// used for modifying chain state with a payload that includes ETH. These flags
// include:
//   --value flag to specify the ETH amount to send with the interaction,
// as well as all flags in NonConstFlags.
func InitPayableFlags(cmd *cobra.Command) {
	InitNonConstFlags(cmd)

	flag.WeiVarPFlag(
		cmd.Flags(),
		&ValueFlagValue,
		valueFlag,
		valueShort,
		*ethereum.WrapWei(big.NewInt(0)),
		"Send `VALUE` ether with this call.",
	)
}

// ComposableArgChecker is a type that allows multiple spf13/cobra Before functions
// to be chained. See AndThen for more.
type ComposableArgChecker func(*cobra.Command, []string) error

// AndThen on a ComposableArgChecker allows composing a ComposableArgChecker
// with another one, such that this ComposableArgChecker runs and, if it
// succeeds, the nextChecker runs.
//
// As an example, this allows for two BeforeFuncs to be composed as:
//
//   ComposableArgChecker(checkFlagAValue).AndThen(ComposableArgChecker(checkFlagBValue))
//
// The resulting ComposableArgChecker will run checkFlagAValue and, if it
// passes, checkFlagBValue.
func (cac ComposableArgChecker) AndThen(nextChecker ComposableArgChecker) ComposableArgChecker {
	return func(c *cobra.Command, args []string) error {
		cacErr := cac(c, args)
		if cacErr != nil {
			return cacErr
		}

		return nextChecker(c, args)
	}
}

var (
	valueArgChecker ComposableArgChecker = func(c *cobra.Command, args []string) error {
		if err := c.MarkFlagRequired(valueFlag); err != nil {
			return fmt.Errorf("failed to mark %s flag required: %w", valueFlag, err)
		}

		return nil
	}

	submittedArgChecker ComposableArgChecker = func(c *cobra.Command, args []string) error {
		c.MarkFlagsMutuallyExclusive(SubmitFlag, blockFlag)

		return nil
	}
)

var (
	// NonConstArgsChecker runs any validation of parameters needed for a
	// command that will submit a mutating transaction to an Ethereum node. In
	// particular, it will disallow const arguments like --block and
	// --transaction if --submit is specified.
	NonConstArgsChecker = submittedArgChecker
	// PayableArgsChecker runs any validation of parameters needed for a
	// command that will submit a payable transaction to an Ethereum node. In
	// particular, it will disallow const arguments like --block and
	// --transaction if --submit is specified, and will also ensure that the
	// call includes specifies a --value.
	PayableArgsChecker = NonConstArgsChecker.AndThen(valueArgChecker)
)

// ArgCountChecker provides a consistent error in case the number of arguments
// passed on the command-line is incorrect.
func ArgCountChecker(expectedArgCount int) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		if len(args) != expectedArgCount {
			return fmt.Errorf(
				"expected [%d] arguments but got [%d]",
				expectedArgCount,
				len(args),
			)
		}

		return nil
	}
}

// PrintOutput provides for custom command-line-friendly ways of printing
// addresses (as hex) and transaction and related hashes (also as hex). It falls
// back on a standard Println for other types.
func PrintOutput(output interface{}) {
	switch out := output.(type) {
	case common.Address:
		fmt.Println(out.Hex())
	case common.Hash:
		fmt.Println(out.Hex())
	default:
		fmt.Printf("%+v\n", out)
	}
}
