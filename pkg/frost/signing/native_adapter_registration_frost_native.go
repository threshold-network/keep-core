//go:build frost_native

package signing

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// buildTaggedNativeExecutionAdapter is a placeholder adapter wired when
// the frost_native build tag is enabled.
type buildTaggedNativeExecutionAdapter struct{}

func registerNativeExecutionAdapterForBuild() {
	err := RegisterNativeExecutionAdapter(&buildTaggedNativeExecutionAdapter{})
	if err != nil {
		panic(fmt.Sprintf("failed to register build-tagged native adapter: [%v]", err))
	}
}

func (btnea *buildTaggedNativeExecutionAdapter) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	return nil, fmt.Errorf(
		"%w: build tag [frost_native] uses placeholder adapter",
		ErrNativeExecutionBackendNotImplemented,
	)
}

func (btnea *buildTaggedNativeExecutionAdapter) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}
