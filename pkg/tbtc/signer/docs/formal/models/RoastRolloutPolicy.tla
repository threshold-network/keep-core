----------------------------- MODULE RoastRolloutPolicy -----------------------------
EXTENDS TLC

Stages == {"bootstrap", "canary", "broad", "rollback", "halted"}

VARIABLES stage, canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop

vars == <<stage, canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

Init ==
    /\ stage = "bootstrap"
    /\ canaryCompleted = FALSE
    /\ holdTrigger \in BOOLEAN
    /\ rollbackTrigger \in BOOLEAN
    /\ manualOverride \in BOOLEAN
    /\ emergencyStop \in BOOLEAN

UpdateSignals ==
    /\ holdTrigger' \in BOOLEAN
    /\ rollbackTrigger' \in BOOLEAN
    /\ manualOverride' \in BOOLEAN
    /\ emergencyStop' \in BOOLEAN
    /\ UNCHANGED <<stage, canaryCompleted>>

StartCanary ==
    /\ stage = "bootstrap"
    /\ ~emergencyStop
    /\ stage' = "canary"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

PromoteCanaryToBroad ==
    /\ stage = "canary"
    /\ ~holdTrigger
    /\ ~rollbackTrigger
    /\ ~emergencyStop
    /\ stage' = "broad"
    /\ canaryCompleted' = TRUE
    /\ UNCHANGED <<holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

HoldCanary ==
    /\ stage = "canary"
    /\ holdTrigger
    /\ UNCHANGED vars

RollbackFromCanary ==
    /\ stage = "canary"
    /\ rollbackTrigger
    /\ stage' = "rollback"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

RollbackFromBroad ==
    /\ stage = "broad"
    /\ rollbackTrigger
    /\ stage' = "rollback"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

RecoverRollbackToCanary ==
    /\ stage = "rollback"
    /\ manualOverride
    /\ ~rollbackTrigger
    /\ ~emergencyStop
    /\ stage' = "canary"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

EmergencyHalt ==
    /\ emergencyStop
    /\ stage' = "halted"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

StayHalted ==
    /\ stage = "halted"
    /\ stage' = "halted"
    /\ UNCHANGED <<canaryCompleted, holdTrigger, rollbackTrigger, manualOverride, emergencyStop>>

NoOp ==
    /\ ~emergencyStop
    /\ UNCHANGED vars

Next ==
    \/ UpdateSignals
    \/ StartCanary
    \/ PromoteCanaryToBroad
    \/ HoldCanary
    \/ RollbackFromCanary
    \/ RollbackFromBroad
    \/ RecoverRollbackToCanary
    \/ EmergencyHalt
    \/ StayHalted
    \/ NoOp

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ stage \in Stages
    /\ canaryCompleted \in BOOLEAN
    /\ holdTrigger \in BOOLEAN
    /\ rollbackTrigger \in BOOLEAN
    /\ manualOverride \in BOOLEAN
    /\ emergencyStop \in BOOLEAN

BroadRequiresCanaryHistory == stage = "broad" => canaryCompleted

RollbackTransitionRequiresTrigger ==
    [][((stage # "rollback" /\ stage' = "rollback") =>
        /\ rollbackTrigger
        /\ stage \in {"canary", "broad"})]_vars

CanaryHoldBlocksPromotion ==
    [][((stage = "canary" /\ (holdTrigger \/ rollbackTrigger)) => stage' # "broad")]_vars

BootstrapCannotJumpToBroad == [][(stage = "bootstrap" => stage' # "broad")]_vars

EmergencyStopBlocksForwardProgress ==
    [][
        /\ ((emergencyStop /\ stage = "bootstrap") => stage' # "canary")
        /\ ((emergencyStop /\ stage = "canary") => stage' # "broad")
        /\ ((emergencyStop /\ stage = "rollback") => stage' # "canary")
      ]_vars

HaltedModeIsTerminal == [][(stage = "halted" => stage' = "halted")]_vars

=====================================================================================
