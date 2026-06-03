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
				"/ip4/34.95.131.254/tcp/3919/ipfs/16Uiu2HAm4ZKNenwo6osQ9uWGovuA4XQWgFbjM7LLgHkhDk17bVsH",
				"/ip4/13.209.70.114/tcp/3919/ipfs/16Uiu2HAmCnzWWJdyfH2yY1d6BqLY8yBijcc2Gm2T3t5Hv7zCf2mv",
				"/ip4/204.93.241.110/tcp/3919/ipfs/16Uiu2HAm5ouZoUsG9s8NHpYXecEHLv7pog3nLbEHFM2QAcuquEPV",
				"/ip4/51.81.154.237/tcp/3919/ipfs/16Uiu2HAm75UztoweN19V8tmXNBbFfxcUApT21Bv17u1vv215QsGZ",
				"/ip4/135.181.90.126/tcp/3919/ipfs/16Uiu2HAmSKJDirLDh6zyahVFHFf7QEYktQFztC6vAT7rJi7B3jNm",
				"/ip4/173.234.17.141/tcp/3919/ipfs/16Uiu2HAmL8L2LFTYuZHxjTNmGzpjhfftBpGJrFomVg1yU82rXzy4",
				"/ip4/18.218.95.143/tcp/3919/ipfs/16Uiu2HAkzY9Y8ETqYNKKcnJ63bH26X6AgbrSZxCByJzLVgnGvHt4",
				"/ip4/167.99.58.9/tcp/3919/ipfs/16Uiu2HAmSj77bdmdiYBXUbkjvFN9dgsb5LENejp2pi35eRvkr8Ec",
				"/ip4/84.32.71.25/tcp/3919/ipfs/16Uiu2HAkwSJL9y6qDmGTh5yAPRaAJt9wFSgbWJkpffBjQcf8Z7gC",
				"/ip4/143.198.18.229/tcp/3919/ipfs/16Uiu2HAmDP4Z6LCogRMictJ6deGs4DRo99A5JTz5u3CLMg7URxC6",
				"/ip4/64.227.165.209/tcp/3919/ipfs/16Uiu2HAmBDvbdZHR5NdgGC5kfd36EXMKSViaMGNpx1y36Bm1VmW3",
				"/ip4/91.236.199.161/tcp/3919/ipfs/16Uiu2HAmUN9pYMu9s59DqjvW8kXBczAVXsFyhaHo7Ws4bzaRcLCi",
				"/dns4/keep.liquidlambda.com/tcp/3919/ipfs/16Uiu2HAmLJJY4ZUieacHffLva9crjz7epaGgP716Mp41yY48pnhx",
				"/dns4/klimah.venndo.com/tcp/3919/ipfs/16Uiu2HAm8ZXVc6bm1YrfXpqPZhBhL29yJgshp42bT5UrDDmwDu3x",
				"/ip4/51.75.71.106/tcp/3919/ipfs/16Uiu2HAm9fdcsU6hV7DbYbhvaqT2faAYWwso5JHe8rVNEFVejWap",
				"/dns4/keep-validator-3.prod-eks-ap-northeast-2.staked.cloud/tcp/3919/ipfs/16Uiu2HAmMzfoAfwVxBGvQq2cBuxMYv6WX2gJzRtYKYFVDVJT4xHh",
				"/ip4/35.210.22.52/tcp/3901/ipfs/16Uiu2HAm3RRVxCccHsUWe9dgjJo6fDqFUX3agDaMyeP9PknRtr7j",
				"/ip4/13.58.238.128/tcp/3919/ipfs/16Uiu2HAmGTVWggjSUiu2gxsQPHxCjzTV9TwaZdzPbuHRXmdyww6h",
				"/ip4/35.208.181.200/tcp/6012/ipfs/16Uiu2HAm6kbiAX7H6acR8XUdxxbpQPZYLNetXFi89tjwyaiWSZ6K",
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
