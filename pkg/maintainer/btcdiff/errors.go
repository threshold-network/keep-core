package btcdiff

import "errors"

// ErrUniformPreRetargetDifficulty is returned when a pre-retarget header's
// difficulty target is neither the current epoch's target (from the anchor
// block) nor the minimum-difficulty target allowed by LightRelay (testnet
// min-diff blocks).
var ErrUniformPreRetargetDifficulty = errors.New(
	"bitcoin pre-retarget headers are not valid for LightRelay",
)
