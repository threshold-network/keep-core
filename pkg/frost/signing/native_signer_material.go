package signing

import "fmt"

const (
	// NativeSignerMaterialFormatFrostUniFFIV1 is the canonical format name for
	// serialized signer material expected by UniFFI-based native FROST bridges.
	NativeSignerMaterialFormatFrostUniFFIV1 = "frost-uniffi-v1"
)

// NativeSignerMaterial carries backend-native signer material required by
// native FROST execution paths.
type NativeSignerMaterial struct {
	Format  string
	Payload []byte
}

func (nsm *NativeSignerMaterial) clone() *NativeSignerMaterial {
	if nsm == nil {
		return nil
	}

	result := &NativeSignerMaterial{
		Format: nsm.Format,
	}

	if len(nsm.Payload) > 0 {
		result.Payload = append([]byte{}, nsm.Payload...)
	}

	return result
}

func (nsm *NativeSignerMaterial) validate() error {
	if nsm == nil {
		return fmt.Errorf("native signer material is nil")
	}

	if nsm.Format == "" {
		return fmt.Errorf("native signer material format is empty")
	}

	if len(nsm.Payload) == 0 {
		return fmt.Errorf("native signer material payload is empty")
	}

	return nil
}

// NativeSignerMaterial resolves native signer material required by
// FFI-backed native execution.
//
// Supported Request.SignerMaterial forms:
//   - *NativeSignerMaterial
//   - NativeSignerMaterial
//   - []byte (interpreted as NativeSignerMaterialFormatFrostUniFFIV1 payload)
func (r *Request) NativeSignerMaterial() (*NativeSignerMaterial, error) {
	if r == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if r.SignerMaterial == nil {
		return nil, fmt.Errorf("native signer material is nil")
	}

	var nativeSignerMaterial *NativeSignerMaterial

	switch signerMaterial := r.SignerMaterial.(type) {
	case *NativeSignerMaterial:
		nativeSignerMaterial = signerMaterial.clone()
	case NativeSignerMaterial:
		nativeSignerMaterial = signerMaterial.clone()
	case []byte:
		nativeSignerMaterial = &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: append([]byte{}, signerMaterial...),
		}
	default:
		return nil, fmt.Errorf(
			"native signer material has wrong type: [%T]",
			r.SignerMaterial,
		)
	}

	if err := nativeSignerMaterial.validate(); err != nil {
		return nil, err
	}

	return nativeSignerMaterial, nil
}
