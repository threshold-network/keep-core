------------------------------ MODULE StateKeyProviderPolicy ------------------------------
EXTENDS TLC

Profiles == {"development", "production"}
Providers == {"env", "command", "kms", "hsm"}
KeyIds == {"kid-a", "kid-b", "kid-c"}

CONSTANT EnforceProductionProfileGate

SupportedProviders(profile) ==
    IF /\ EnforceProductionProfileGate
       /\ profile = "production"
    THEN {"command"}
    ELSE {"env"}

VARIABLES
    profile,
    runtimeProvider,
    runtimeKeyId,
    envelopeProvider,
    envelopeKeyId,
    requestedProvider,
    requestedKeyId,
    loadOutcome

vars ==
    <<profile, runtimeProvider, runtimeKeyId, envelopeProvider, envelopeKeyId, requestedProvider, requestedKeyId, loadOutcome>>

LoadSucceeds(profileValue, provider, keyId) ==
    /\ provider = envelopeProvider
    /\ keyId = envelopeKeyId
    /\ provider \in SupportedProviders(profileValue)

Init ==
    /\ profile = "development"
    /\ runtimeProvider = "env"
    /\ runtimeKeyId = "kid-a"
    /\ envelopeProvider = runtimeProvider
    /\ envelopeKeyId = runtimeKeyId
    /\ requestedProvider = runtimeProvider
    /\ requestedKeyId = runtimeKeyId
    /\ loadOutcome = "ok"

SetProfile(newProfile) ==
    /\ newProfile \in Profiles
    /\ profile' = newProfile
    /\ loadOutcome' = IF LoadSucceeds(newProfile, requestedProvider, requestedKeyId) THEN "ok" ELSE "reject"
    /\ UNCHANGED <<runtimeProvider, runtimeKeyId, envelopeProvider, envelopeKeyId, requestedProvider, requestedKeyId>>

SetRuntime(provider, keyId) ==
    /\ provider \in Providers
    /\ keyId \in KeyIds
    /\ runtimeProvider' = provider
    /\ runtimeKeyId' = keyId
    /\ UNCHANGED <<profile, envelopeProvider, envelopeKeyId, requestedProvider, requestedKeyId, loadOutcome>>

Persist ==
    /\ runtimeProvider \in SupportedProviders(profile)
    /\ envelopeProvider' = runtimeProvider
    /\ envelopeKeyId' = runtimeKeyId
    /\ loadOutcome' = IF /\ requestedProvider = runtimeProvider
                         /\ requestedKeyId = runtimeKeyId
                         /\ requestedProvider \in SupportedProviders(profile)
                      THEN "ok"
                      ELSE "reject"
    /\ UNCHANGED <<profile, runtimeProvider, runtimeKeyId, requestedProvider, requestedKeyId>>

AttemptLoad(provider, keyId) ==
    /\ provider \in Providers
    /\ keyId \in KeyIds
    /\ requestedProvider' = provider
    /\ requestedKeyId' = keyId
    /\ loadOutcome' = IF LoadSucceeds(profile, provider, keyId) THEN "ok" ELSE "reject"
    /\ UNCHANGED <<profile, runtimeProvider, runtimeKeyId, envelopeProvider, envelopeKeyId>>

Next ==
    \/ \E newProfile \in Profiles: SetProfile(newProfile)
    \/ \E provider \in Providers: \E keyId \in KeyIds: SetRuntime(provider, keyId)
    \/ Persist
    \/ \E provider \in Providers: \E keyId \in KeyIds: AttemptLoad(provider, keyId)

Spec ==
    Init /\ [][Next]_vars

TypeOK ==
    /\ profile \in Profiles
    /\ runtimeProvider \in Providers
    /\ runtimeKeyId \in KeyIds
    /\ envelopeProvider \in Providers
    /\ envelopeKeyId \in KeyIds
    /\ requestedProvider \in Providers
    /\ requestedKeyId \in KeyIds
    /\ loadOutcome \in {"ok", "reject"}

LoadSuccessImpliesExactBinding ==
    loadOutcome = "ok" =>
        /\ requestedProvider = envelopeProvider
        /\ requestedKeyId = envelopeKeyId
        /\ requestedProvider \in SupportedProviders(profile)

FailClosedDisallowedProvider ==
    requestedProvider \notin SupportedProviders(profile) => loadOutcome = "reject"

PersistedProviderCompliesWithPolicy ==
    loadOutcome = "ok" => envelopeProvider \in SupportedProviders(profile)

ProductionGateRejectsEnv ==
    /\ EnforceProductionProfileGate
    /\ profile = "production"
    /\ requestedProvider = "env"
    => loadOutcome = "reject"

=============================================================================
