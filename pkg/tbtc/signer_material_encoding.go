package tbtc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

var signerMaterialEnvelopePrefix = []byte("tbtc-signer-material-v1:")

type unmarshaledSignerMaterial struct {
	signerMaterial  any
	privateKeyShare *tecdsa.PrivateKeyShare
}

func marshalSignerMaterialForPersistence(
	signerMaterial any,
	fallbackPrivateKeyShare *tecdsa.PrivateKeyShare,
) ([]byte, error) {
	if signerMaterial == nil {
		signerMaterial = fallbackPrivateKeyShare
	}

	switch material := signerMaterial.(type) {
	case *tecdsa.PrivateKeyShare:
		if material == nil {
			return nil, fmt.Errorf("legacy private key share is nil")
		}

		return material.Marshal()
	case tecdsa.PrivateKeyShare:
		materialCopy := material
		return (&materialCopy).Marshal()
	case *frostsigning.NativeSignerMaterial:
		if material == nil {
			return nil, fmt.Errorf("native signer material is nil")
		}

		return encodeNativeSignerMaterialForPersistence(
			material.Format,
			material.Payload,
		)
	case frostsigning.NativeSignerMaterial:
		return encodeNativeSignerMaterialForPersistence(
			material.Format,
			material.Payload,
		)
	case []byte:
		// Transitional compatibility: raw bytes are treated as
		// frost-uniffi-v1 payloads produced by default resolver paths.
		return encodeNativeSignerMaterialForPersistence(
			frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			material,
		)
	default:
		return nil, fmt.Errorf("unsupported signer material type: [%T]", signerMaterial)
	}
}

func unmarshalSignerMaterialFromPersistence(
	data []byte,
) (*unmarshaledSignerMaterial, error) {
	nativeSignerMaterial, isNative, err := decodeNativeSignerMaterialFromPersistence(
		data,
	)
	if err != nil {
		return nil, err
	}

	if isNative {
		privateKeyShare := legacyPrivateKeyShareFromNativeSignerMaterial(
			nativeSignerMaterial,
		)

		return &unmarshaledSignerMaterial{
			signerMaterial:  nativeSignerMaterial,
			privateKeyShare: privateKeyShare,
		}, nil
	}

	privateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := privateKeyShare.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("cannot unmarshal private key share: [%w]", err)
	}

	resolvedSignerMaterial, err := resolveSignerMaterial(privateKeyShare)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot resolve signer material from legacy private key share: [%w]",
			err,
		)
	}

	if resolvedSignerMaterial == nil {
		return nil, fmt.Errorf(
			"resolved signer material from legacy private key share is nil",
		)
	}

	return &unmarshaledSignerMaterial{
		signerMaterial:  resolvedSignerMaterial,
		privateKeyShare: privateKeyShare,
	}, nil
}

func encodeNativeSignerMaterialForPersistence(
	format string,
	payload []byte,
) ([]byte, error) {
	material := &frostsigning.NativeSignerMaterial{
		Format:  format,
		Payload: append([]byte{}, payload...),
	}

	if err := validateNativeSignerMaterialForPersistence(material); err != nil {
		return nil, err
	}

	result := make([]byte, 0, len(signerMaterialEnvelopePrefix)+len(format)+len(payload)+20)
	result = append(result, signerMaterialEnvelopePrefix...)

	var varintBuffer [binary.MaxVarintLen64]byte

	formatLength := binary.PutUvarint(varintBuffer[:], uint64(len(material.Format)))
	result = append(result, varintBuffer[:formatLength]...)
	result = append(result, []byte(material.Format)...)

	payloadLength := binary.PutUvarint(varintBuffer[:], uint64(len(material.Payload)))
	result = append(result, varintBuffer[:payloadLength]...)
	result = append(result, material.Payload...)

	return result, nil
}

func decodeNativeSignerMaterialFromPersistence(
	data []byte,
) (
	*frostsigning.NativeSignerMaterial,
	bool,
	error,
) {
	if !bytes.HasPrefix(data, signerMaterialEnvelopePrefix) {
		return nil, false, nil
	}

	offset := len(signerMaterialEnvelopePrefix)

	formatLength, lengthBytes, err := readPersistenceUvarint(data, offset)
	if err != nil {
		return nil, true, fmt.Errorf("invalid signer material format length: [%w]", err)
	}
	offset += lengthBytes

	if offset+int(formatLength) > len(data) {
		return nil, true, fmt.Errorf("signer material format length exceeds payload")
	}

	format := string(data[offset : offset+int(formatLength)])
	offset += int(formatLength)

	payloadLength, lengthBytes, err := readPersistenceUvarint(data, offset)
	if err != nil {
		return nil, true, fmt.Errorf("invalid signer material payload length: [%w]", err)
	}
	offset += lengthBytes

	if offset+int(payloadLength) > len(data) {
		return nil, true, fmt.Errorf("signer material payload length exceeds payload")
	}

	payload := append([]byte{}, data[offset:offset+int(payloadLength)]...)
	offset += int(payloadLength)

	if offset != len(data) {
		return nil, true, fmt.Errorf("unexpected trailing signer material payload bytes")
	}

	material := &frostsigning.NativeSignerMaterial{
		Format:  format,
		Payload: payload,
	}

	if err := validateNativeSignerMaterialForPersistence(material); err != nil {
		return nil, true, err
	}

	return material, true, nil
}

func validateNativeSignerMaterialForPersistence(
	material *frostsigning.NativeSignerMaterial,
) error {
	if material == nil {
		return fmt.Errorf("native signer material is nil")
	}

	if material.Format == "" {
		return fmt.Errorf("native signer material format is empty")
	}

	if len(material.Payload) == 0 {
		return fmt.Errorf("native signer material payload is empty")
	}

	return nil
}

func readPersistenceUvarint(data []byte, offset int) (uint64, int, error) {
	if offset >= len(data) {
		return 0, 0, fmt.Errorf("offset [%d] out of bounds", offset)
	}

	value, lengthBytes := binary.Uvarint(data[offset:])
	if lengthBytes == 0 {
		return 0, 0, fmt.Errorf("incomplete uvarint")
	}

	if lengthBytes < 0 {
		return 0, 0, fmt.Errorf("overflowed uvarint")
	}

	return value, lengthBytes, nil
}

func legacyPrivateKeyShareFromNativeSignerMaterial(
	nativeSignerMaterial *frostsigning.NativeSignerMaterial,
) *tecdsa.PrivateKeyShare {
	if nativeSignerMaterial == nil {
		return nil
	}

	switch nativeSignerMaterial.Format {
	case frostsigning.NativeSignerMaterialFormatFrostUniFFIV1:
		privateKeyShare := &tecdsa.PrivateKeyShare{}
		if err := privateKeyShare.Unmarshal(nativeSignerMaterial.Payload); err != nil {
			return nil
		}

		return privateKeyShare

	case frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1:
		var payload tbtcSignerMaterialPayload
		if err := json.Unmarshal(nativeSignerMaterial.Payload, &payload); err != nil {
			return nil
		}

		if payload.LegacyPrivateKeyShareHex == "" {
			return nil
		}

		legacyPayload, err := hex.DecodeString(payload.LegacyPrivateKeyShareHex)
		if err != nil {
			return nil
		}

		privateKeyShare := &tecdsa.PrivateKeyShare{}
		if err := privateKeyShare.Unmarshal(legacyPayload); err != nil {
			return nil
		}

		return privateKeyShare

	default:
		return nil
	}
}
