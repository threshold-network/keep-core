package cmd

import (
	"fmt"
	"strconv"
	"strings"

	commonEthereum "github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/build"
	chainEthereum "github.com/keep-network/keep-core/pkg/chain/ethereum"
)

func nodeHeader(addrStrings []string, operator string, port int, ethereumConfig commonEthereum.Config) {
	header := ` 

▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
  ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
  ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
  ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓▌        ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
  ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
  ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓

Trust math, not hardware.
	
`

	prefix := "| "
	suffix := " |"

	maxLineLength := len(strconv.Itoa(port))

	for _, addrString := range addrStrings {
		if addrLength := len(addrString); addrLength > maxLineLength {
			maxLineLength = addrLength
		}
	}

	maxLineLength += len(prefix) + len(suffix) + 6
	dashes := strings.Repeat("-", maxLineLength) + "\n"

	fmt.Print(
		header,
		dashes,
		buildLine(maxLineLength, prefix, suffix, "Keep Client Node"),
		buildLine(maxLineLength, prefix, suffix, ""),
		buildVersion(maxLineLength, prefix, suffix),
		buildLine(maxLineLength, prefix, suffix, ""),
		buildLine(maxLineLength, prefix, suffix, fmt.Sprintf("Operator: %s", operator)),
		buildLine(maxLineLength, prefix, suffix, ""),
		buildLine(maxLineLength, prefix, suffix, fmt.Sprintf("Port: %d", port)),
		buildMultiLine(maxLineLength, prefix, suffix, "IPs : ", addrStrings),
		buildLine(maxLineLength, prefix, suffix, ""),
		buildContractAddresses(maxLineLength, prefix, suffix, ethereumConfig),
		dashes,
		"\n",
	)
}

func buildLine(lineLength int, prefix, suffix string, internalContent string) string {
	contentLength := len(prefix) + len(suffix) + len(internalContent)
	padding := lineLength - contentLength

	return fmt.Sprint(
		prefix,
		internalContent,
		strings.Repeat(" ", padding),
		suffix,
		"\n",
	)
}

func buildMultiLine(lineLength int, prefix, suffix, startPrefix string, lines []string) string {
	combinedLines := buildLine(lineLength, prefix+startPrefix, suffix, lines[0])

	startPadding := strings.Repeat(" ", len(startPrefix))
	for _, line := range lines[1:] {
		combinedLines += buildLine(lineLength, prefix+startPadding, suffix, line)
	}

	return combinedLines
}

func buildVersion(lineLength int, prefix, suffix string) string {
	return buildLine(
		lineLength,
		prefix,
		suffix,
		fmt.Sprintf("Version: %s (%s)", build.Version, build.Revision))
}

func buildContractAddresses(lineLength int, prefix, suffix string, ethereumConfig commonEthereum.Config) string {
	firstLine := buildLine(lineLength, prefix, suffix, "Contracts: ")

	contractNames := []string{
		chainEthereum.RandomBeaconContractName,
		chainEthereum.BridgeContractName,
		chainEthereum.WalletRegistryContractName,
		chainEthereum.FrostWalletRegistryContractName,
		chainEthereum.FrostDkgValidatorContractName,
		chainEthereum.TokenStakingContractName,
		chainEthereum.WalletProposalValidatorContractName,
	}

	entries := []string{}
	for _, contractName := range contractNames {
		contractAddress, err := ethereumConfig.ContractAddress(contractName)
		if err != nil {
			logger.Fatal(err)
		}
		entries = append(entries, fmt.Sprintf("%-15s: %s", contractName, contractAddress))
	}
	return firstLine + buildMultiLine(lineLength, prefix, suffix, "", entries)
}
