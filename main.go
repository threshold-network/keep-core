package main

import (
	"os"

	"fmt"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-core/build"
	"github.com/keep-network/keep-core/cmd"
	"github.com/keep-network/keep-core/config"
	"github.com/keep-network/keep-core/pkg/keepcommon/logging"
)

//go:generate make gen_proto

var logger = log.Logger("keep-main")

func main() {
	configureLogging()

	rootCmd := cmd.Initialize(build.Version, build.Revision)

	if err := rootCmd.Execute(); err != nil {
		logger.Fatal(err)
	}
}

func configureLogging() {
	logLevel := "info"
	if env := os.Getenv(config.LogLevelEnvVariable); len(env) > 0 {
		logLevel = env
	}

	pubsubLogLevel := "warn"
	if env := os.Getenv(config.PubsubLogLevelEnvVariable); len(env) > 0 {
		pubsubLogLevel = env
	}

	levelDirective := fmt.Sprintf(
		"%s pubsub=%s",
		logLevel,
		pubsubLogLevel,
	)

	err := logging.Configure(levelDirective)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure logging: [%v]\n", err)
	}
}
