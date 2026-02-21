package signing

// RegisterNativeExecutionFFISigningPrimitiveForBuild attempts to register
// build-flavor native FFI signing primitive bindings.
//
// On default builds, this is a no-op.
// On `frost_native` builds, this can be wired to a concrete primitive.
func RegisterNativeExecutionFFISigningPrimitiveForBuild() {
	registerNativeExecutionFFISigningPrimitiveForBuild()
}
