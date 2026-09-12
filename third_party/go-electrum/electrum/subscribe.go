package electrum

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

// SubscribeHeadersResp represent the response to SubscribeHeaders().
type SubscribeHeadersResp struct {
	Result *SubscribeHeadersResult `json:"result"`
}

// SubscribeHeadersNotif represent the notification to SubscribeHeaders().
type SubscribeHeadersNotif struct {
	Params []*SubscribeHeadersResult `json:"params"`
}

// SubscribeHeadersResult represents the content of the result field in the response to SubscribeHeaders().
type SubscribeHeadersResult struct {
	Height int32  `json:"height,omitempty"`
	Hex    string `json:"hex"`
}

// SubscribeHeaders subscribes to receive block headers notifications when new blocks are found.
//
// BEWARE: This function can lead to a memory leak if the caller stops
// pulling from the returned channel. See the SubscribeHeadersSingle method
// for a safer alternative.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-headers-subscribe
func (s *Client) SubscribeHeaders(ctx context.Context) (<-chan *SubscribeHeadersResult, error) {
	var resp SubscribeHeadersResp

	err := s.request(ctx, "blockchain.headers.subscribe", []interface{}{}, &resp)
	if err != nil {
		return nil, err
	}

	respChan := make(chan *SubscribeHeadersResult, 1)
	respChan <- resp.Result

	go func() {
		for msg := range s.listenPush("blockchain.headers.subscribe") {
			if msg.err != nil {
				return
			}

			var resp SubscribeHeadersNotif

			err := json.Unmarshal(msg.content, &resp)
			if err != nil {
				return
			}

			for _, param := range resp.Params {
				respChan <- param
			}
		}
	}()

	return respChan, nil
}

// SubscribeHeadersSingle subscribes to receive the header of the current
// blockchain tip. Unlike SubscribeHeaders, this method only returns the tip
// and does not listen for new block headers.
//
// Worth noting that this action still creates a new subscription in the Electrum
// server. The protocol does neither support a single-shot request for the
// current blockchain tip nor subscription cancellation. Although this limitation
// causes a slight resource overhead on the client, it does not cause a memory
// leak like the SubscribeHeaders method which spawns a goroutine that may hang
// on the channel if the caller is no longer pulling from it.
//
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-headers-subscribe
func (s *Client) SubscribeHeadersSingle(ctx context.Context) (
	*SubscribeHeadersResult,
	error,
) {
	var resp SubscribeHeadersResp

	err := s.request(
		ctx,
		"blockchain.headers.subscribe",
		[]interface{}{},
		&resp,
	)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// ScripthashSubscription ...
type ScripthashSubscription struct {
	server    *Client
	notifChan chan *SubscribeNotif

	subscribedSH  []string
	scripthashMap map[string]string

	lock sync.RWMutex
}

// SubscribeNotif represent the notification to SubscribeScripthash() and SubscribeMasternode().
type SubscribeNotif struct {
	Params [2]string `json:"params"`
}

// SubscribeScripthash ...
func (s *Client) SubscribeScripthash() (*ScripthashSubscription, <-chan *SubscribeNotif) {
	sub := &ScripthashSubscription{
		server:        s,
		notifChan:     make(chan *SubscribeNotif, 1),
		scripthashMap: make(map[string]string),
	}

	go func() {
		for msg := range s.listenPush("blockchain.scripthash.subscribe") {
			if msg.err != nil {
				return
			}

			var resp SubscribeNotif

			err := json.Unmarshal(msg.content, &resp)
			if err != nil {
				return
			}

			sub.lock.Lock()
			for _, a := range sub.subscribedSH {
				if a == resp.Params[0] {
					sub.notifChan <- &resp
					break
				}
			}
			sub.lock.Unlock()
		}
	}()

	return sub, sub.notifChan
}

// Add ...
func (sub *ScripthashSubscription) Add(ctx context.Context, scripthash string, address ...string) error {
	var resp basicResp

	err := sub.server.request(ctx, "blockchain.scripthash.subscribe", []interface{}{scripthash}, &resp)
	if err != nil {
		return err
	}

	if len(resp.Result) > 0 {
		sub.notifChan <- &SubscribeNotif{[2]string{scripthash, resp.Result}}
	}

	sub.lock.Lock()
	sub.subscribedSH = append(sub.subscribedSH[:], scripthash)
	if len(address) > 0 {
		sub.scripthashMap[scripthash] = address[0]
	}
	sub.lock.Unlock()

	return nil
}

// GetAddress ...
func (sub *ScripthashSubscription) GetAddress(scripthash string) (string, error) {
	address, ok := sub.scripthashMap[scripthash]
	if ok {
		return address, nil
	}

	return "", errors.New("scripthash not found in map")
}

// GetScripthash ...
func (sub *ScripthashSubscription) GetScripthash(address string) (string, error) {
	var found bool
	var scripthash string

	for k, v := range sub.scripthashMap {
		if v == address {
			scripthash = k
			found = true
		}
	}

	if found {
		return scripthash, nil
	}

	return "", errors.New("address not found in map")
}

// GetChannel ...
func (sub *ScripthashSubscription) GetChannel() <-chan *SubscribeNotif {
	return sub.notifChan
}

// Remove ...
func (sub *ScripthashSubscription) Remove(scripthash string) error {
	for i, v := range sub.subscribedSH {
		if v == scripthash {
			sub.lock.Lock()
			sub.subscribedSH = append(sub.subscribedSH[:i], sub.subscribedSH[i+1:]...)
			sub.lock.Unlock()
			return nil
		}
	}

	return errors.New("scripthash not found")
}

// RemoveAddress ...
func (sub *ScripthashSubscription) RemoveAddress(address string) error {
	scripthash, err := sub.GetScripthash(address)
	if err != nil {
		return err
	}

	for i, v := range sub.subscribedSH {
		if v == scripthash {
			sub.lock.Lock()
			sub.subscribedSH = append(sub.subscribedSH[:i], sub.subscribedSH[i+1:]...)
			delete(sub.scripthashMap, scripthash)
			sub.lock.Unlock()
			return nil
		}
	}

	return errors.New("scripthash not found")
}

// Resubscribe ...
func (sub *ScripthashSubscription) Resubscribe(ctx context.Context) error {
	for _, v := range sub.subscribedSH {
		err := sub.Add(ctx, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// SubscribeMasternode subscribes to receive notifications when a masternode status changes.
// https://electrumx.readthedocs.io/en/latest/protocol-methods.html#blockchain-headers-subscribe
func (s *Client) SubscribeMasternode(ctx context.Context, collateral string) (<-chan string, error) {
	var resp basicResp

	err := s.request(ctx, "blockchain.masternode.subscribe", []interface{}{collateral}, &resp)
	if err != nil {
		return nil, err
	}

	respChan := make(chan string, 1)
	if len(resp.Result) > 0 {
		respChan <- resp.Result
	}

	go func() {
		for msg := range s.listenPush("blockchain.masternode.subscribe") {
			if msg.err != nil {
				return
			}

			var resp SubscribeNotif

			err := json.Unmarshal(msg.content, &resp)
			if err != nil {
				return
			}

			for _, param := range resp.Params {
				respChan <- param
			}
		}
	}()

	return respChan, nil
}
