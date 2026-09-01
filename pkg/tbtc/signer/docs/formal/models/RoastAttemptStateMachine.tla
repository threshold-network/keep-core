------------------------------ MODULE RoastAttemptStateMachine ------------------------------
EXTENDS FiniteSets, Naturals, Sequences, TLC

Participants == {1, 2, 3, 4}
Threshold == 2
MaxAttempt == 6

VARIABLES
    attemptNumber,
    lastAttemptNumber,
    activeParticipants,
    coordinator,
    consumedAttemptIds,
    durableConsumedAttemptIds

vars ==
    <<attemptNumber, lastAttemptNumber, activeParticipants, coordinator, consumedAttemptIds, durableConsumedAttemptIds>>

SetMin(S) ==
    CHOOSE x \in S: \A y \in S: x <= y

AttemptId(attempt, coordinatorIdentifier, includedParticipants) ==
    <<attempt, coordinatorIdentifier, includedParticipants>>

CurrentAttemptId ==
    AttemptId(attemptNumber, coordinator, activeParticipants)

CanAdvance(excluded) ==
    /\ attemptNumber < MaxAttempt
    /\ excluded \subseteq activeParticipants
    /\ activeParticipants \ excluded # {}
    /\ Cardinality(activeParticipants \ excluded) >= Threshold

Init ==
    /\ attemptNumber = 1
    /\ lastAttemptNumber = 1
    /\ activeParticipants = Participants
    /\ coordinator = SetMin(Participants)
    /\ consumedAttemptIds = {}
    /\ durableConsumedAttemptIds = {}

Advance(excluded) ==
    /\ CanAdvance(excluded)
    /\ attemptNumber' = attemptNumber + 1
    /\ lastAttemptNumber' = attemptNumber
    /\ activeParticipants' = activeParticipants \ excluded
    /\ coordinator' = SetMin(activeParticipants')
    /\ consumedAttemptIds' = consumedAttemptIds \cup {CurrentAttemptId}
    /\ durableConsumedAttemptIds' = durableConsumedAttemptIds \cup {CurrentAttemptId}

RestartReload ==
    /\ attemptNumber' = attemptNumber
    /\ lastAttemptNumber' = lastAttemptNumber
    /\ activeParticipants' = activeParticipants
    /\ coordinator' = coordinator
    /\ consumedAttemptIds' = durableConsumedAttemptIds
    /\ durableConsumedAttemptIds' = durableConsumedAttemptIds

CacheLoss ==
    /\ attemptNumber' = attemptNumber
    /\ lastAttemptNumber' = lastAttemptNumber
    /\ activeParticipants' = activeParticipants
    /\ coordinator' = coordinator
    /\ consumedAttemptIds' = {}
    /\ durableConsumedAttemptIds' = durableConsumedAttemptIds

Stay ==
    /\ attemptNumber' = attemptNumber
    /\ lastAttemptNumber' = lastAttemptNumber
    /\ activeParticipants' = activeParticipants
    /\ coordinator' = coordinator
    /\ consumedAttemptIds' = consumedAttemptIds
    /\ durableConsumedAttemptIds' = durableConsumedAttemptIds

Next ==
    \/ \E excluded \in SUBSET activeParticipants: Advance(excluded)
    \/ RestartReload
    \/ CacheLoss
    \/ Stay

Spec ==
    Init /\ [][Next]_vars

ConsumedIdWellFormed(id) ==
    /\ Len(id) = 3
    /\ id[1] \in 1..MaxAttempt
    /\ id[2] \in Participants
    /\ id[3] \subseteq Participants

TypeOK ==
    /\ attemptNumber \in 1..MaxAttempt
    /\ lastAttemptNumber \in 1..MaxAttempt
    /\ lastAttemptNumber <= attemptNumber
    /\ activeParticipants \subseteq Participants
    /\ activeParticipants # {}
    /\ Cardinality(activeParticipants) >= Threshold
    /\ coordinator \in activeParticipants
    /\ \A id \in consumedAttemptIds: ConsumedIdWellFormed(id)
    /\ \A id \in durableConsumedAttemptIds: ConsumedIdWellFormed(id)
    /\ consumedAttemptIds \subseteq durableConsumedAttemptIds

MonotonicAttemptNumber ==
    attemptNumber >= lastAttemptNumber

ReplaySafe ==
    /\ CurrentAttemptId \notin durableConsumedAttemptIds
    /\ \A id \in durableConsumedAttemptIds: id[1] < attemptNumber

ConsumedRegistryIsDurableSuperset ==
    consumedAttemptIds \subseteq durableConsumedAttemptIds

=============================================================================
