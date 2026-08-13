// SPDX-License-Identifier: GPL-3.0-only

pragma solidity ^0.8.0;

/// @notice Test-only programmable mock. Answers any call according to a table
///         set up from the test, and records the calls it receives.
///
///         This is the contract half of the replacement for the archived
///         defi-wonderland smock package. smock worked by reaching into
///         Hardhat's provider internals, which is why it broke on Hardhat
///         >= 2.20 and why it cannot be carried forward. Everything here is
///         ordinary EVM state, so it survives any Hardhat, ethers, viem or
///         Foundry change.
///
///         The model is deliberately Foundry's `vm.mockCall`: a
///         calldata-to-returndata table plus call recording. If the suite ever
///         moves to Solidity tests, the semantics carry over unchanged.
///
///         Lookup order for an incoming call, first match wins:
///           1. exact full-calldata match — smock's `whenCalledWith`
///           2. selector match — smock's bare `returns`
///           3. the response of last resort installed by the helper when the
///              mock was created: the zero value of the function's return
///              type, which is how smock answered an unstubbed function
///
///         Reverts are configured the same way and take priority over returns
///         at the same specificity.
///
/// @dev Administrative entry points are prefixed `__mock__` so they are
///      addressable alongside whatever interface is being mocked. A four-byte
///      collision between one of them and a real function of the mocked
///      interface would silently shadow that function, so the TypeScript helper
///      asserts no collision exists at construction time rather than leaving it
///      to chance.
contract MockContract {
    // The `__mock__` prefix namespaces this contract's own entry points away
    // from whatever interface it is answering, which is the point; mixedCase
    // would defeat it.
    // solhint-disable func-name-mixedcase

    enum Behaviour {
        Unset,
        Return,
        Revert
    }

    struct Response {
        Behaviour behaviour;
        bytes data;
    }

    /// @dev All mock state lives behind one hashed base slot.
    ///
    ///      Mocks are routinely installed over an address that already holds a
    ///      deployed contract — `test/fixtures/bridge.ts` pins them at the
    ///      Bridge's real `ecdsaWalletRegistry` and `relay` addresses, and 19
    ///      call sites pass an explicit address. `hardhat_setCode` replaces the
    ///      code but leaves that contract's storage behind, so state at slots
    ///      0, 1, 2... would be read back as configuration. An ERC-7201-style
    ///      base slot puts this contract's state where nothing else has been.
    struct State {
        /// @dev keccak256(full calldata) => response.
        mapping(bytes32 => Response) responseByCalldata;
        /// @dev selector => response.
        mapping(bytes4 => Response) responseBySelector;
        /// @dev selector => response of last resort, installed once when the
        ///      mock is created and never cleared by `reset`.
        ///
        ///      Solidity checks returndatasize against the size its ABI expects
        ///      and reverts on a short answer, so an unconfigured function
        ///      cannot simply return nothing: it has to return a correctly
        ///      encoded zero. Only the helper knows the mocked ABI, so it
        ///      computes those encodings and installs them here. That
        ///      reproduces smock, where an unstubbed function yields the zero
        ///      value of its return type.
        mapping(bytes4 => Response) baseResponseBySelector;
        /// @dev Selectors that must never be recorded, because the mocked ABI
        ///      declares them `view` or `pure` and so they arrive by
        ///      STATICCALL. See `__mock__record`.
        mapping(bytes4 => bool) nonRecording;
        /// @dev Keys of every exact-calldata entry configured so far, with the
        ///      selector each belongs to. Exact-calldata entries are keyed by
        ///      hash and so cannot be enumerated from the mapping; `reset`
        ///      needs to clear them, and inferring them from recorded calls
        ///      would be wrong because view calls are never recorded.
        bytes32[] configuredCalldataKeys;
        mapping(bytes32 => bytes4) selectorOfCalldataKey;
        mapping(bytes32 => bool) calldataKeyKnown;
        /// @dev Selectors given a default response, for the same reason.
        bytes4[] configuredSelectors;
        mapping(bytes4 => bool) selectorKnown;
        /// @dev Every call recorded, in order, as raw calldata. Kept whole
        ///      rather than decoded so the helper can decode against whichever
        ///      ABI the test declared.
        bytes[] receivedCalls;
        /// @dev msg.value of each recorded call, parallel to `receivedCalls`.
        ///      smock exposed this as `getCall(n).value`, and payable mocks
        ///      are asserted on it -- `transferTokensWithPayload` carries the
        ///      Wormhole message fee.
        uint256[] receivedValues;
        /// @dev Set to disable recording entirely.
        ///
        ///      Recording costs real gas -- smock zeroed gas for faked calls,
        ///      this contract SSTOREs the calldata -- so a test measuring the
        ///      gas of the contract under test would otherwise be measuring
        ///      the mock's bookkeeping too.
        bool recordingDisabled;
    }

    /// @dev keccak256("tbtc.test.MockContract.state.v1") - 1, masked, per the
    ///      ERC-7201 convention.
    /// @dev Gas handed to the recording self-call.
    ///
    ///      An explicit cap matters more than the value. try/catch does not
    ///      recover from out-of-gas: an uncapped sub-call takes 63/64 of what
    ///      is left, so if recording runs out, the caller is left with too
    ///      little to continue and the whole transaction reverts. Capping it
    ///      means a failed recording costs this much and nothing more, which
    ///      keeps recording genuinely best-effort. Generous enough for any
    ///      realistic calldata.
    uint256 private constant RECORD_GAS_STIPEND = 1_000_000;

    bytes32 private constant STATE_SLOT =
        0xdd9627cd601a6555f69239ac336fba8db95c419564b84ebb3ee2c21fed88ae00;

    function _state() private pure returns (State storage state) {
        bytes32 slot = STATE_SLOT;
        // solhint-disable-next-line no-inline-assembly
        assembly {
            state.slot := slot
        }
    }

    /// @notice Configures the response for one exact calldata payload.
    /// @param callData Full ABI-encoded calldata, selector included.
    /// @param returnData ABI-encoded return value. Empty for void functions.
    function __mock__setReturnForCalldata(
        bytes calldata callData,
        bytes calldata returnData
    ) external {
        __mock__rememberCalldataKey(callData);
        _state().responseByCalldata[keccak256(callData)] = Response(
            Behaviour.Return,
            returnData
        );
    }

    /// @notice Configures the default response for a selector, used when no
    ///         exact-calldata entry matches.
    function __mock__setReturnForSelector(
        bytes4 selector,
        bytes calldata returnData
    ) external {
        __mock__rememberSelector(selector);
        _state().responseBySelector[selector] = Response(
            Behaviour.Return,
            returnData
        );
    }

    /// @notice Makes one exact calldata payload revert.
    /// @param revertData Raw revert payload. Empty reverts with no data.
    function __mock__setRevertForCalldata(
        bytes calldata callData,
        bytes calldata revertData
    ) external {
        __mock__rememberCalldataKey(callData);
        _state().responseByCalldata[keccak256(callData)] = Response(
            Behaviour.Revert,
            revertData
        );
    }

    /// @notice Makes every call to a selector revert, unless an exact-calldata
    ///         entry matches first.
    function __mock__setRevertForSelector(
        bytes4 selector,
        bytes calldata revertData
    ) external {
        __mock__rememberSelector(selector);
        _state().responseBySelector[selector] = Response(
            Behaviour.Revert,
            revertData
        );
    }

    /// @notice Clears every response configured for one selector and forgets
    ///         the calls recorded for it. This is smock's `fn.reset()`.
    function __mock__resetSelector(bytes4 selector) external {
        delete _state().responseBySelector[selector];

        uint256 keptKeys = 0;
        uint256 totalKeys = _state().configuredCalldataKeys.length;
        for (uint256 i = 0; i < totalKeys; i++) {
            bytes32 key = _state().configuredCalldataKeys[i];

            if (_state().selectorOfCalldataKey[key] == selector) {
                delete _state().responseByCalldata[key];
                delete _state().selectorOfCalldataKey[key];
                delete _state().calldataKeyKnown[key];
            } else {
                _state().configuredCalldataKeys[keptKeys] = key;
                keptKeys++;
            }
        }
        while (_state().configuredCalldataKeys.length > keptKeys) {
            _state().configuredCalldataKeys.pop();
        }

        uint256 keptCalls = 0;
        uint256 totalCalls = _state().receivedCalls.length;
        for (uint256 i = 0; i < totalCalls; i++) {
            if (__mock__selectorOf(_state().receivedCalls[i]) != selector) {
                _state().receivedCalls[keptCalls] = _state().receivedCalls[i];
                _state().receivedValues[keptCalls] = _state().receivedValues[i];
                keptCalls++;
            }
        }
        while (_state().receivedCalls.length > keptCalls) {
            _state().receivedCalls.pop();
            _state().receivedValues.pop();
        }
    }

    /// @notice Clears every configured response and every recorded call.
    function __mock__reset() external {
        uint256 totalKeys = _state().configuredCalldataKeys.length;
        for (uint256 i = 0; i < totalKeys; i++) {
            bytes32 key = _state().configuredCalldataKeys[i];
            delete _state().responseByCalldata[key];
            delete _state().selectorOfCalldataKey[key];
            delete _state().calldataKeyKnown[key];
        }
        delete _state().configuredCalldataKeys;

        uint256 totalSelectors = _state().configuredSelectors.length;
        for (uint256 i = 0; i < totalSelectors; i++) {
            delete _state().responseBySelector[_state().configuredSelectors[i]];
            delete _state().selectorKnown[_state().configuredSelectors[i]];
        }
        delete _state().configuredSelectors;

        delete _state().receivedCalls;
        delete _state().receivedValues;
    }

    /// @notice Installs the responses of last resort. Called once by the
    ///         helper at construction with the zero value of every function's
    ///         return type.
    function __mock__setBaseReturns(
        bytes4[] calldata selectors,
        bytes[] calldata returnData
    ) external {
        require(
            selectors.length == returnData.length,
            "MockContract: length mismatch"
        );

        for (uint256 i = 0; i < selectors.length; i++) {
            _state().baseResponseBySelector[selectors[i]] = Response(
                Behaviour.Return,
                returnData[i]
            );
        }
    }

    /// @notice Turns call recording on or off for this mock.
    function __mock__setRecording(bool enabled) external {
        _state().recordingDisabled = !enabled;
    }

    /// @notice Marks selectors that must never be recorded. The helper calls
    ///         this once with every `view` and `pure` function of the mocked
    ///         ABI.
    function __mock__setNonRecordingSelectors(bytes4[] calldata selectors)
        external
    {
        for (uint256 i = 0; i < selectors.length; i++) {
            _state().nonRecording[selectors[i]] = true;
        }
    }

    /// @notice Whether a selector is excluded from recording.
    function __mock__isNonRecording(bytes4 selector)
        external
        view
        returns (bool)
    {
        return _state().nonRecording[selector];
    }

    /// @notice Clears the default response for one selector without touching
    ///         its exact-calldata entries or recorded calls.
    function __mock__clearSelector(bytes4 selector) external {
        delete _state().responseBySelector[selector];
    }

    /// @notice Appends a recorded call. Only ever invoked by this contract, on
    ///         itself, from `fallback`.
    /// @dev Recording is a storage write, so it is impossible when the mocked
    ///      function was reached by STATICCALL — which is what Solidity emits
    ///      for a `view` or `pure` function on the mocked interface. Routing
    ///      the write through an external self-call lets `fallback` attempt it
    ///      and carry on when it fails, so a stubbed view function still
    ///      answers instead of reverting.
    ///
    ///      The cost is that calls to view functions are not recorded, and so
    ///      cannot be asserted on. That is sound for this suite: every call
    ///      assertion in it targets a state-changing function, and a `view`
    ///      that a test wanted to count could not have been reached by
    ///      STATICCALL in the first place.
    function __mock__record(bytes calldata callData, uint256 value) external {
        require(
            msg.sender == address(this),
            "MockContract: recording is internal"
        );
        _state().receivedCalls.push(callData);
        _state().receivedValues.push(value);
    }

    /// @notice Number of calls recorded, across all selectors.
    function __mock__callCount() external view returns (uint256) {
        return _state().receivedCalls.length;
    }

    /// @notice Raw calldata of the i-th call recorded, across all selectors.
    function __mock__callAt(uint256 index)
        external
        view
        returns (bytes memory)
    {
        return _state().receivedCalls[index];
    }

    /// @notice Number of calls recorded for one selector.
    function __mock__callCountForSelector(bytes4 selector)
        external
        view
        returns (uint256 count)
    {
        uint256 total = _state().receivedCalls.length;
        for (uint256 i = 0; i < total; i++) {
            if (__mock__selectorOf(_state().receivedCalls[i]) == selector) {
                count++;
            }
        }
    }

    /// @notice Raw calldata of the i-th call recorded for one selector.
    /// @notice msg.value of the i-th call recorded for one selector.
    function __mock__callValueForSelectorAt(bytes4 selector, uint256 index)
        external
        view
        returns (uint256)
    {
        uint256 seen = 0;
        uint256 total = _state().receivedCalls.length;

        for (uint256 i = 0; i < total; i++) {
            if (__mock__selectorOf(_state().receivedCalls[i]) == selector) {
                if (seen == index) {
                    return _state().receivedValues[i];
                }
                seen++;
            }
        }

        revert("MockContract: call index out of range");
    }

    function __mock__callForSelectorAt(bytes4 selector, uint256 index)
        external
        view
        returns (bytes memory)
    {
        uint256 seen = 0;
        uint256 total = _state().receivedCalls.length;

        for (uint256 i = 0; i < total; i++) {
            if (__mock__selectorOf(_state().receivedCalls[i]) == selector) {
                if (seen == index) {
                    return _state().receivedCalls[i];
                }
                seen++;
            }
        }

        revert("MockContract: call index out of range");
    }

    /// @dev Leading four bytes of `callData`, or zero if it is shorter.
    function __mock__selectorOf(bytes memory callData)
        public
        pure
        returns (bytes4 selector)
    {
        if (callData.length < 4) {
            return bytes4(0);
        }

        // solhint-disable-next-line no-inline-assembly
        assembly {
            selector := mload(add(callData, 32))
        }
    }

    // A typed `fallback(bytes calldata) returns (bytes memory)` would read
    // better, but prettier-plugin-solidity silently rewrites it to
    // `fallback() external payable`, dropping the parameter and the return
    // type — a formatter quietly changing semantics. Reading msg.data and
    // returning through assembly is immune to that.
    // solhint-disable-next-line no-complex-fallback
    fallback() external payable {
        // A `view` or `pure` function on the mocked ABI arrives by STATICCALL,
        // where the storage write recording needs is impossible. Those
        // selectors are flagged up front so the attempt is skipped outright
        // rather than made and swallowed — which keeps them off the gas budget
        // of the hot path, SPV proofs being the case that matters.
        //
        // The try/catch still guards the rest: a state-changing function is
        // also reached statically under `eth_call`/`callStatic`.
        if (
            !_state().recordingDisabled &&
            !_state().nonRecording[__mock__selectorOf(msg.data)]
        ) {
            // solhint-disable-next-line no-empty-blocks
            try
                this.__mock__record{gas: RECORD_GAS_STIPEND}(
                    msg.data,
                    msg.value
                )
            {} catch {}
        }

        bytes memory result = __mock__responseFor(msg.data);

        // solhint-disable-next-line no-inline-assembly
        assembly {
            return(add(result, 32), mload(result))
        }
    }

    // solhint-disable-next-line no-empty-blocks
    receive() external payable {}

    /// @dev Resolves an incoming call against the three layers, most specific
    ///      first. Reverts here if the matched response is a configured revert.
    function __mock__responseFor(bytes memory callData)
        private
        view
        returns (bytes memory)
    {
        Response storage exact = _state().responseByCalldata[
            keccak256(callData)
        ];
        if (exact.behaviour != Behaviour.Unset) {
            return __mock__respond(exact);
        }

        bytes4 selector = __mock__selectorOf(callData);

        Response storage bySelector = _state().responseBySelector[selector];
        if (bySelector.behaviour != Behaviour.Unset) {
            return __mock__respond(bySelector);
        }

        Response storage base = _state().baseResponseBySelector[selector];
        if (base.behaviour != Behaviour.Unset) {
            return __mock__respond(base);
        }

        // Last resort: a block of zeroes long enough to decode as the zero
        // value of essentially any return shape.
        //
        // Empty returndata will not do. Solidity compares returndatasize
        // against the size its ABI expects and reverts the caller on a short
        // answer, and the helper's per-function zeroes only cover the ABI it
        // was given. Production code reaches functions outside that ABI --
        // `AbstractBTCDepositor` calls `bridge.depositParameters()` through its
        // own view of the Bridge -- and smock answered those too, with exactly
        // this: a fixed run of zero bytes.
        return new bytes(2048);
    }

    function __mock__rememberSelector(bytes4 selector) private {
        if (!_state().selectorKnown[selector]) {
            _state().selectorKnown[selector] = true;
            _state().configuredSelectors.push(selector);
        }
    }

    function __mock__rememberCalldataKey(bytes calldata callData) private {
        bytes32 key = keccak256(callData);

        if (!_state().calldataKeyKnown[key]) {
            _state().calldataKeyKnown[key] = true;
            _state().selectorOfCalldataKey[key] = __mock__selectorOf(callData);
            _state().configuredCalldataKeys.push(key);
        }
    }

    function __mock__respond(Response storage response)
        private
        view
        returns (bytes memory)
    {
        if (response.behaviour == Behaviour.Revert) {
            bytes memory revertData = response.data;

            // solhint-disable-next-line no-inline-assembly
            assembly {
                revert(add(revertData, 32), mload(revertData))
            }
        }

        return response.data;
    }
}
