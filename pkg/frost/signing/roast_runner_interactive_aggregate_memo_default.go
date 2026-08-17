//go:build !frost_native

package signing

// InteractiveAggregateMemoSession is a build-safe no-op in binaries that do
// not include the native interactive signing engine.
type InteractiveAggregateMemoSession struct{}

func BeginInteractiveAggregateMemoSession(
	sessionID string,
) (*InteractiveAggregateMemoSession, error) {
	return &InteractiveAggregateMemoSession{}, nil
}

func (session *InteractiveAggregateMemoSession) Release() {}
