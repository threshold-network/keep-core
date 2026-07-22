//go:build tools

// tools.go pins indirect dependencies that would otherwise be dropped by
// `go mod tidy`. They are anchored here with blank imports so they remain in
// go.mod and go.sum for reproducible builds, even though they are not
// referenced directly by runtime or generated code.
package tools

import (
	_ "github.com/ferranbt/fastssz"
	_ "github.com/graph-gophers/graphql-go"
	_ "github.com/influxdata/influxdb-client-go/v2"
	_ "github.com/influxdata/influxdb1-client"
	_ "github.com/peterh/liner"
)
