//go:build tools

// tools.go pins dependencies that `go mod tidy` would otherwise drop
// because they are only referenced under the `tools` build tag (or are
// no longer referenced at all). They remain in go.mod / go.sum so version
// resolution stays reproducible for codegen and tooling that does pull
// them in.
package tools

import (
	_ "github.com/ferranbt/fastssz"
	_ "github.com/graph-gophers/graphql-go"
	_ "github.com/influxdata/influxdb-client-go/v2"
	_ "github.com/influxdata/influxdb1-client"
	_ "github.com/peterh/liner"
)
