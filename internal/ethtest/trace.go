package ethtest

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// ExpectedCall is one entry of a contract-read trace: the contract the read
// must land on, the method it must select and the arguments it must carry.
type ExpectedCall struct {
	Contract string
	Method   string
	Args     []interface{}
}

// Read builds a trace entry.
func Read(contract, method string, args ...interface{}) ExpectedCall {
	return ExpectedCall{Contract: contract, Method: method, Args: args}
}

// AssertTrace fails unless the reads served since the fixture last forgot them
// are exactly the expected ones, in order. Comparing whole traces rather than
// counting matches means a repeated read, a read of another modelled method or
// a read in the wrong order is a failure rather than something an assertion
// over one method happens not to look at.
//
// Each entry is rendered from the target address the contract is registered at,
// the selector the caller actually encoded and the decoded arguments, so the
// comparison covers what went over the wire and not only what the fixture
// resolved it to.
func (b *Backend) AssertTrace(
	t *testing.T,
	description string,
	expected ...ExpectedCall,
) {
	t.Helper()

	want := make([]string, 0, len(expected))
	for _, entry := range expected {
		want = append(want, b.renderExpected(t, entry))
	}

	served := b.Calls()
	got := make([]string, 0, len(served))
	for _, call := range served {
		got = append(got, renderCall(call))
	}

	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Errorf(
			"unexpected %s\nexpected:\n  %s\nactual:\n  %s",
			description,
			strings.Join(want, "\n  "),
			strings.Join(got, "\n  "),
		)
	}
}

// renderExpected renders a trace entry the way a matching served read renders,
// resolving the target address and selector from the registered contract.
func (b *Backend) renderExpected(t *testing.T, entry ExpectedCall) string {
	t.Helper()

	b.mutex.Lock()
	defer b.mutex.Unlock()

	for address, registered := range b.contracts {
		if registered.name != entry.Contract {
			continue
		}

		method, known := registered.abi.Methods[entry.Method]
		if !known {
			t.Fatalf(
				"the %s ABI declares no method [%s]",
				entry.Contract,
				entry.Method,
			)
		}

		return render(entry.Contract, address, method.ID, entry.Args)
	}

	t.Fatalf("the fixture serves no contract named [%s]", entry.Contract)

	return ""
}

func renderCall(call Call) string {
	return render(call.Contract, call.To, call.Selector, call.Args)
}

func render(
	contractName string,
	address common.Address,
	selector []byte,
	args []interface{},
) string {
	rendered := make([]string, 0, len(args))
	for _, arg := range args {
		rendered = append(rendered, fmt.Sprintf("%v", arg))
	}

	return fmt.Sprintf(
		"%s@%s.%s(%s)",
		contractName,
		address.Hex(),
		hex.EncodeToString(selector),
		strings.Join(rendered, ", "),
	)
}
