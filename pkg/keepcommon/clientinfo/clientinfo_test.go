package clientinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

var registry *Registry

const port = 9799

func TestMain(m *testing.M) {
	registry = NewRegistry()
	registry.EnableServer(port)

	os.Exit(m.Run())
}

func TestClientInfoServerMetrics(t *testing.T) {
	// Register gauge metric
	gauge, err := registry.NewMetricGauge(
		"test_gauge",
		NewLabel("gaugeLabelName", "gauge-label-value"),
	)
	if err != nil {
		t.Fatal(err)
	}
	gauge.value = float64(12)
	gauge.timestamp = int64(51241)

	// Register info metric
	if _, err := registry.NewMetricInfo(
		"test_info",
		[]Label{
			NewLabel("infoLabelName", "info-label-value"),
		},
	); err != nil {
		t.Fatal(err)
	}

	// Assemble the expected endpoint response
	expected := new(strings.Builder)
	expected.WriteString("# TYPE test_gauge gauge\n")
	expected.WriteString("test_gauge{gaugeLabelName=\"gauge-label-value\"} 12 51241\n\n")
	expected.WriteString("test_info{infoLabelName=\"info-label-value\"} 1\n")

	// Execute Test
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("failed to get metrics: %v", err)
	}

	actual := new(strings.Builder)
	_, err = io.Copy(actual, resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if actual.String() != expected.String() {
		t.Fatalf(
			"unexpected server response\n"+
				"expected:\n[%s]\n"+
				"actual:\n[%v]\n",
			expected.String(),
			actual.String(),
		)
	}
}

func TestClientInfoServerDiagnostics(t *testing.T) {

	// Register Diagnostics Sources
	nodeChainAddress := "0x1234567890"
	registry.RegisterDiagnosticSource("chain_address", func() string {
		result, err := json.Marshal(nodeChainAddress)
		if err != nil {
			t.Fatal(err)
		}
		return string(result)
	})

	peers := []testDiagnosticsPeerInfo{
		{
			ChainAddress:          "0xABCdef",
			NetworkMultiAddresses: []string{"/dns4/address1/3919/ipfs/AaBbCc"},
		},
		{
			ChainAddress:          "0x98765A",
			NetworkMultiAddresses: []string{"/ip4/127.0.0.1/3919/ipfs/963"},
		},
	}
	registry.RegisterDiagnosticSource("connected_peers", func() string {
		result, err := json.Marshal(peers)
		if err != nil {
			t.Fatal(err)
		}
		return string(result)
	})

	expectedDiagnostics := &testDiagnosticsInfo{nodeChainAddress, peers}

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/diagnostics", port))
	if err != nil {
		t.Fatalf("failed to get diagnostics: %v", err)
	}

	actualDiagnostics := &testDiagnosticsInfo{}
	if err = json.NewDecoder(resp.Body).Decode(actualDiagnostics); err != nil {
		t.Fatalf("failed to decode diagnostics: %v", err)
	}

	if !reflect.DeepEqual(expectedDiagnostics, actualDiagnostics) {
		t.Errorf(
			"incorrect diagnostics\n"+
				"expected: %+v\n"+
				"actual:   %+v",
			expectedDiagnostics,
			actualDiagnostics,
		)
	}
}

type testDiagnosticsInfo struct {
	ChainAddress   string                    `json:"chain_address"`
	ConnectedPeers []testDiagnosticsPeerInfo `json:"connected_peers"`
}

type testDiagnosticsPeerInfo struct {
	ChainAddress          string   `json:"chain_address"`
	NetworkMultiAddresses []string `json:"multiaddrs"`
}
