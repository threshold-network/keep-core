// SPDX-License-Identifier: GPL-3.0-only
//
// ▓▓▌ ▓▓ ▐▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▄
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▌▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓    ▓▓▓▓▓▓▓▀    ▐▓▓▓▓▓▓    ▐▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▄▄▓▓▓▓▓▓▓▀      ▐▓▓▓▓▓▓▄▄▄▄         ▓▓▓▓▓▓▄▄▄▄         ▐▓▓▓▓▓▌   ▐▓▓▓▓▓▓
//   ▓▓▓▓▓▓▓▓▓▓▓▓▓▀        ▐▓▓▓▓▓▓▓▓▓▓         ▓▓▓▓▓▓▓▓▓▓         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
//   ▓▓▓▓▓▓▀▀▓▓▓▓▓▓▄       ▐▓▓▓▓▓▓▀▀▀▀         ▓▓▓▓▓▓▀▀▀▀         ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▀
//   ▓▓▓▓▓▓   ▀▓▓▓▓▓▓▄     ▐▓▓▓▓▓▓     ▓▓▓▓▓   ▓▓▓▓▓▓     ▓▓▓▓▓   ▐▓▓▓▓▓▌
// ▓▓▓▓▓▓▓▓▓▓ █▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
// ▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓ ▐▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓
//
//

// Initial version copied from Keep Network Random Beacon:
// https://github.com/keep-network/keep-core/blob/5138c7628868dbeed3ae2164f76fccc6c1fbb9e8/solidity/random-beacon/contracts/libraries/DKG.sol
//
// With the following differences:
// - the group size was set to 100,
// - offchainDkgTimeout was removed,
// - submission eligibility verification is not performed on-chain,
// - submission eligibility delay was replaced with a submission timeout,
// - seed timeout notification requires seedTimeout period to pass.

pragma solidity 0.8.17;

import "@openzeppelin/contracts-upgradeable/utils/cryptography/ECDSAUpgradeable.sol";
import "@keep-network/sortition-pools/contracts/SortitionPool.sol";
import "@keep-network/random-beacon/contracts/libraries/BytesLib.sol";
import "../EcdsaDkgValidator.sol";

library EcdsaDkg {
    using BytesLib for bytes;
    using ECDSAUpgradeable for bytes32;

    struct Parameters {
        // Time in blocks during which a seed is expected to be delivered.
        // DKG starts only after a seed is delivered. The time the contract
        // awaits for a seed is not included in the DKG timeout.
        uint256 seedTimeout;
        // Time in blocks during which a submitted result can be challenged.
        uint256 resultChallengePeriodLength;
        // Extra gas required to be left at the end of the challenge DKG result
        // transaction.
        uint256 resultChallengeExtraGas;
        // Time in blocks during which a result is expected to be submitted.
        uint256 resultSubmissionTimeout;
        // Time in blocks during which only the result submitter is allowed to
        // approve it. Once this period ends and the submitter have not approved
        // the result, anyone can do it.
        uint256 submitterPrecedencePeriodLength;
        // This struct doesn't contain `__gap` property as the structure is
        // stored inside `Data` struct, that already have a gap that can be used
        // on upgrade.
    }

    struct Data {
        // Address of the Sortition Pool contract.
        SortitionPool sortitionPool;
        // Address of the EcdsaDkgValidator contract.
        EcdsaDkgValidator dkgValidator;
        // DKG parameters. The parameters should persist between DKG executions.
        // They should be updated with dedicated set functions only when DKG is not
        // in progress.
        Parameters parameters;
        // Time in block at which DKG state was locked.
        uint256 stateLockBlock;
        // Time in blocks at which DKG started.
        uint256 startBlock;
        // Seed used to start DKG.
        uint256 seed;
        // Time in blocks that should be added to result submission eligibility
        // delay calculation. It is used in case of a challenge to adjust
        // DKG timeout calculation.
        uint256 resultSubmissionStartBlockOffset;
        // Hash of submitted DKG result.
        bytes32 submittedResultHash;
        // Block number from the moment of the DKG result submission.
        uint256 submittedResultBlock;
        // Reserved storage space in case we need to add more variables.
        // See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
        // slither-disable-next-line unused-state
        uint256[38] __gap;
    }

    /// @notice DKG result.
    struct Result {
        // Claimed submitter candidate group member index.
        // Must be in range [1, groupSize].
        uint256 submitterMemberIndex;
        // Generated candidate group public key
        bytes groupPubKey;
        // Array of misbehaved members indices (disqualified or inactive).
        // Indices must be in range [1, groupSize], unique, and sorted in ascending
        // order.
        uint8[] misbehavedMembersIndices;
        // Concatenation of signatures from members supporting the result.
        // The message to be signed by each member is keccak256 hash of the
        // calculated group public key, misbehaved members indices and DKG
        // start block. The calculated hash should be prefixed with prefixed with
        // `\x19Ethereum signed message:\n` before signing, so the message to
        // sign is:
        // `\x19Ethereum signed message:\n${keccak256(
        //    groupPubKey, misbehavedMembersIndices, dkgStartBlock
        // )}`
        bytes signatures;
        // Indices of members corresponding to each signature. Indices must be
        // be in range [1, groupSize], unique, and sorted in ascending order.
        uint256[] signingMembersIndices;
        // Identifiers of candidate group members as outputted by the group
        // selection protocol.
        uint32[] members;
        // Keccak256 hash of group members identifiers that actively took part
        // in DKG (excluding IA/DQ members).
        bytes32 membersHash;
        // This struct doesn't contain `__gap` property as the structure is not
        // stored, it is used as a function's calldata argument.
    }

    /// @notice States for phases of group creation. The states doesn't include
    ///         timeouts which should be tracked and notified individually.
    enum State {
        // Group creation is not in progress. It is a state set after group creation
        // completion either by timeout or by a result approval.
        IDLE,
        // Group creation is awaiting the seed and sortition pool is locked.
        AWAITING_SEED,
        // DKG protocol execution is in progress. A result is being calculated
        // by the clients in this state and the contract awaits a result submission.
        // This is a state to which group creation returns in case of a result
        // challenge notification.
        AWAITING_RESULT,
        // DKG result was submitted and awaits an approval or a challenge. If a result
        // gets challenge the state returns to `AWAITING_RESULT`. If a result gets
        // approval the state changes to `IDLE`.
        CHALLENGE
    }

    /// @dev Size of a group in ECDSA wallet.
    uint256 public constant groupSize = 100;

    event DkgStarted(uint256 indexed seed);

    // To recreate the members that actively took part in dkg, the selected members
    // array should be filtered out from misbehavedMembersIndices.
    event DkgResultSubmitted(
        bytes32 indexed resultHash,
        uint256 indexed seed,
        Result result
    );

    event DkgTimedOut();

    event DkgResultApproved(
        bytes32 indexed resultHash,
        address indexed approver
    );

    event DkgResultChallenged(
        bytes32 indexed resultHash,
        address indexed challenger,
        string reason
    );

    event DkgStateLocked();

    event DkgSeedTimedOut();

    /// @notice Initializes SortitionPool and EcdsaDkgValidator addresses.
    ///        Can be performed only once.
    /// @param _sortitionPool Sortition Pool reference
    /// @param _dkgValidator EcdsaDkgValidator reference
    function init(
        Data storage self,
        SortitionPool _sortitionPool,
        EcdsaDkgValidator _dkgValidator
    ) internal {
        require(
            address(self.sortitionPool) == address(0),
            "Sortition Pool address already set"
        );

        require(
            address(self.dkgValidator) == address(0),
            "DKG Validator address already set"
        );

        self.sortitionPool = _sortitionPool;
        self.dkgValidator = _dkgValidator;
    }

    /// @notice Determines the current state of group creation. It doesn't take
    ///         timeouts into consideration. The timeouts should be tracked and
    ///         notified separately.
    function currentState(Data storage self)
        internal
        view
        returns (State state)
    {
        state = State.IDLE;

        if (self.sortitionPool.isLocked()) {
            state = State.AWAITING_SEED;

            if (self.startBlock > 0) {
                state = State.AWAITING_RESULT;

                if (self.submittedResultBlock > 0) {
                    state = State.CHALLENGE;
                }
            }
        }
    }

    /// @notice Locks the sortition pool and starts awaiting for the
    ///         group creation seed.
    function lockState(Data storage self) internal {
        require(currentState(self) == State.IDLE, "Current state is not IDLE");

        emit DkgStateLocked();

        self.sortitionPool.lock();

        self.stateLockBlock = block.number;
    }

    function start(Data storage self, uint256 seed) internal {
        require(
            currentState(self) == State.AWAITING_SEED,
            "Current state is not AWAITING_SEED"
        );

        emit DkgStarted(seed);

        self.startBlock = block.number;
        self.seed = seed;
    }

    /// @notice Allows to submit a DKG result. The submitted result does not go
    ///         through a validation and before it gets accepted, it needs to
    ///         wait through the challenge period during which everyone has
    ///         a chance to challenge the result as invalid one. Submitter of
    ///         the result needs to be in the sortition pool and if the result
    ///         gets challenged, the submitter will get slashed.
    function submitResult(Data storage self, Result calldata result) internal {
        require(
            currentState(self) == State.AWAITING_RESULT,
            "Current state is not AWAITING_RESULT"
        );
        require(!hasDkgTimedOut(self), "DKG timeout already passed");

        SortitionPool sortitionPool = self.sortitionPool;

        // Submitter must be an operator in the sortition pool.
        // Declared submitter's member index in the DKG result needs to match
        // the address calling this function.
        require(
            sortitionPool.isOperatorInPool(msg.sender),
            "Submitter not in the sortition pool"
        );
        require(
            sortitionPool.getIDOperator(
                result.members[result.submitterMemberIndex - 1]
            ) == msg.sender,
            "Unexpected submitter index"
        );

        self.submittedResultHash = keccak256(abi.encode(result));
        self.submittedResultBlock = block.number;

        emit DkgResultSubmitted(self.submittedResultHash, self.seed, result);
    }

    /// @notice Checks if awaiting seed timed out.
    /// @return True if awaiting seed timed out, false otherwise.
    function hasSeedTimedOut(Data storage self) internal view returns (bool) {
        return
            currentState(self) == State.AWAITING_SEED &&
            block.number > (self.stateLockBlock + self.parameters.seedTimeout);
    }

    /// @notice Checks if DKG timed out. The DKG timeout period includes time required
    ///         for off-chain protocol execution and time for the result publication.
    ///         After this time a result cannot be submitted and DKG can be notified
    ///         about the timeout. DKG period is adjusted by result submission
    ///         offset that include blocks that were mined while invalid result
    ///         has been registered until it got challenged.
    /// @return True if DKG timed out, false otherwise.
    function hasDkgTimedOut(Data storage self) internal view returns (bool) {
        return
            currentState(self) == State.AWAITING_RESULT &&
            block.number >
            (self.startBlock +
                self.resultSubmissionStartBlockOffset +
                self.parameters.resultSubmissionTimeout);
    }

    /// @notice Notifies about the seed was not delivered and restores the
    ///         initial DKG state (IDLE).
    function notifySeedTimeout(Data storage self) internal {
        require(hasSeedTimedOut(self), "Awaiting seed has not timed out");

        emit DkgSeedTimedOut();

        complete(self);
    }

    /// @notice Notifies about DKG timeout.
    function notifyDkgTimeout(Data storage self) internal {
        require(hasDkgTimedOut(self), "DKG has not timed out");

        emit DkgTimedOut();

        complete(self);
    }

    /// @notice Approves DKG result. Can be called when the challenge period for
    ///         the submitted result is finished. Considers the submitted result
    ///         as valid. For the first `submitterPrecedencePeriodLength`
    ///         blocks after the end of the challenge period can be called only
    ///         by the DKG result submitter. After that time, can be called by
    ///         anyone.
    /// @dev Can be called after a challenge period for the submitted result.
    /// @param result Result to approve. Must match the submitted result stored
    ///        during `submitResult`.
    /// @return misbehavedMembers Identifiers of members who misbehaved during DKG.
    function approveResult(Data storage self, Result calldata result)
        internal
        returns (uint32[] memory misbehavedMembers)
    {
        require(
            currentState(self) == State.CHALLENGE,
            "Current state is not CHALLENGE"
        );

        uint256 challengePeriodEnd = self.submittedResultBlock +
            self.parameters.resultChallengePeriodLength;

        require(
            block.number > challengePeriodEnd,
            "Challenge period has not passed yet"
        );

        require(
            keccak256(abi.encode(result)) == self.submittedResultHash,
            "Result under approval is different than the submitted one"
        );

        // Extract submitter member address. Submitter member index is in
        // range [1, groupSize] so we need to -1 when fetching identifier from members
        // array.
        address submitterMember = self.sortitionPool.getIDOperator(
            result.members[result.submitterMemberIndex - 1]
        );

        require(
            msg.sender == submitterMember ||
                block.number >
                challengePeriodEnd +
                    self.parameters.submitterPrecedencePeriodLength,
            "Only submitter can approve now"
        );

        // Extract misbehaved members identifiers. Misbehaved members indices
        // are in range [1, groupSize], so we need to -1 when fetching identifiers from
        // members array.
        misbehavedMembers = new uint32[](
            result.misbehavedMembersIndices.length
        );
        for (uint256 i = 0; i < result.misbehavedMembersIndices.length; i++) {
            misbehavedMembers[i] = result.members[
                result.misbehavedMembersIndices[i] - 1
            ];
        }

        emit DkgResultApproved(self.submittedResultHash, msg.sender);

        return misbehavedMembers;
    }

    /// @notice Challenges DKG result. If the submitted result is proved to be
    ///         invalid it reverts the DKG back to the result submission phase.
    /// @dev Can be called during a challenge period for the submitted result.
    /// @param result Result to challenge. Must match the submitted result
    ///        stored during `submitResult`.
    /// @return maliciousResultHash Hash of the malicious result.
    /// @return maliciousSubmitter Identifier of the malicious submitter.
    function challengeResult(Data storage self, Result calldata result)
        internal
        returns (bytes32 maliciousResultHash, uint32 maliciousSubmitter)
    {
        require(
            currentState(self) == State.CHALLENGE,
            "Current state is not CHALLENGE"
        );

        require(
            block.number <=
                self.submittedResultBlock +
                    self.parameters.resultChallengePeriodLength,
            "Challenge period has already passed"
        );

        require(
            keccak256(abi.encode(result)) == self.submittedResultHash,
            "Result under challenge is different than the submitted one"
        );

        // https://github.com/crytic/slither/issues/982
        // slither-disable-next-line unused-return
        try
            self.dkgValidator.validate(result, self.seed, self.startBlock)
        returns (
            // slither-disable-next-line uninitialized-local,variable-scope
            bool isValid,
            // slither-disable-next-line uninitialized-local,variable-scope
            string memory errorMsg
        ) {
            if (isValid) {
                revert("unjustified challenge");
            }

            emit DkgResultChallenged(
                self.submittedResultHash,
                msg.sender,
                errorMsg
            );
        } catch {
            // if the validation reverted we consider the DKG result as invalid
            emit DkgResultChallenged(
                self.submittedResultHash,
                msg.sender,
                "validation reverted"
            );
        }

        // Consider result hash as malicious.
        maliciousResultHash = self.submittedResultHash;
        maliciousSubmitter = result.members[result.submitterMemberIndex - 1];

        // Adjust DKG result submission block start, so submission stage starts
        // from the beginning.
        self.resultSubmissionStartBlockOffset = block.number - self.startBlock;

        submittedResultCleanup(self);

        return (maliciousResultHash, maliciousSubmitter);
    }

    /// @notice Checks if DKG result is valid for the current DKG.
    /// @param result DKG result.
    /// @return True if the result is valid. If the result is invalid it returns
    ///         false and an error message.
    function isResultValid(Data storage self, Result calldata result)
        internal
        view
        returns (bool, string memory)
    {
        require(self.startBlock > 0, "DKG has not been started");

        return self.dkgValidator.validate(result, self.seed, self.startBlock);
    }

    /// @notice Set setSeedTimeout parameter.
    /// @dev State validation is performed by the caller to reduce bytecode size
    ///      by eliminating redundant checks across multiple setter functions.
    function setSeedTimeout(Data storage self, uint256 newSeedTimeout)
        internal
    {
        require(newSeedTimeout > 0, "Value must be greater than zero");

        self.parameters.seedTimeout = newSeedTimeout;
    }

    /// @notice Set resultChallengePeriodLength parameter.
    /// @dev State validation is performed by the caller to reduce bytecode size
    ///      by eliminating redundant checks across multiple setter functions.
    function setResultChallengePeriodLength(
        Data storage self,
        uint256 newResultChallengePeriodLength
    ) internal {
        require(
            newResultChallengePeriodLength > 0,
            "Value must be greater than zero"
        );

        self
            .parameters
            .resultChallengePeriodLength = newResultChallengePeriodLength;
    }

    /// @notice Set resultChallengeExtraGas parameter.
    /// @dev State validation is performed by the caller to reduce bytecode size
    ///      by eliminating redundant checks across multiple setter functions.
    function setResultChallengeExtraGas(
        Data storage self,
        uint256 newResultChallengeExtraGas
    ) internal {
        self.parameters.resultChallengeExtraGas = newResultChallengeExtraGas;
    }

    /// @notice Set resultSubmissionTimeout parameter.
    /// @dev State validation is performed by the caller to reduce bytecode size
    ///      by eliminating redundant checks across multiple setter functions.
    function setResultSubmissionTimeout(
        Data storage self,
        uint256 newResultSubmissionTimeout
    ) internal {
        require(
            newResultSubmissionTimeout > 0,
            "Value must be greater than zero"
        );

        self.parameters.resultSubmissionTimeout = newResultSubmissionTimeout;
    }

    /// @notice Set submitterPrecedencePeriodLength parameter.
    /// @dev This setter retains its state validation check because it requires
    ///      additional validation logic that compares against resultSubmissionTimeout.
    function setSubmitterPrecedencePeriodLength(
        Data storage self,
        uint256 newSubmitterPrecedencePeriodLength
    ) internal {
        require(currentState(self) == State.IDLE, "Current state is not IDLE");

        require(
            newSubmitterPrecedencePeriodLength <
                self.parameters.resultSubmissionTimeout,
            "Value exceeds result submission timeout"
        );

        self
            .parameters
            .submitterPrecedencePeriodLength = newSubmitterPrecedencePeriodLength;
    }

    /// @notice Completes DKG by cleaning up state.
    /// @dev Should be called after DKG times out or a result is approved.
    function complete(Data storage self) internal {
        delete self.startBlock;
        delete self.seed;
        delete self.resultSubmissionStartBlockOffset;
        submittedResultCleanup(self);
        self.sortitionPool.unlock();
    }

    /// @notice Cleans up submitted result state either after DKG completion
    ///         (as part of `complete` method) or after justified challenge.
    function submittedResultCleanup(Data storage self) private {
        delete self.submittedResultHash;
        delete self.submittedResultBlock;
    }
}
