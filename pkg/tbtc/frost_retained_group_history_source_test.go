package tbtc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/keep-network/keep-core/pkg/chain"
	frostabi "github.com/keep-network/keep-core/pkg/chain/ethereum/frost/gen/abi"
)

type frostRetainedGroupHistoryTestVerifier struct {
	mutex          sync.Mutex
	chainID        *big.Int
	finalized      *types.Header
	headers        map[uint64]*types.Header
	reads          map[uint64]int
	reorgBlock     uint64
	reorgAfterRead int
	reorgHeader    *types.Header
	receipts       map[common.Hash]*types.Receipt
	code           map[common.Address][]byte
	sortitionPool  common.Address
	operator       common.Address
	operatorAt     uint64
	operatorID     uint32
	closed         bool
}

func (verifier *frostRetainedGroupHistoryTestVerifier) ChainID(
	context.Context,
) (*big.Int, error) {
	return new(big.Int).Set(verifier.chainID), nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) HeaderByNumber(
	_ context.Context,
	number *big.Int,
) (*types.Header, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if number.Sign() < 0 {
		return verifier.finalized, nil
	}
	blockNumber := number.Uint64()
	verifier.reads[blockNumber]++
	if blockNumber == verifier.reorgBlock && verifier.reorgHeader != nil &&
		verifier.reads[blockNumber] > verifier.reorgAfterRead {
		return verifier.reorgHeader, nil
	}
	return verifier.headers[blockNumber], nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) Close() {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	verifier.closed = true
}

func (verifier *frostRetainedGroupHistoryTestVerifier) TransactionReceipt(
	_ context.Context,
	hash common.Hash,
) (*types.Receipt, error) {
	receipt := verifier.receipts[hash]
	if receipt == nil {
		return nil, fmt.Errorf("missing receipt")
	}
	return receipt, nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) CodeAt(
	_ context.Context,
	address common.Address,
	_ *big.Int,
) ([]byte, error) {
	code := verifier.code[address]
	if len(code) == 0 {
		return nil, fmt.Errorf("missing code")
	}
	return append([]byte{}, code...), nil
}

func (verifier *frostRetainedGroupHistoryTestVerifier) CallContract(
	_ context.Context,
	call ethereum.CallMsg,
	block *big.Int,
) ([]byte, error) {
	if call.To == nil || *call.To != verifier.sortitionPool || block == nil ||
		!block.IsUint64() || block.Uint64() != verifier.operatorAt || len(call.Data) != 36 ||
		!bytes.Equal(call.Data[:4], []byte{0x5a, 0x48, 0xb4, 0x6b}) ||
		!bytes.Equal(call.Data[4:16], make([]byte, 12)) ||
		!bytes.Equal(call.Data[16:], verifier.operator[:]) {
		return nil, fmt.Errorf("unexpected operator call")
	}
	result := make([]byte, 32)
	binary.BigEndian.PutUint32(result[28:], verifier.operatorID)
	return result, nil
}

type frostRetainedGroupHistoryTestExport struct {
	t                 *testing.T
	privateKey        ed25519.PrivateKey
	publicKeyDER      []byte
	historyResponder  func(frostRetainedGroupHistoryPageRequest) interface{}
	operatorResponder func(frostRetainedGroupOperatorQuery) interface{}
	corruptSignature  bool
}

func (export *frostRetainedGroupHistoryTestExport) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "method", http.StatusMethodNotAllowed)
		return
	}
	var payload interface{}
	switch request.URL.Path {
	case "/history":
		historyRequest := frostRetainedGroupHistoryPageRequest{}
		if err := json.NewDecoder(request.Body).Decode(&historyRequest); err != nil {
			http.Error(responseWriter, "request", http.StatusBadRequest)
			return
		}
		payload = export.historyResponder(historyRequest)
	case "/operator-id":
		operatorRequest := frostRetainedGroupOperatorQuery{}
		if err := json.NewDecoder(request.Body).Decode(&operatorRequest); err != nil {
			http.Error(responseWriter, "request", http.StatusBadRequest)
			return
		}
		payload = export.operatorResponder(operatorRequest)
	default:
		http.NotFound(responseWriter, request)
		return
	}
	if payload == nil {
		http.Error(responseWriter, "missing", http.StatusServiceUnavailable)
		return
	}
	canonical, err := canonicalFrostActivationValue(payload)
	if err != nil {
		export.t.Fatal(err)
	}
	payloadHash := sha256.Sum256(canonical)
	signed := append([]byte(frostRetainedGroupHistorySignatureDomain), canonical...)
	signature := ed25519.Sign(export.privateKey, signed)
	if export.corruptSignature {
		signature[0] ^= 0xff
	}
	envelope := frostRetainedGroupSignedEnvelope{
		Schema:              "tbtc-frost-retained-group-signed-envelope/v1",
		Payload:             json.RawMessage(canonical),
		PayloadSHA256:       frostActivationHex32(payloadHash),
		SignerPublicKeySPKI: base64.StdEncoding.EncodeToString(export.publicKeyDER),
		SignatureAlgorithm:  "ed25519",
		Signature:           base64.StdEncoding.EncodeToString(signature),
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(responseWriter).Encode(envelope); err != nil {
		export.t.Fatal(err)
	}
}

type frostRetainedGroupHistorySourceFixture struct {
	t                 *testing.T
	verifier          *frostRetainedGroupHistoryTestVerifier
	export            *frostRetainedGroupHistoryTestExport
	server            *httptest.Server
	source            *signedFrostRetainedGroupHistorySource
	identity          FrostRetainedGroupHistoryIdentity
	profile           FrostPreSignActivationProfile
	from              FrostPreSignFinality
	to                FrostPreSignFinality
	descriptorSetHash [32]byte
	snapshotID        [32]byte
	mutations         []FrostRetainedGroupMutation
	pageMutator       func(*frostRetainedGroupHistoryPagePayload)
	operatorMutator   func(*frostRetainedGroupOperatorReceiptPayload)
}

func newFrostRetainedGroupHistorySourceFixture(
	t *testing.T,
) *frostRetainedGroupHistorySourceFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	operatorFingerprint := sha256.Sum256(publicKeyDER)
	headers := make(map[uint64]*types.Header)
	for blockNumber := uint64(1); blockNumber <= 10; blockNumber++ {
		headers[blockNumber] = &types.Header{
			Number: new(big.Int).SetUint64(blockNumber),
			Time:   blockNumber,
			Extra:  []byte{byte(blockNumber), 0x9a},
		}
	}
	bridgeCode := []byte{0x60, 0x01, 0x60, 0x02}
	registryCode := []byte{0x60, 0x03, 0x60, 0x04}
	sortitionPoolCode := []byte{0x60, 0x05, 0x60, 0x06}
	profile := frostRetainedGroupHistoryTestProfile(
		bridgeCode,
		registryCode,
		sortitionPoolCode,
	)
	operatorAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")
	verifier := &frostRetainedGroupHistoryTestVerifier{
		chainID:       big.NewInt(1),
		finalized:     headers[10],
		headers:       headers,
		reads:         make(map[uint64]int),
		receipts:      make(map[common.Hash]*types.Receipt),
		code:          make(map[common.Address][]byte),
		sortitionPool: common.Address(profile.SortitionPool),
		operator:      operatorAddress,
		operatorAt:    6,
		operatorID:    17,
	}
	verifier.code[common.Address(profile.BridgeAddress)] = bridgeCode
	verifier.code[common.Address(profile.FrostRegistry)] = registryCode
	verifier.code[common.Address(profile.SortitionPool)] = sortitionPoolCode
	export := &frostRetainedGroupHistoryTestExport{
		t:            t,
		privateKey:   privateKey,
		publicKeyDER: publicKeyDER,
	}
	server := httptest.NewServer(export)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	identity := FrostRetainedGroupHistoryIdentity{
		TrustDomainID:       "independent-retained-history-test",
		EndpointFingerprint: [32]byte{0x31},
		OperatorFingerprint: operatorFingerprint,
	}
	source, err := newSignedFrostRetainedGroupHistorySource(
		context.Background(),
		endpoint,
		verifier,
		server.Client(),
		1,
		identity,
		operatorFingerprint,
	)
	if err != nil {
		t.Fatal(err)
	}
	descriptorSetHash := [32]byte{0x42}
	if err := source.BindFrostRetainedGroupActivationEvidence(
		profile,
		profile.ActivationManifestHash,
		descriptorSetHash,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(source.Close)
	fixture := &frostRetainedGroupHistorySourceFixture{
		t:                 t,
		verifier:          verifier,
		export:            export,
		server:            server,
		source:            source,
		identity:          identity,
		profile:           profile,
		from:              FrostPreSignFinality{BlockNumber: 1, BlockHash: headers[1].Hash()},
		to:                FrostPreSignFinality{BlockNumber: 6, BlockHash: headers[6].Hash()},
		descriptorSetHash: descriptorSetHash,
		snapshotID:        [32]byte{0x53},
	}
	fixture.mutations = frostRetainedGroupHistoryTestMutations(t, headers, source.evidence)
	fixture.installReceipts()
	export.historyResponder = fixture.historyResponse
	export.operatorResponder = fixture.operatorResponse
	return fixture
}

func frostRetainedGroupHistoryTestProfile(
	bridgeCode []byte,
	registryCode []byte,
	sortitionPoolCode []byte,
) FrostPreSignActivationProfile {
	profile := FrostPreSignActivationProfile{
		DomainChainID:             [32]byte{0x01},
		ActivationManifestHash:    [32]byte{0x02},
		ImplementationSetHash:     [32]byte{0x03},
		BridgeAddress:             [20]byte{0x11},
		RegistryAddress:           [20]byte{0x12},
		CompleteRouter:            [20]byte{0x13},
		FrostRegistry:             [20]byte{0x14},
		ProposalValidator:         [20]byte{0x15},
		SortitionPool:             [20]byte{0x16},
		BridgeCodeHash:            [32]byte(crypto.Keccak256Hash(bridgeCode)),
		RegistryCodeHash:          [32]byte{0x22},
		CompleteRouterCodeHash:    [32]byte{0x23},
		FrostRegistryCodeHash:     [32]byte(crypto.Keccak256Hash(registryCode)),
		ProposalValidatorCodeHash: [32]byte{0x25},
		SortitionPoolCodeHash:     [32]byte(crypto.Keccak256Hash(sortitionPoolCode)),
		ReservationProtocolID:     frostPreSignReservationProtocolID(),
		EvidenceProtocolID:        frostCompleteEvidenceProtocolID(),
		SigningPolicyHash:         frostPreSignSigningPolicyHash(),
	}
	profile.ProfileHash = profile.ComputeHash()
	return profile
}

func frostRetainedGroupHistoryTestMutations(
	t *testing.T,
	headers map[uint64]*types.Header,
	evidence *frostRetainedGroupEvidenceProfile,
) []FrostRetainedGroupMutation {
	t.Helper()
	walletID := [32]byte{0x71}
	walletPublicKeyHash := [20]byte{0x72}
	operatorIDs := make([]uint32, 51)
	for index := range operatorIDs {
		operatorIDs[index] = uint32(index + 1)
	}
	dkgSubmission := FrostRetainedGroupEventPoint{
		BlockNumber:      2,
		BlockHash:        headers[2].Hash(),
		TransactionHash:  [32]byte{0xa2},
		TransactionIndex: 0,
		LogIndex:         2,
	}
	admissionTransaction := [32]byte{0xa3}
	dkgApproval := FrostRetainedGroupEventPoint{
		BlockNumber:      3,
		BlockHash:        headers[3].Hash(),
		TransactionHash:  admissionTransaction,
		TransactionIndex: 3,
		LogIndex:         10,
	}
	creation := FrostRetainedGroupEventPoint{
		BlockNumber:      3,
		BlockHash:        headers[3].Hash(),
		TransactionHash:  admissionTransaction,
		TransactionIndex: 3,
		LogIndex:         11,
	}
	registration := creation
	registration.LogIndex = 12
	closing := FrostRetainedGroupEventPoint{
		BlockNumber:      4,
		BlockHash:        headers[4].Hash(),
		TransactionHash:  [32]byte{0xa4},
		TransactionIndex: 1,
		LogIndex:         20,
	}
	closureTransaction := [32]byte{0xa5}
	closed := FrostRetainedGroupEventPoint{
		BlockNumber:      5,
		BlockHash:        headers[5].Hash(),
		TransactionHash:  closureTransaction,
		TransactionIndex: 2,
		LogIndex:         30,
	}
	registryClosure := closed
	registryClosure.LogIndex = 31
	dkgResult, _, dkgResultHash := frostRetainedGroupHistoryTestDkgResult(
		t,
		evidence,
		walletID,
		operatorIDs,
	)
	return []FrostRetainedGroupMutation{
		{
			Point:                   registration,
			Kind:                    FrostRetainedGroupAdmissionMutation,
			WalletID:                walletID,
			WalletPublicKeyHash:     walletPublicKeyHash,
			OperatorIDs:             operatorIDs,
			RetainedGroupHash:       dkgResult.MembersHash,
			DkgResultHash:           dkgResultHash,
			DkgSubmissionPoint:      dkgSubmission,
			DkgApprovalPoint:        dkgApproval,
			CreationPoint:           creation,
			BridgeRegistrationPoint: registration,
		},
		{
			Point:               closing,
			Kind:                FrostRetainedGroupClosingMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
		{
			Point:               closed,
			Kind:                FrostRetainedGroupClosedMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
		{
			Point:               registryClosure,
			Kind:                FrostRetainedGroupRegistryClosureMutation,
			WalletID:            walletID,
			WalletPublicKeyHash: walletPublicKeyHash,
		},
	}
}

func frostRetainedGroupHistoryTestDkgResult(
	t *testing.T,
	evidence *frostRetainedGroupEvidenceProfile,
	walletID [32]byte,
	operatorIDs []uint32,
) (frostabi.FrostDkgResult, []byte, [32]byte) {
	t.Helper()
	encodedMembers, err := evidence.membersArguments.Pack(operatorIDs)
	if err != nil {
		t.Fatal(err)
	}
	result := frostabi.FrostDkgResult{
		SubmitterMemberIndex:     big.NewInt(1),
		XOnlyOutputKey:           walletID,
		MisbehavedMembersIndices: []uint8{},
		Signatures:               []byte{0x01, 0x02},
		SigningMembersIndices:    []*big.Int{big.NewInt(1)},
		Members:                  append([]uint32{}, operatorIDs...),
		MembersHash:              [32]byte(crypto.Keccak256Hash(encodedMembers)),
	}
	data, err := evidence.registryABI.Events["DkgResultSubmitted"].Inputs.NonIndexed().Pack(result)
	if err != nil {
		t.Fatal(err)
	}
	return result, data, [32]byte(crypto.Keccak256Hash(data))
}

func (fixture *frostRetainedGroupHistorySourceFixture) installReceipts() {
	fixture.t.Helper()
	evidence, err := fixture.source.activationEvidence()
	if err != nil {
		fixture.t.Fatal(err)
	}
	admission := fixture.mutations[0]
	_, dkgData, _ := frostRetainedGroupHistoryTestDkgResult(
		fixture.t,
		evidence,
		admission.WalletID,
		admission.OperatorIDs,
	)
	submissionLog := frostRetainedGroupHistoryTestLog(
		admission.DkgSubmissionPoint,
		evidence.registry.address,
		[]common.Hash{
			evidence.registryABI.Events["DkgResultSubmitted"].ID,
			common.Hash(admission.DkgResultHash),
			common.BigToHash(big.NewInt(123)),
		},
		dkgData,
	)
	fixture.verifier.receipts[common.Hash(admission.DkgSubmissionPoint.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(admission.DkgSubmissionPoint, submissionLog)

	approvalLog := frostRetainedGroupHistoryTestLog(
		admission.DkgApprovalPoint,
		evidence.registry.address,
		[]common.Hash{
			evidence.registryABI.Events["DkgResultApproved"].ID,
			common.Hash(admission.DkgResultHash),
			common.BytesToHash(common.HexToAddress("0x2222222222222222222222222222222222222222").Bytes()),
		},
		nil,
	)
	creationLog := frostRetainedGroupHistoryTestLog(
		admission.CreationPoint,
		evidence.registry.address,
		[]common.Hash{
			evidence.registryABI.Events["WalletCreated"].ID,
			common.Hash(admission.WalletID),
			common.Hash(admission.DkgResultHash),
		},
		nil,
	)
	registrationLog := frostRetainedGroupHistoryTestLog(
		admission.BridgeRegistrationPoint,
		evidence.bridge.address,
		[]common.Hash{
			evidence.bridgeABI.Events["NewWalletRegisteredV2"].ID,
			common.Hash(admission.WalletID),
			{},
			frostRetainedGroupBytes20Topic(admission.WalletPublicKeyHash),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(admission.DkgApprovalPoint.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(
			admission.DkgApprovalPoint,
			approvalLog,
			creationLog,
			registrationLog,
		)

	closing := fixture.mutations[1]
	closingLog := frostRetainedGroupHistoryTestLog(
		closing.Point,
		evidence.bridge.address,
		[]common.Hash{
			evidence.bridgeABI.Events["WalletClosing"].ID,
			{},
			frostRetainedGroupBytes20Topic(closing.WalletPublicKeyHash),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(closing.Point.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(closing.Point, closingLog)

	closed := fixture.mutations[2]
	registryClosure := fixture.mutations[3]
	closedLog := frostRetainedGroupHistoryTestLog(
		closed.Point,
		evidence.bridge.address,
		[]common.Hash{
			evidence.bridgeABI.Events["WalletClosed"].ID,
			{},
			frostRetainedGroupBytes20Topic(closed.WalletPublicKeyHash),
		},
		nil,
	)
	registryClosureLog := frostRetainedGroupHistoryTestLog(
		registryClosure.Point,
		evidence.registry.address,
		[]common.Hash{
			evidence.registryABI.Events["WalletClosed"].ID,
			common.Hash(registryClosure.WalletID),
		},
		nil,
	)
	fixture.verifier.receipts[common.Hash(closed.Point.TransactionHash)] =
		frostRetainedGroupHistoryTestReceipt(closed.Point, closedLog, registryClosureLog)
}

func frostRetainedGroupHistoryTestLog(
	point FrostRetainedGroupEventPoint,
	address common.Address,
	topics []common.Hash,
	data []byte,
) *types.Log {
	return &types.Log{
		Address:     address,
		Topics:      append([]common.Hash{}, topics...),
		Data:        append([]byte{}, data...),
		BlockNumber: point.BlockNumber,
		TxHash:      common.Hash(point.TransactionHash),
		TxIndex:     uint(point.TransactionIndex),
		BlockHash:   common.Hash(point.BlockHash),
		Index:       uint(point.LogIndex),
	}
}

func frostRetainedGroupHistoryTestReceipt(
	point FrostRetainedGroupEventPoint,
	logs ...*types.Log,
) *types.Receipt {
	return &types.Receipt{
		Status:           types.ReceiptStatusSuccessful,
		TxHash:           common.Hash(point.TransactionHash),
		BlockHash:        common.Hash(point.BlockHash),
		BlockNumber:      new(big.Int).SetUint64(point.BlockNumber),
		TransactionIndex: uint(point.TransactionIndex),
		Logs:             logs,
	}
}

func (fixture *frostRetainedGroupHistorySourceFixture) historyPages(
	query frostRetainedGroupHistoryQuery,
) []*frostRetainedGroupHistoryPagePayload {
	fixture.t.Helper()
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupHistoryQueryDomain, query)
	if err != nil {
		fixture.t.Fatal(err)
	}
	wireMutations := make([]frostRetainedGroupWireMutation, len(fixture.mutations))
	for index, mutation := range fixture.mutations {
		wireMutations[index] = frostRetainedGroupMutationToWire(mutation)
	}
	pageCount := (len(wireMutations) + 1) / 2
	if pageCount == 0 {
		pageCount = 1
	}
	pages := make([]*frostRetainedGroupHistoryPagePayload, pageCount)
	previousPageHash := [32]byte{}
	identity := frostRetainedGroupWireIdentity{
		TrustDomainID:       fixture.identity.TrustDomainID,
		EndpointFingerprint: frostActivationHex32(fixture.identity.EndpointFingerprint),
		OperatorFingerprint: frostActivationHex32(fixture.identity.OperatorFingerprint),
	}
	for index := 0; index < pageCount; index++ {
		start := index * 2
		end := start + 2
		if end > len(wireMutations) {
			end = len(wireMutations)
		}
		cursor := ""
		if index > 0 {
			cursor = "page_" + string(rune('0'+index))
		}
		nextCursor := ""
		if index+1 < pageCount {
			nextCursor = "page_" + string(rune('0'+index+1))
		}
		page := &frostRetainedGroupHistoryPagePayload{
			Schema:            frostRetainedGroupHistoryPageSchema,
			Identity:          identity,
			ChainID:           1,
			QueryHash:         frostActivationHex32(queryHash),
			SnapshotID:        frostActivationHex32(fixture.snapshotID),
			PageIndex:         uint64(index),
			Cursor:            cursor,
			PreviousPageHash:  frostActivationHex32(previousPageHash),
			From:              query.From,
			To:                query.To,
			EmptyAtFrom:       true,
			DescriptorSetHash: frostActivationHex32(fixture.descriptorSetHash),
			Mutations:         append([]frostRetainedGroupWireMutation{}, wireMutations[start:end]...),
			NextCursor:        nextCursor,
			Complete:          index+1 == pageCount,
		}
		pages[index] = page
		canonical, err := canonicalFrostActivationValue(page)
		if err != nil {
			fixture.t.Fatal(err)
		}
		previousPageHash = sha256.Sum256(canonical)
	}
	historyRoot, err := frostRetainedGroupHistoryRoot(queryHash, wireMutations)
	if err != nil {
		fixture.t.Fatal(err)
	}
	pages[len(pages)-1].Receipt = &frostRetainedGroupHistoryReceipt{
		PageCount:     uint64(len(pages)),
		MutationCount: uint64(len(wireMutations)),
		HistoryRoot:   frostActivationHex32(historyRoot),
	}
	return pages
}

func (fixture *frostRetainedGroupHistorySourceFixture) historyResponse(
	request frostRetainedGroupHistoryPageRequest,
) interface{} {
	pages := fixture.historyPages(request.Query)
	for _, page := range pages {
		if page.Cursor == request.Cursor {
			copy := *page
			copy.Mutations = append([]frostRetainedGroupWireMutation{}, page.Mutations...)
			if page.Receipt != nil {
				receipt := *page.Receipt
				copy.Receipt = &receipt
			}
			if fixture.pageMutator != nil {
				fixture.pageMutator(&copy)
			}
			return &copy
		}
	}
	return nil
}

func (fixture *frostRetainedGroupHistorySourceFixture) operatorResponse(
	query frostRetainedGroupOperatorQuery,
) interface{} {
	queryHash, err := frostRetainedGroupDomainHash(frostRetainedGroupOperatorQueryDomain, query)
	if err != nil {
		fixture.t.Fatal(err)
	}
	payload := &frostRetainedGroupOperatorReceiptPayload{
		Schema: frostRetainedGroupOperatorReceiptSchema,
		Identity: frostRetainedGroupWireIdentity{
			TrustDomainID:       fixture.identity.TrustDomainID,
			EndpointFingerprint: frostActivationHex32(fixture.identity.EndpointFingerprint),
			OperatorFingerprint: frostActivationHex32(fixture.identity.OperatorFingerprint),
		},
		ChainID:         1,
		QueryHash:       frostActivationHex32(queryHash),
		OperatorAddress: query.OperatorAddress,
		At:              query.At,
		OperatorID:      17,
		Found:           true,
	}
	if fixture.operatorMutator != nil {
		fixture.operatorMutator(payload)
	}
	return payload
}

func TestSignedFrostRetainedGroupHistorySource_ReadsCompletePaginatedHistory(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	identity, err := fixture.source.Identity(context.Background())
	if err != nil || identity != fixture.identity {
		t.Fatalf("unexpected identity: [%v] [%v]", identity, err)
	}
	history, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !history.Complete || !history.EmptyAtFrom || len(history.Mutations) != 4 ||
		history.DescriptorSetHash != fixture.descriptorSetHash {
		t.Fatalf("unexpected complete history: [%+v]", history)
	}
	operatorID, err := fixture.source.ResolveOperatorID(
		context.Background(),
		chain.Address("0x1111111111111111111111111111111111111111"),
		fixture.to,
	)
	if err != nil || operatorID != 17 {
		t.Fatalf("unexpected operator ID: [%d] [%v]", operatorID, err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsOmittedPage(t *testing.T) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.export.historyResponder = func(request frostRetainedGroupHistoryPageRequest) interface{} {
		pages := fixture.historyPages(request.Query)
		return pages[len(pages)-1]
	}
	_, err := fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
	if err == nil || !strings.Contains(err.Error(), "wrong identity or position") {
		t.Fatalf("expected omitted-page rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsTruncationAndForgery(
	t *testing.T,
) {
	t.Run("truncated pagination", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		defaultResponder := fixture.export.historyResponder
		fixture.export.historyResponder = func(
			request frostRetainedGroupHistoryPageRequest,
		) interface{} {
			if request.Cursor != "" {
				return nil
			}
			return defaultResponder(request)
		}
		_, err := fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
		if err == nil || !strings.Contains(err.Error(), "HTTP status [503]") {
			t.Fatalf("expected truncated pagination rejection, got [%v]", err)
		}
	})

	t.Run("forged envelope", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.export.corruptSignature = true
		_, err := fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
		if err == nil || !strings.Contains(err.Error(), "signature is invalid") {
			t.Fatalf("expected forged envelope rejection, got [%v]", err)
		}
	})
}

func TestSignedFrostRetainedGroupHistorySource_RejectsDuplicateAndReorderedHistory(
	t *testing.T,
) {
	tests := map[string]func(*frostRetainedGroupHistorySourceFixture){
		"duplicate event": func(fixture *frostRetainedGroupHistorySourceFixture) {
			fixture.mutations = append(fixture.mutations, fixture.mutations[3])
		},
		"reordered event": func(fixture *frostRetainedGroupHistorySourceFixture) {
			fixture.mutations[0], fixture.mutations[1] = fixture.mutations[1], fixture.mutations[0]
		},
		"duplicate DKG member": func(fixture *frostRetainedGroupHistorySourceFixture) {
			fixture.mutations[0].OperatorIDs[1] = fixture.mutations[0].OperatorIDs[0]
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			mutate(fixture)
			_, err := fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
			if err == nil {
				t.Fatal("expected malformed exact history to be rejected")
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsIdentityCheckpointAndReceiptDrift(
	t *testing.T,
) {
	tests := map[string]func(*frostRetainedGroupHistoryPagePayload){
		"identity": func(page *frostRetainedGroupHistoryPagePayload) {
			page.Identity.TrustDomainID = "wrong-domain"
		},
		"checkpoint": func(page *frostRetainedGroupHistoryPagePayload) {
			page.From.BlockHash = frostActivationHex32([32]byte{0xff})
		},
		"manifest descriptor": func(page *frostRetainedGroupHistoryPagePayload) {
			page.DescriptorSetHash = frostActivationHex32([32]byte{0xdd})
		},
		"receipt": func(page *frostRetainedGroupHistoryPagePayload) {
			if page.Complete {
				page.Receipt.HistoryRoot = frostActivationHex32([32]byte{0xee})
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newFrostRetainedGroupHistorySourceFixture(t)
			fixture.pageMutator = mutate
			_, err := fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
			if err == nil {
				t.Fatalf("expected %s drift to be rejected", name)
			}
		})
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsSemanticForgeryInCanonicalBlock(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	for index := range fixture.mutations {
		fixture.mutations[index].WalletPublicKeyHash[0] ^= 0xff
	}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
	)
	if err == nil || !strings.Contains(err.Error(), "Bridge registration log") {
		t.Fatalf("expected canonical-block semantic forgery rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongReceiptLog(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	admission := fixture.mutations[0]
	receipt := fixture.verifier.receipts[common.Hash(admission.BridgeRegistrationPoint.TransactionHash)]
	receipt.Logs[2].Topics[3] = common.Hash{0xff}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
	)
	if err == nil || !strings.Contains(err.Error(), "Bridge registration log") {
		t.Fatalf("expected wrong receipt-log rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsManifestCodeDrift(
	t *testing.T,
) {
	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.verifier.code[common.Address(fixture.profile.BridgeAddress)] = []byte{0xff}
	_, err := fixture.source.ReadCompleteHistory(
		context.Background(),
		fixture.from,
		fixture.to,
	)
	if err == nil || !strings.Contains(err.Error(), "signed activation manifest") {
		t.Fatalf("expected manifest code-drift rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongEndpointAndReorg(
	t *testing.T,
) {
	_, _, _, _, _, err := validateFrostRetainedGroupSourceConfig(
		FrostRetainedGroupHistorySourceConfig{
			ExportURL:            "https://export.example/history",
			EthereumURL:          "https://primary.example/independent",
			TrustDomainID:        "independent",
			TrustedSignerKeyHash: frostActivationHex32([32]byte{0x01}),
		},
		"wss://primary.example/rpc",
	)
	if err == nil || !strings.Contains(err.Error(), "not independent") {
		t.Fatalf("expected shared endpoint rejection, got [%v]", err)
	}
	_, _, _, _, _, err = validateFrostRetainedGroupSourceConfig(
		FrostRetainedGroupHistorySourceConfig{
			ExportURL:            "https://history.example/export",
			EthereumURL:          "https://history.example/rpc",
			TrustDomainID:        "independent",
			TrustedSignerKeyHash: frostActivationHex32([32]byte{0x01}),
		},
		"wss://primary.example/rpc",
	)
	if err == nil || !strings.Contains(err.Error(), "exporter and Ethereum verifier") {
		t.Fatalf("expected exporter/verifier separation rejection, got [%v]", err)
	}

	fixture := newFrostRetainedGroupHistorySourceFixture(t)
	fixture.verifier.reorgBlock = fixture.to.BlockNumber
	fixture.verifier.reorgAfterRead = 2
	fixture.verifier.reorgHeader = &types.Header{
		Number: new(big.Int).SetUint64(fixture.to.BlockNumber),
		Time:   999,
		Extra:  []byte{0xff},
	}
	_, err = fixture.source.ReadCompleteHistory(context.Background(), fixture.from, fixture.to)
	if err == nil || !strings.Contains(err.Error(), "canonical chain") {
		t.Fatalf("expected finalized-chain reorg rejection, got [%v]", err)
	}
}

func TestSignedFrostRetainedGroupHistorySource_RejectsWrongOperatorReceipt(
	t *testing.T,
) {
	t.Run("wrong point", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.operatorMutator = func(payload *frostRetainedGroupOperatorReceiptPayload) {
			payload.At.BlockHash = frostActivationHex32([32]byte{0x99})
		}
		_, err := fixture.source.ResolveOperatorID(
			context.Background(),
			chain.Address("0x1111111111111111111111111111111111111111"),
			fixture.to,
		)
		if err == nil || !strings.Contains(err.Error(), "differently bound") {
			t.Fatalf("expected operator receipt rejection, got [%v]", err)
		}
	})

	t.Run("wrong ID", func(t *testing.T) {
		fixture := newFrostRetainedGroupHistorySourceFixture(t)
		fixture.operatorMutator = func(payload *frostRetainedGroupOperatorReceiptPayload) {
			payload.OperatorID++
		}
		_, err := fixture.source.ResolveOperatorID(
			context.Background(),
			chain.Address("0x1111111111111111111111111111111111111111"),
			fixture.to,
		)
		if err == nil || !strings.Contains(err.Error(), "disagrees with exact finalized") {
			t.Fatalf("expected independent operator-ID rejection, got [%v]", err)
		}
	})
}
