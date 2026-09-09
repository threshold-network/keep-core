package config

import (
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/config/network"
)

func TestResolvePeers(t *testing.T) {
	var tests = map[string]struct {
		network       network.Type
		expectedPeers []string
		expectedError error
	}{
		"mainnet network": {
			network: network.Mainnet,
			expectedPeers: []string{
				"/ip4/138.201.251.149/tcp/3919/ipfs/16Uiu2HAm3ZwkmwUe4ivC2hGmekjZSjHhdAsw692yTEqbG71rS2pL",
				"/dns4/threshold.nodus.systems/tcp/3919/ipfs/16Uiu2HAm4ZKNenwo6osQ9uWGovuA4XQWgFbjM7LLgHkhDk17bVsH",
				"/dns4/tbtc.delightlabs.sh/tcp/3919/ipfs/16Uiu2HAmCnzWWJdyfH2yY1d6BqLY8yBijcc2Gm2T3t5Hv7zCf2mv",
				"/dns4/tbtc.globalstake.io/tcp/3919/ipfs/16Uiu2HAm5ouZoUsG9s8NHpYXecEHLv7pog3nLbEHFM2QAcuquEPV",
				"/dns4/keep.ponkila.com/tcp/3919/ipfs/16Uiu2HAmSKJDirLDh6zyahVFHFf7QEYktQFztC6vAT7rJi7B3jNm",
				"/dns4/tbtc.liquify.com/tcp/3919/ipfs/16Uiu2HAmL8L2LFTYuZHxjTNmGzpjhfftBpGJrFomVg1yU82rXzy4",
				"/dns4/threshold-sentry.vol-069756b811be95253.prod.stake.capital/tcp/3919/ipfs/16Uiu2HAkzY9Y8ETqYNKKcnJ63bH26X6AgbrSZxCByJzLVgnGvHt4",
				"/ip4/167.99.58.9/tcp/3919/ipfs/16Uiu2HAmSj77bdmdiYBXUbkjvFN9dgsb5LENejp2pi35eRvkr8Ec",
				"/dns4/slatemoth.mooo.com/tcp/3919/ipfs/16Uiu2HAkwSJL9y6qDmGTh5yAPRaAJt9wFSgbWJkpffBjQcf8Z7gC",
				"/dns4/tbtc.wanabai.com/tcp/3919/ipfs/16Uiu2HAmDP4Z6LCogRMictJ6deGs4DRo99A5JTz5u3CLMg7URxC6",
				"/dns4/tbtcnode.com/tcp/3919/ipfs/16Uiu2HAmBDvbdZHR5NdgGC5kfd36EXMKSViaMGNpx1y36Bm1VmW3",
				"/dns4/node.wearethekeepers.net/tcp/3919/ipfs/16Uiu2HAmUN9pYMu9s59DqjvW8kXBczAVXsFyhaHo7Ws4bzaRcLCi",
				"/dns4/keep.liquidlambda.com/tcp/3919/ipfs/16Uiu2HAmLJJY4ZUieacHffLva9crjz7epaGgP716Mp41yY48pnhx",
				"/dns4/klimah.venndo.com/tcp/3919/ipfs/16Uiu2HAm8ZXVc6bm1YrfXpqPZhBhL29yJgshp42bT5UrDDmwDu3x",
				"/dns4/keep.alephnodes.com/tcp/3919/ipfs/16Uiu2HAm9fdcsU6hV7DbYbhvaqT2faAYWwso5JHe8rVNEFVejWap",
				"/dns4/keep-validator-3.prod-eks-ap-northeast-2.staked.cloud/tcp/3919/ipfs/16Uiu2HAmMzfoAfwVxBGvQq2cBuxMYv6WX2gJzRtYKYFVDVJT4xHh",
				"/dns4/tbtc01.threshold.p2p.org/tcp/3901/ipfs/16Uiu2HAm3RRVxCccHsUWe9dgjJo6fDqFUX3agDaMyeP9PknRtr7j",
				"/dns4/tbtc.nucypher.com/tcp/3919/ipfs/16Uiu2HAmGTVWggjSUiu2gxsQPHxCjzTV9TwaZdzPbuHRXmdyww6h",
				"/dns4/tbtc.boar.network/tcp/6012/ipfs/16Uiu2HAm6kbiAX7H6acR8XUdxxbpQPZYLNetXFi89tjwyaiWSZ6K",
			},
		},
		"testnet network": {
			network: network.Testnet,
			expectedPeers: []string{
				"/ip4/143.198.69.177/tcp/3919/ipfs/16Uiu2HAkvjus5MH3y2tJBC6Bt1Ff9tiSowxGCw8J4FzLonnfDeG2",
				"/ip4/143.198.69.177/tcp/3920/ipfs/16Uiu2HAmSBn6CgZ4r7HnC4RVMMFMe5vfkLvykUUfS3MnKiHLSuPD",
				"/ip4/143.198.69.177/tcp/3921/ipfs/16Uiu2HAm5w7qg2BEBWjiYqUSw9Prf1Jdmk35Rm9sfz35g9EWZR8f",
			},
		},
		"developer network": {
			network: network.Developer,
		},
		"unknown network": {
			network: network.Unknown,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cfg := &Config{}

			err := cfg.resolvePeers(test.network)
			if !reflect.DeepEqual(test.expectedError, err) {
				t.Errorf(
					"unexpected error\nexpected: %+v\nactual:   %+v\n",
					test.expectedError,
					err,
				)
			}

			if len(test.expectedPeers) > 0 {
				if !reflect.DeepEqual(test.expectedPeers, cfg.LibP2P.Peers) {
					t.Errorf(
						"unexpected peers\nexpected: %+v\nactual:   %+v\n",
						test.expectedPeers,
						cfg.LibP2P.Peers,
					)
				}
			} else if len(cfg.LibP2P.Peers) != 0 {
				t.Errorf(
					"unexpected peers\nexpected: []\nactual:   %+v\n",
					cfg.LibP2P.Peers,
				)
			}
		})
	}
}
