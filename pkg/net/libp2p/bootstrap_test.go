package libp2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	config "github.com/ipfs/go-ipfs-config"
	log2 "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/test"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
)

func TestMultipleAddrsPerPeer(t *testing.T) {
	var bsps []peer.AddrInfo
	for i := 0; i < 10; i++ {
		pid, err := test.RandPeerID()
		if err != nil {
			t.Fatal(err)
		}

		addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/5001/ipfs/%s", pid.String())
		bsp1, err := config.ParseBootstrapPeers([]string{addr})
		if err != nil {
			t.Fatal(err)
		}

		addr = fmt.Sprintf("/ip4/127.0.0.1/udp/5002/utp/ipfs/%s", pid.String())
		bsp2, err := config.ParseBootstrapPeers([]string{addr})
		if err != nil {
			t.Fatal(err)
		}

		bsp1Addr, err := peer.AddrInfoFromP2pAddr(bsp1[0].Multiaddr())
		if err != nil {
			t.Fatal(err)
		}

		bsp2Addr, err := peer.AddrInfoFromP2pAddr(bsp2[0].Multiaddr())
		if err != nil {
			t.Fatal(err)
		}

		bsps = append(bsps, *bsp1Addr, *bsp2Addr)
	}

	pinfos := peers.toPeerInfos(bsps)
	if len(pinfos) != len(bsps)/2 {
		t.Fatal("expected fewer peers")
	}
}

// --- log capture helpers ---

type capturedLogEntry struct {
	Level   string `json:"level"`
	Logger  string `json:"logger"`
	Message string `json:"msg"`
}

// captureLogs runs fn while capturing log entries emitted by the given
// logger subsystem and returns the captured entries.
func captureLogs(t *testing.T, subsystem string, fn func()) []capturedLogEntry {
	t.Helper()

	if err := log2.SetLogLevel(subsystem, "debug"); err != nil {
		t.Fatal(err)
	}

	pipe := log2.NewPipeReader()

	var mutex sync.Mutex
	var entries []capturedLogEntry

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			var entry capturedLogEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				continue
			}
			if entry.Logger != subsystem {
				continue
			}
			mutex.Lock()
			entries = append(entries, entry)
			mutex.Unlock()
		}
	}()

	fn()

	if err := pipe.Close(); err != nil {
		t.Logf("could not close log pipe reader: %v", err)
	}
	<-done

	mutex.Lock()
	defer mutex.Unlock()
	return append([]capturedLogEntry(nil), entries...)
}

func countLogEntries(
	entries []capturedLogEntry,
	level string,
	messageFragment string,
) int {
	count := 0
	for _, entry := range entries {
		if entry.Level == level &&
			strings.Contains(entry.Message, messageFragment) {
			count++
		}
	}
	return count
}

// --- well-known peers connection round tests ---

// newWellknownPeersTestHost creates a mock-network-backed host acting as the
// local node in well-known peers connection round tests.
func newWellknownPeersTestHost(t *testing.T) (mocknet.Mocknet, host.Host) {
	t.Helper()

	mockNetwork := mocknet.New()
	t.Cleanup(func() {
		if err := mockNetwork.Close(); err != nil {
			t.Logf("could not close mock network: %v", err)
		}
	})

	localHost, err := mockNetwork.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	return mockNetwork, localHost
}

// newConnectedWellknownPeer creates a well-known peer the local host is
// connected to and returns its address info.
func newConnectedWellknownPeer(
	t *testing.T,
	mockNetwork mocknet.Mocknet,
	localHost host.Host,
) peer.AddrInfo {
	t.Helper()

	remoteHost, err := mockNetwork.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mockNetwork.LinkPeers(localHost.ID(), remoteHost.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := mockNetwork.ConnectPeers(localHost.ID(), remoteHost.ID()); err != nil {
		t.Fatal(err)
	}

	return peer.AddrInfo{ID: remoteHost.ID(), Addrs: remoteHost.Addrs()}
}

// newDialableWellknownPeer creates a well-known peer the local host is not
// connected to but can successfully dial, and returns its address info.
func newDialableWellknownPeer(
	t *testing.T,
	mockNetwork mocknet.Mocknet,
	localHost host.Host,
) peer.AddrInfo {
	t.Helper()

	remoteHost, err := mockNetwork.GenPeer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mockNetwork.LinkPeers(localHost.ID(), remoteHost.ID()); err != nil {
		t.Fatal(err)
	}

	return peer.AddrInfo{ID: remoteHost.ID(), Addrs: remoteHost.Addrs()}
}

// newUnreachableWellknownPeer returns address info of a well-known peer that
// does not exist on the mock network, so every dial to it fails.
func newUnreachableWellknownPeer(t *testing.T) peer.AddrInfo {
	t.Helper()

	peerID, err := test.RandPeerID()
	if err != nil {
		t.Fatal(err)
	}

	address, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1")
	if err != nil {
		t.Fatal(err)
	}

	return peer.AddrInfo{ID: peerID, Addrs: []ma.Multiaddr{address}}
}

func TestBootstrapRound_ZeroWellknownPeersConnected_ReturnsErrorAndWarns(t *testing.T) {
	_, localHost := newWellknownPeersTestHost(t)

	wellknownPeers := []peer.AddrInfo{
		newUnreachableWellknownPeer(t),
		newUnreachableWellknownPeer(t),
	}

	var roundErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		roundErr = bootstrapRound(
			context.Background(),
			localHost,
			bootstrapConfigWithPeers(wellknownPeers),
		)
	})

	if roundErr == nil {
		t.Fatal(
			"expected an error when the node is not connected to any " +
				"well-known peer and all dials failed",
		)
	}

	expectedMessage := "2 of 2 well-known peers unreachable; " +
		"node is not connected to any well-known peer"
	if !strings.Contains(roundErr.Error(), expectedMessage) {
		t.Errorf(
			"unexpected round error\nexpected to contain: %s\nactual: %v",
			expectedMessage,
			roundErr,
		)
	}

	// With zero well-known peers connected, the summary is logged at
	// warning level.
	if count := countLogEntries(entries, "warn", expectedMessage); count != 1 {
		t.Errorf(
			"expected 1 warning entry with message [%s], got %d",
			expectedMessage,
			count,
		)
	}

	// With zero well-known peers connected, per-peer dial failures are
	// logged at warning level.
	warns := countLogEntries(
		entries,
		"warn",
		"could not establish connection with well-known peer",
	)
	if warns != 2 {
		t.Errorf("expected 2 per-peer dial failure warnings, got %d", warns)
	}
}

func TestBootstrapRound_ZeroConnectedAtStart_PartialDialSuccess_LogsInfoNotWarn(t *testing.T) {
	mockNetwork, localHost := newWellknownPeersTestHost(t)

	// The node starts the round with zero well-known peers connected: one
	// peer is dialable and one is unreachable, so the round ends with the
	// node connected to one well-known peer.
	wellknownPeers := []peer.AddrInfo{
		newDialableWellknownPeer(t, mockNetwork, localHost),
		newUnreachableWellknownPeer(t),
	}

	var roundErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		roundErr = bootstrapRound(
			context.Background(),
			localHost,
			bootstrapConfigWithPeers(wellknownPeers),
		)
	})

	if roundErr != nil {
		t.Fatalf(
			"round should not error when a dial succeeded and the node "+
				"ends the round connected to a well-known peer: [%v]",
			roundErr,
		)
	}

	for _, level := range []string{"warn", "error"} {
		if count := countLogEntries(entries, level, ""); count != 0 {
			t.Errorf(
				"expected no %s entries when the round ends with a "+
					"well-known peer connected, got %d",
				level,
				count,
			)
		}
	}

	expectedSummary := "1 of 2 well-known peers unreachable; " +
		"connected to 1 well-known peer(s)"
	if count := countLogEntries(entries, "info", expectedSummary); count != 1 {
		t.Errorf(
			"expected 1 info entry with message [%s], got %d",
			expectedSummary,
			count,
		)
	}

	// The dial failure is logged at debug level because the node ends the
	// round connected, even though it started the round with zero
	// well-known peers connected.
	debugs := countLogEntries(
		entries,
		"debug",
		"could not establish connection with well-known peer",
	)
	if debugs != 1 {
		t.Errorf("expected 1 per-peer dial failure debug entry, got %d", debugs)
	}
}

func TestBootstrapRound_SomeWellknownPeersConnected_LogsInfoNotWarn(t *testing.T) {
	mockNetwork, localHost := newWellknownPeersTestHost(t)

	wellknownPeers := []peer.AddrInfo{
		newConnectedWellknownPeer(t, mockNetwork, localHost),
		newUnreachableWellknownPeer(t),
	}

	var roundErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		roundErr = bootstrapRound(
			context.Background(),
			localHost,
			bootstrapConfigWithPeers(wellknownPeers),
		)
	})

	if roundErr != nil {
		t.Fatalf(
			"round should not error when at least one well-known peer "+
				"is connected: [%v]",
			roundErr,
		)
	}

	for _, level := range []string{"warn", "error"} {
		if count := countLogEntries(entries, level, ""); count != 0 {
			t.Errorf(
				"expected no %s entries on a partially-connected node, "+
					"got %d",
				level,
				count,
			)
		}
	}

	expectedSummary := "1 of 2 well-known peers unreachable; " +
		"connected to 1 well-known peer(s)"
	if count := countLogEntries(entries, "info", expectedSummary); count != 1 {
		t.Errorf(
			"expected 1 info entry with message [%s], got %d",
			expectedSummary,
			count,
		)
	}

	// Per-peer dial failures are logged at debug level on a
	// partially-connected node.
	debugs := countLogEntries(
		entries,
		"debug",
		"could not establish connection with well-known peer",
	)
	if debugs != 1 {
		t.Errorf("expected 1 per-peer dial failure debug entry, got %d", debugs)
	}
}

func TestBootstrapRound_AllWellknownPeersConnected_Silent(t *testing.T) {
	mockNetwork, localHost := newWellknownPeersTestHost(t)

	wellknownPeers := []peer.AddrInfo{
		newConnectedWellknownPeer(t, mockNetwork, localHost),
		newConnectedWellknownPeer(t, mockNetwork, localHost),
	}

	var roundErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		roundErr = bootstrapRound(
			context.Background(),
			localHost,
			bootstrapConfigWithPeers(wellknownPeers),
		)
	})

	if roundErr != nil {
		t.Fatalf("round should not error on a fully-connected node: [%v]", roundErr)
	}

	for _, level := range []string{"warn", "error"} {
		if count := countLogEntries(entries, level, ""); count != 0 {
			t.Errorf(
				"expected no %s entries on a fully-connected node, got %d",
				level,
				count,
			)
		}
	}

	if count := countLogEntries(entries, "info", "unreachable"); count != 0 {
		t.Errorf(
			"expected no unreachability info entries on a fully-connected "+
				"node, got %d",
			count,
		)
	}
}

// dropWellknownPeer disconnects the local host from a previously-connected
// well-known peer and verifies the disconnection took effect.
func dropWellknownPeer(
	t *testing.T,
	mockNetwork mocknet.Mocknet,
	localHost host.Host,
	peerInfo peer.AddrInfo,
) {
	t.Helper()

	if err := mockNetwork.DisconnectPeers(localHost.ID(), peerInfo.ID); err != nil {
		t.Fatal(err)
	}
	if localHost.Network().Connectedness(peerInfo.ID) == network.Connected {
		t.Fatal("well-known peer expected to be disconnected")
	}
}

func TestBootstrapConnect_ConnectedPeerDropsDuringRound_WarnsWithLiveRatio(t *testing.T) {
	mockNetwork, localHost := newWellknownPeersTestHost(t)

	droppedPeer := newConnectedWellknownPeer(t, mockNetwork, localHost)
	unreachablePeer := newUnreachableWellknownPeer(t)
	wellknownPeers := []peer.AddrInfo{droppedPeer, unreachablePeer}

	// The dial set passed below was computed while droppedPeer was still
	// connected, so it contains only unreachablePeer. Dropping droppedPeer
	// before the dial batch runs recreates a peer disconnecting mid-round,
	// after the round-start connectivity snapshot but before the final
	// severity decision.
	dropWellknownPeer(t, mockNetwork, localHost, droppedPeer)

	var connectErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		connectErr = bootstrapConnect(
			context.Background(),
			localHost,
			[]peer.AddrInfo{unreachablePeer},
			wellknownPeers,
		)
	})

	if connectErr == nil {
		t.Fatal(
			"expected an error when the node ends the round not connected " +
				"to any well-known peer",
		)
	}

	// The node ends the round connected to neither well-known peer, so the
	// summary must report the live 2-of-2 ratio, not the single dial failure.
	expectedMessage := "2 of 2 well-known peers unreachable; " +
		"node is not connected to any well-known peer"
	if !strings.Contains(connectErr.Error(), expectedMessage) {
		t.Errorf(
			"unexpected error\nexpected to contain: %s\nactual: %v",
			expectedMessage,
			connectErr,
		)
	}

	if count := countLogEntries(entries, "warn", expectedMessage); count != 1 {
		t.Errorf(
			"expected 1 warning entry with message [%s], got %d",
			expectedMessage,
			count,
		)
	}

	// Only the dialed peer produces a per-peer dial failure log; the dropped
	// peer is reflected in the summary ratio alone.
	warns := countLogEntries(
		entries,
		"warn",
		"could not establish connection with well-known peer",
	)
	if warns != 1 {
		t.Errorf("expected 1 per-peer dial failure warning, got %d", warns)
	}
}

func TestBootstrapConnect_AllDialsSucceedButConnectedPeerDrops_LogsInfoRatio(t *testing.T) {
	mockNetwork, localHost := newWellknownPeersTestHost(t)

	droppedPeer := newConnectedWellknownPeer(t, mockNetwork, localHost)
	dialablePeer := newDialableWellknownPeer(t, mockNetwork, localHost)
	wellknownPeers := []peer.AddrInfo{droppedPeer, dialablePeer}

	// droppedPeer disconnects after the dial set (only dialablePeer) was
	// computed. Every dial in the batch then succeeds, yet the node still
	// ends the round with one well-known peer unreachable -- the round must
	// detect that from live connectivity, not from dial results.
	dropWellknownPeer(t, mockNetwork, localHost, droppedPeer)

	var connectErr error
	entries := captureLogs(t, "keep-libp2p", func() {
		connectErr = bootstrapConnect(
			context.Background(),
			localHost,
			[]peer.AddrInfo{dialablePeer},
			wellknownPeers,
		)
	})

	if connectErr != nil {
		t.Fatalf(
			"round should not error when the node ends the round connected "+
				"to a well-known peer: [%v]",
			connectErr,
		)
	}

	for _, level := range []string{"warn", "error"} {
		if count := countLogEntries(entries, level, ""); count != 0 {
			t.Errorf(
				"expected no %s entries when the round ends with a "+
					"well-known peer connected, got %d",
				level,
				count,
			)
		}
	}

	expectedSummary := "1 of 2 well-known peers unreachable; " +
		"connected to 1 well-known peer(s)"
	if count := countLogEntries(entries, "info", expectedSummary); count != 1 {
		t.Errorf(
			"expected 1 info entry with message [%s], got %d",
			expectedSummary,
			count,
		)
	}
}

func TestBootstrap_IsolatedRound_EmitsExactlyOneSummaryWarn(t *testing.T) {
	_, localHost := newWellknownPeersTestHost(t)

	wellknownPeers := []peer.AddrInfo{
		newUnreachableWellknownPeer(t),
		newUnreachableWellknownPeer(t),
	}

	cfg := bootstrapConfigWithPeers(wellknownPeers)
	// A period far longer than the test ensures only the initial round runs
	// before the supervisor is closed.
	cfg.Period = time.Hour

	entries := captureLogs(t, "keep-libp2p", func() {
		// Bootstrap returns only after the initial round has completed, so
		// all of the round's log entries are captured before the pipe closes.
		supervisor, err := Bootstrap(localHost.ID(), localHost, nil, cfg)
		if err != nil {
			t.Errorf("unexpected error starting the supervisor: [%v]", err)
			return
		}
		if err := supervisor.Close(); err != nil {
			t.Errorf("could not close the supervisor: [%v]", err)
		}
	})

	// An isolated round emits exactly one ratio warning -- owned by the
	// round itself -- while the periodic supervisor only traces the returned
	// error at debug level instead of duplicating the warning.
	expectedMessage := "2 of 2 well-known peers unreachable; " +
		"node is not connected to any well-known peer"
	if count := countLogEntries(entries, "warn", expectedMessage); count != 1 {
		t.Errorf(
			"expected 1 warning entry with message [%s], got %d",
			expectedMessage,
			count,
		)
	}

	roundErrorMessage := "well-known peers connection round error"
	if count := countLogEntries(entries, "warn", roundErrorMessage); count != 0 {
		t.Errorf(
			"expected no warning entries with message [%s], got %d",
			roundErrorMessage,
			count,
		)
	}
	if count := countLogEntries(entries, "debug", roundErrorMessage); count != 1 {
		t.Errorf(
			"expected 1 debug entry with message [%s], got %d",
			roundErrorMessage,
			count,
		)
	}
}
