package firewall

import (
	"fmt"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"
	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/operator"
)

const cachingPeriod = time.Second

func TestValidate_PeerNotRecognized_NoApplications(t *testing.T) {
	policy := &anyApplicationPolicy{
		applications:        []Application{},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_PeerNotRecognized_MultipleApplications(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	policy := &anyApplicationPolicy{
		applications: []Application{
			newMockApplication(),
			newMockApplication()},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_PeerRecognized_FirstApplicationRecognizes(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications: []Application{
			application,
			newMockApplication()},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_PeerRecognized_SecondApplicationRecognizes(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications: []Application{
			newMockApplication(),
			application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_PeerNotRecognized_FirstApplicationReturnedError(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	// First application returns error during operator recognition check.
	// Even though the second application could recognize the operator, the
	// validation should fail, since an error occurred during checks.
	applicationError := fmt.Errorf("dummy error")
	application1 := newMockApplication()
	application1.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: false,
		err:          applicationError,
	})

	application2 := newMockApplication()
	application2.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications: []Application{
			application1,
			application2},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertAnyErrorInChainMatchesTarget(t, applicationError, err)
}

func TestValidate_PeerRecognized_Rechecked(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications:        []Application{application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the application does not recognize the operator anymore.
	// Validation should fail because positive results are rechecked to avoid
	// accepting peers whose application recognition was revoked.
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: false,
		err:          nil,
	})

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_PeerNotRecognized_CacheEmptied(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications:        []Application{application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the application does not recognize the operator anymore.
	// Wait for the caching period to end. Validation should fail, as the
	// operator has been removed from the cache.
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: false,
		err:          nil,
	})

	time.Sleep(cachingPeriod)

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_PeerNotRecognized_Cached(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	policy := &anyApplicationPolicy{
		applications:        []Application{application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)

	// Ensure the application recognizes the operator, but the validation should
	// fail since the result from the previous call has been cached.
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_PeerRecognized_CacheEmptied(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: false,
		err:          nil,
	})

	policy := &anyApplicationPolicy{
		applications:        []Application{application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)

	// Ensure the application recognizes the operator. Wait for the caching
	// period to elapse. The validation should pass since the result from the
	// previous call has been removed from the cache.
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	time.Sleep(cachingPeriod)

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_PeerIsAllowlistedNode(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	// This test validates that the AllowList type mechanism still works
	// correctly at the type level. In production, EmptyAllowList is used,
	// so no peer receives this bypass. See the EmptyAllowList tests below
	// for the production-relevant security behavior.
	allowList := NewAllowList([]*operator.PublicKey{peerOperatorPublicKey})

	policy := &anyApplicationPolicy{
		applications:        []Application{newMockApplication()},
		allowList:           allowList,
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_EmptyAllowList_RecognizedPeerAccepted(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	application := newMockApplication()
	application.setIsRecognized(peerOperatorPublicKey, result{
		isRecognized: true,
		err:          nil,
	})

	// With EmptyAllowList(), a recognized peer must pass validation through
	// the IsRecognized path, not through an AllowList bypass.
	policy := &anyApplicationPolicy{
		applications:        []Application{application},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidate_EmptyAllowList_UnrecognizedPeerRejected(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	// With EmptyAllowList(), a peer not recognized by any application must
	// be rejected. No AllowList bypass is available.
	policy := &anyApplicationPolicy{
		applications:        []Application{newMockApplication()},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func TestValidate_EmptyAllowList_PreviouslyAllowlistedPeerMustPassIsRecognized(t *testing.T) {
	_, peerOperatorPublicKey, err := operator.GenerateKeyPair(
		local_v1.DefaultCurve,
	)
	if err != nil {
		t.Fatal(err)
	}

	// This is the core security assertion: a peer that would have been on
	// a populated AllowList (e.g., a bootstrap node) is now subject to
	// standard IsRecognized staking checks when EmptyAllowList is used.
	// The peer is not recognized by the application and must be rejected.
	policy := &anyApplicationPolicy{
		applications:        []Application{newMockApplication()},
		allowList:           EmptyAllowList(),
		negativeResultCache: cache.NewTimeCache(cachingPeriod),
	}

	err = policy.Validate(peerOperatorPublicKey)
	testutils.AssertErrorsSame(t, errNotRecognized, err)
}

func newMockApplication() *mockApplication {
	return &mockApplication{
		results: make(map[*operator.PublicKey]result),
	}
}

type result struct {
	isRecognized bool
	err          error
}

type mockApplication struct {
	results map[*operator.PublicKey]result
}

func (ma *mockApplication) setIsRecognized(
	operatorPublicKey *operator.PublicKey,
	result result,
) {
	ma.results[operatorPublicKey] = result
}

func (ma *mockApplication) IsRecognized(operatorPublicKey *operator.PublicKey) (
	bool,
	error,
) {
	result := ma.results[operatorPublicKey]
	return result.isRecognized, result.err
}
