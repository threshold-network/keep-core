package flag

// errorMessage returns err.Error() or "" if err is nil. Used to compare error
// equality by message string rather than by concrete type, since pflag wraps
// underlying errors with fmt.Errorf("...: %w", inner) — the rendered message
// stays stable across pflag versions but the concrete type does not.
func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
