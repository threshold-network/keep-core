package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

const tbtcSignerAnchorBootstrapArtifactReadLimit = 16 * 1024 * 1024

type tbtcSignerAnchorBootstrapClientFactory func(
	context.Context,
	string,
) (tbtc.FrostNativeSignerAnchorBootstrapClient, error)

// TBTCSignerCommand contains fail-closed signer administration tools. The
// bootstrap subtree is safe in all builds: facts fails when the native ABI is
// unavailable, and initialize fails until the separately reviewed bootstrap
// transport supplies its narrow client factory.
var TBTCSignerCommand = newTBTCSignerCommand(nil)

func newTBTCSignerCommand(
	clientFactory tbtcSignerAnchorBootstrapClientFactory,
) *cobra.Command {
	signer := &cobra.Command{
		Use:          "tbtc-signer",
		Short:        "Administers the native tBTC signer",
		SilenceUsage: true,
	}
	anchor := &cobra.Command{
		Use:   "anchor",
		Short: "Administers the native signer state anchor",
	}
	bootstrap := &cobra.Command{
		Use:   "bootstrap",
		Short: "Runs the offline-authorized initial anchor ceremony",
		Long: "Runs the four-phase initial anchor ceremony. The online " +
			"commands accept detached signatures only and never accept or load " +
			"the offline authority private key.",
	}
	bootstrap.AddCommand(
		newTBTCSignerAnchorBootstrapFactsCommand(),
		newTBTCSignerAnchorBootstrapCoreCommand(),
		newTBTCSignerAnchorBootstrapInitializeCommand(clientFactory),
		newTBTCSignerAnchorBootstrapFinalizeCommand(),
	)
	anchor.AddCommand(bootstrap)
	signer.AddCommand(anchor)
	return signer
}

func newTBTCSignerAnchorBootstrapFactsCommand() *cobra.Command {
	var provisioningConfig string
	var output string
	command := &cobra.Command{
		Use:   "facts",
		Short: "Exports the pristine native store genesis",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err :=
				frostsigning.InstallNativeTBTCSignerStateAnchorBootstrapProvisioningConfigFile(
					provisioningConfig,
				); err != nil {
				return err
			}
			facts, err :=
				frostsigning.ReadNativeTBTCSignerStateAnchorBootstrapFacts()
			if err != nil {
				return err
			}
			encoded, err :=
				frostsigning.EncodeNativeTBTCSignerStateAnchorBootstrapFacts(
					facts,
				)
			if err != nil {
				return err
			}
			return tbtc.WriteFrostNativeSignerAnchorProvisioningArtifact(
				output,
				encoded,
			)
		},
	}
	command.Flags().StringVar(
		&provisioningConfig,
		"provisioning-config",
		"",
		"canonical absolute path to the exact owner-only provisioning init config",
	)
	command.Flags().StringVar(
		&output,
		"output",
		"",
		"canonical absolute no-replace output artifact path",
	)
	_ = command.MarkFlagRequired("provisioning-config")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTBTCSignerAnchorBootstrapCoreCommand() *cobra.Command {
	var factsPath string
	var planPath string
	var output string
	command := &cobra.Command{
		Use:   "core",
		Short: "Builds the first offline signing request",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			factsJSON, err := readTBTCSignerAnchorBootstrapArtifact(factsPath)
			if err != nil {
				return err
			}
			facts, err :=
				frostsigning.DecodeNativeTBTCSignerStateAnchorBootstrapFacts(
					factsJSON,
				)
			if err != nil {
				return err
			}
			planJSON, err := readTBTCSignerAnchorBootstrapArtifact(planPath)
			if err != nil {
				return err
			}
			plan, err := tbtc.DecodeFrostNativeSignerAnchorBootstrapPlan(
				planJSON,
			)
			if err != nil {
				return err
			}
			core, err := tbtc.PrepareFrostNativeSignerAnchorBootstrapCore(
				facts,
				plan,
			)
			if err != nil {
				return err
			}
			encoded, err :=
				tbtc.EncodeFrostNativeSignerAnchorBootstrapCoreArtifact(core)
			if err != nil {
				return err
			}
			return tbtc.WriteFrostNativeSignerAnchorProvisioningArtifact(
				output,
				encoded,
			)
		},
	}
	command.Flags().StringVar(
		&factsPath,
		"facts",
		"",
		"canonical absolute path to the bootstrap facts artifact",
	)
	command.Flags().StringVar(
		&planPath,
		"plan",
		"",
		"canonical absolute path to the authenticated public bootstrap plan",
	)
	command.Flags().StringVar(
		&output,
		"output",
		"",
		"canonical absolute no-replace core signing-request path",
	)
	_ = command.MarkFlagRequired("facts")
	_ = command.MarkFlagRequired("plan")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTBTCSignerAnchorBootstrapInitializeCommand(
	clientFactory tbtcSignerAnchorBootstrapClientFactory,
) *cobra.Command {
	var corePath string
	var signaturePath string
	var clientConfigPath string
	var output string
	command := &cobra.Command{
		Use:   "initialize",
		Short: "Initializes and reconciles the remote anchor stream",
		Long: "Submits the detached core authorization, then requires a fresh " +
			"authenticated Read of the exact created stream. This online phase " +
			"never accepts an offline authority private key.",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if clientFactory == nil {
				return fmt.Errorf(
					"native signer anchor bootstrap transport is not available in this build",
				)
			}
			if !filepath.IsAbs(clientConfigPath) ||
				filepath.Clean(clientConfigPath) != clientConfigPath {
				return fmt.Errorf(
					"bootstrap client config path is not canonical absolute",
				)
			}
			coreJSON, err := readTBTCSignerAnchorBootstrapArtifact(corePath)
			if err != nil {
				return err
			}
			core, err :=
				tbtc.DecodeFrostNativeSignerAnchorBootstrapCoreArtifact(
					coreJSON,
				)
			if err != nil {
				return err
			}
			signatureJSON, err :=
				readTBTCSignerAnchorBootstrapArtifact(signaturePath)
			if err != nil {
				return err
			}
			signature, err :=
				tbtc.DecodeFrostNativeSignerAnchorBootstrapDetachedSignature(
					signatureJSON,
				)
			if err != nil {
				return err
			}
			client, err := clientFactory(command.Context(), clientConfigPath)
			if err != nil {
				return err
			}
			final, err := tbtc.InitializeFrostNativeSignerAnchorBootstrap(
				command.Context(),
				core,
				signature,
				client,
			)
			if err != nil {
				return err
			}
			encoded, err :=
				tbtc.EncodeFrostNativeSignerAnchorBootstrapFinalArtifact(
					final,
				)
			if err != nil {
				return err
			}
			return tbtc.WriteFrostNativeSignerAnchorProvisioningArtifact(
				output,
				encoded,
			)
		},
	}
	command.Flags().StringVar(
		&corePath,
		"core",
		"",
		"canonical absolute path to the core signing request",
	)
	command.Flags().StringVar(
		&signaturePath,
		"core-signature",
		"",
		"canonical absolute path to the detached offline core signature",
	)
	command.Flags().StringVar(
		&clientConfigPath,
		"client-config",
		"",
		"canonical absolute owner-only online bootstrap client config path",
	)
	command.Flags().StringVar(
		&output,
		"output",
		"",
		"canonical absolute no-replace final signing-request path",
	)
	_ = command.MarkFlagRequired("core")
	_ = command.MarkFlagRequired("core-signature")
	_ = command.MarkFlagRequired("client-config")
	_ = command.MarkFlagRequired("output")
	return command
}

func newTBTCSignerAnchorBootstrapFinalizeCommand() *cobra.Command {
	var finalPath string
	var signaturePath string
	var baseConfigPath string
	var output string
	command := &cobra.Command{
		Use:   "finalize",
		Short: "Builds the certified bootstrap output bundle",
		Long: "Validates the detached final signature and atomically emits a " +
			"versioned bundle containing the canonical certificate chain and " +
			"normal-signer init config.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			finalJSON, err := readTBTCSignerAnchorBootstrapArtifact(finalPath)
			if err != nil {
				return err
			}
			final, err :=
				tbtc.DecodeFrostNativeSignerAnchorBootstrapFinalArtifact(
					finalJSON,
				)
			if err != nil {
				return err
			}
			signatureJSON, err :=
				readTBTCSignerAnchorBootstrapArtifact(signaturePath)
			if err != nil {
				return err
			}
			signature, err :=
				tbtc.DecodeFrostNativeSignerAnchorBootstrapDetachedSignature(
					signatureJSON,
				)
			if err != nil {
				return err
			}
			baseConfig, err :=
				readTBTCSignerAnchorBootstrapArtifact(baseConfigPath)
			if err != nil {
				return err
			}
			bundle, err := tbtc.FinalizeFrostNativeSignerAnchorBootstrap(
				final,
				signature,
				baseConfig,
			)
			if err != nil {
				return err
			}
			return tbtc.WriteFrostNativeSignerAnchorProvisioningArtifact(
				output,
				bundle,
			)
		},
	}
	command.Flags().StringVar(
		&finalPath,
		"final",
		"",
		"canonical absolute path to the final signing request",
	)
	command.Flags().StringVar(
		&signaturePath,
		"final-signature",
		"",
		"canonical absolute path to the detached offline final signature",
	)
	command.Flags().StringVar(
		&baseConfigPath,
		"base-config",
		"",
		"canonical absolute path to the owner-only normal-signer base config",
	)
	command.Flags().StringVar(
		&output,
		"output",
		"",
		"canonical absolute no-replace certified output-bundle path",
	)
	_ = command.MarkFlagRequired("final")
	_ = command.MarkFlagRequired("final-signature")
	_ = command.MarkFlagRequired("base-config")
	_ = command.MarkFlagRequired("output")
	return command
}

func readTBTCSignerAnchorBootstrapArtifact(path string) ([]byte, error) {
	return tbtc.ReadFrostNativeSignerAnchorProvisioningArtifact(
		path,
		tbtcSignerAnchorBootstrapArtifactReadLimit,
	)
}
