------------------------------ MODULE TeeEnforcementModes ------------------------------
EXTENDS TLC

\* STATUS: models a PLANNED enforcement profile, not yet implemented in the
\* shipped signer. The crate implements only a binary provenance enforce gate
\* (src/engine/provenance.rs); the three modes below, the
\* disabled->enforce transition guard, and break-glass are design targets per
\* docs/tee-whitelisted-signer-enforcement-plan.md (an explicitly "not active"
\* future hardening profile). A passing TLC run verifies this model's internal
\* consistency, NOT that the shipped signer enforces this behavior.

Modes == {"disabled", "audit", "enforce"}
AttestationStates == {"valid", "invalid", "missing"}

VARIABLES
    mode,
    previousMode,
    attestation,
    breakGlassActive,
    lastAdmission

vars ==
    <<mode, previousMode, attestation, breakGlassActive, lastAdmission>>

AdmissionDecision(enforcementMode, attestationState, breakGlass) ==
    IF /\ enforcementMode = "enforce"
       /\ ~breakGlass
       /\ attestationState # "valid"
    THEN "deny"
    ELSE "allow"

AllowedModeTransition(from, to) ==
    \/ /\ from = "disabled" /\ to \in {"disabled", "audit"}
    \/ /\ from = "audit" /\ to \in {"disabled", "audit", "enforce"}
    \/ /\ from = "enforce" /\ to \in {"audit", "enforce"}

Init ==
    /\ mode = "disabled"
    /\ previousMode = "disabled"
    /\ attestation = "missing"
    /\ breakGlassActive = FALSE
    /\ lastAdmission = AdmissionDecision(mode, attestation, breakGlassActive)

SetMode(newMode) ==
    /\ newMode \in Modes
    /\ AllowedModeTransition(mode, newMode)
    /\ previousMode' = mode
    /\ mode' = newMode
    /\ attestation' = attestation
    /\ breakGlassActive' = breakGlassActive
    /\ lastAdmission' = AdmissionDecision(mode', attestation', breakGlassActive')

SetAttestation(newAttestation) ==
    /\ newAttestation \in AttestationStates
    /\ attestation' = newAttestation
    /\ UNCHANGED <<mode, previousMode, breakGlassActive>>
    /\ lastAdmission' = AdmissionDecision(mode, attestation', breakGlassActive)

SetBreakGlass(newBreakGlass) ==
    /\ newBreakGlass \in BOOLEAN
    /\ breakGlassActive' = newBreakGlass
    /\ UNCHANGED <<mode, previousMode, attestation>>
    /\ lastAdmission' = AdmissionDecision(mode, attestation, breakGlassActive')

ReevaluateAdmission ==
    /\ lastAdmission' = AdmissionDecision(mode, attestation, breakGlassActive)
    /\ UNCHANGED <<mode, previousMode, attestation, breakGlassActive>>

Next ==
    \/ \E newMode \in Modes: SetMode(newMode)
    \/ \E newAttestation \in AttestationStates: SetAttestation(newAttestation)
    \/ \E newBreakGlass \in BOOLEAN: SetBreakGlass(newBreakGlass)
    \/ ReevaluateAdmission

Spec ==
    Init /\ [][Next]_vars

TypeOK ==
    /\ mode \in Modes
    /\ previousMode \in Modes
    /\ attestation \in AttestationStates
    /\ breakGlassActive \in BOOLEAN
    /\ lastAdmission \in {"allow", "deny"}

EnforceModeRequiresValidAttestationWithoutOverride ==
    (/\ mode = "enforce"
     /\ ~breakGlassActive
     /\ lastAdmission = "allow")
        => attestation = "valid"

NoDirectDisabledToEnforceTransition ==
    ~(previousMode = "disabled" /\ mode = "enforce")

AdmissionDecisionIsStable ==
    lastAdmission = AdmissionDecision(mode, attestation, breakGlassActive)

=============================================================================
