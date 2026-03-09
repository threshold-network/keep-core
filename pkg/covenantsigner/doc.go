// Package covenantsigner implements keep-core covenant signer substrate slices:
// durable submit/poll semantics, strict request validation, and a compatible
// HTTP surface for covenant recovery/presign signer jobs. The current branch
// also validates the concrete migration destination reservation artifact that
// later real signing flows will consume.
package covenantsigner
