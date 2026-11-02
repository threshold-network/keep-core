# Entry Point Map

> keep-core (PR #4282 UTXO Reservations Epic, merged head) | 54 entry points | 21 permissionless | 24 role-gated | 9 admin-only
>
> Phase C delta vs the first run: `clientinfo.registerPprofHandlers()` REMOVED (commit `d8dacd993` deletes the pprof wiring and the `EnablePprof` setting; the metrics server serves `/metrics` only). `ethereum.TbtcChain.GetLatestBlockTimestamp()` ADDED (commit `d59e38fd4`/`41fb820a5`; the chain head time accessor every reservation deadline gate now reads). No other entry point was added or removed by the fixes or by dev.

Line numbers are from the frozen worktree at the merged fix head.

---

## Protocol Flow Paths

### 1. Reservation Acceptance Lifecycle
`DepositRevealed` ◄── revealed to reservation vault, mature (age >= depositMinAge in CHAIN time), confirmations >= 6
       │
       ▼
`requestReservationAcceptance()` ◄── depositor requests on Bridge (ReservationRouter)
       │
       ▼
`ReservationAcceptanceTask.Run()` ◄── coordinator discovers pending action, wallet Live or MovingFunds, chainHead + 7200s < TimeoutAt (head time read once per pass; pass fails if unreadable)
       │
       ▼
`ValidateReservationAnchorProposal()` ◄── on-chain proposal simulation against WalletProposalValidator
       │
       ▼
`reservationAnchorAction.execute()` ◄── 1-in-1-out P2WPKH BTC tx assembled; fee <= TxMaxFee; follower hard-rejects fee < 5 sat/vB protocol min; signed within 300-block safety margin
       │
       ▼
`maintainReservationProofs()` ◄── BTC anchor tx confirmed >= requiredConfirmations on Bitcoin; TimedOut generations kept until chain head > TimeoutAt + TermSeconds
       │
       ▼
`SubmitReservationAcceptanceProof()` ◄── onlySpvMaintainer submits BitcoinTxInfo + BitcoinTxProof to Bridge; tBTC minted to vault

### 2. Reservation Re-Anchor Lifecycle & MovingFunds Wind-Down
Wallet enters `MovingFunds` or `Closing` ◄── wallet retiring, custodising active reservation anchors
       │
       ▼
`ReservationReanchorTask.Run()` ◄── discovers Live target wallet with count/amount headroom, cooldown elapsed in CHAIN time, target older than its creation period (IsReanchorTargetAged, chain head + 0 tolerance)
       ├─→ `RequestReservationReanchor()` ◄── coordinator submits request to Bridge (at most 1 per pass)
       │          │
       │          ▼
       ├─→ `ValidateReservationReanchorProposal()` ◄── on-chain simulation against WalletProposalValidator
       │          │
       │          ▼
       ├─→ `reservationReanchorAction.execute()` ◄── 1-in-1-out P2WPKH BTC tx; follower re-checks target age on chain head (24 s tolerance) before co-signing
       │          │
       │          ▼
       ├─→ `SubmitReservationReanchorProof()` ◄── SPV maintainer submits proof; custody shifted to target; in-kind fee financed
       │
       └─→ `NotifyMovingFundsBelowDust()` ◄── zero reservations remaining, main UTXO < dust threshold, prior reservation history

### 3. Housekeeping Watchers (Timeout, Stale-Deposit, Stranding)
Pending action unproven ◄── CHAIN head >= TimeoutAt + operatorStaggerOffset (head time from GetLatestBlockTimestamp; skip the decision entirely when the head is unreadable; 10-min renotify backoff still on node clock)
       ├─→ `alertOverdueAction()` ◄── NEW Warn log + spv_reservation_action_overdue_total metric on every overdue notify decision
       ├─→ `NotifyReservationAcceptanceTimedOut()` ◄── acceptance action: marks TimedOut, releases capacity
       └─→ `NotifyReservationActionTimeout()` ◄── reanchor action: marks TimedOut, defers wallet to stranding recheck

Deposit revealed to vault unaccepted ◄── CHAIN head > refundDeadline + staggerOffset; action not Pending
       └─→ `NotifyStaleReservedDeposit()` ◄── releases reservation lock; confirmed only when on-chain wallet field clears (0)

Wallet closed or terminated (`OnWalletClosed`) ◄── wallet state Closed or Terminated
       └─→ `checkReservationStrandingForWallet()` ◄── walks WalletReservations; Closing-wallet form additionally waits for chain head >= DissolutionEligibleAt
                  └─→ `NotifyReservationStranded()` ◄── marks Active reservation as Stranded

### 4. Chain-Time Source (NEW since the first run)
`TbtcChain.GetLatestBlockTimestamp()` ◄── chain.go interface method (spv.Chain, tbtc.BridgeChain)
       │  reads latest header time via bounded currentBlock()
       ▼
Feeds: timeout-notify gate, stale-deposit refund gate, proof-loop eviction/abandon, dissolution-eligible gate, acceptance candidate gate, re-anchor cooldown/margin/age gates, target-age co-sign gate
       └── zero time on read failure => decision skipped for the pass (fail-safe: notify late or not at all; never early)

---

## Permissionless

Entry points callable by any party, triggered by permissionless on-chain events, or running as unpermissioned background keeper poll loops.

### `spv.ReservationActionTimeoutWatcher.Run()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_action_timeout_watch.go:273`, exported background poll ticker loop |
| Caller | Ticker loop at `DefaultReservationActionTimeoutPollInterval` (1 min), spawned by `WireReservationWatchers` |
| Parameters | `ctx context.Context` `(operator-config)` |
| Call chain | `→ pollPendingActions() → drainStrandingRechecks() → scanActionRequests() → checkReservationActionTimeout() → spvChain.NotifyReservationAcceptanceTimedOut() / spvChain.NotifyReservationActionTimeout()` |
| State modified | `pendingActions[key]` (tracking map, evicted on non-Pending, on abandon, on settled), `strandingRecheckWallets`, `acceptanceLastScannedBlock`, `reanchorLastScannedBlock`, `item.notifiedAt` (node-clock stamp only) |
| Value flow | On-chain tx: `ReservationRouter.notifyReservationAcceptanceTimedOut` or `notifyReservationActionTimeout` (via `Bridge.fallback` delegatecall) |
| Concurrency guard | Dedicated single goroutine; sequential per-tick execution; bounded per-tick check batch (`nextActionCheckBatch`); panic recovery in wiring wrapper: Yes |
| Phase C delta | Deadline gate now chain-time: `headTime := reservationChainHeadTime(...)` (line 356), zero-time skip (461), `headNow >= deadline` (465); overdue-action alert fires at the same decision (466) |

### `spv.ReservationActionTimeoutWatcher.CheckReservationActionTimeouts()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_action_timeout_watch.go:594`, exported query/trigger method |
| Caller | External caller or automated maintenance runner (documented as the "other production integration path"; tests drive it directly) |
| Parameters | `reservationKey *big.Int` `(user-controlled)`, `now uint32` `(user-controlled)` - the caller-supplied `now` is used only for the renotify backoff bookkeeping; the deadline decision is still gated by `checkReservationActionTimeout`'s `now < action.TimeoutAt` predicate at line 680 on the value the caller passed |
| Call chain | `→ checkReservationActionTimeout() → spvChain.GetReservation() → spvChain.GetReservationAction() → spvChain.NotifyReservationAcceptanceTimedOut() / spvChain.NotifyReservationActionTimeout()` |
| State modified | `strandingRecheckWallets[walletPublicKeyHash]` (when action is Reanchor type, enqueued BEFORE the notify call - G2-F18) |
| Value flow | On-chain tx: `ReservationRouter.notifyReservationAcceptanceTimedOut` or `notifyReservationActionTimeout` |
| Concurrency guard | Stateless per-call execution; reused preloads are re-read before a transaction is spent (line 696): Yes |

### `spv.ReservationStaleDepositWatcher.Run()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_stale_deposit_watch.go:722`, exported background poll ticker loop |
| Caller | Ticker loop at the stale-depoll interval (1 min), spawned by `WireReservationWatchers` |
| Parameters | `ctx context.Context` `(operator-config)` |
| Call chain | `→ pollTick(now) → trackRevealedDeposits() → CheckStaleReservedDeposit() → spvChain.NotifyStaleReservedDeposit()` |
| State modified | `pending[depositKey]`, `attempted[depositKey]`, `notifiedAt[depositKey]`, `lastSeenBlock`, `checkCursor` (bounded per-tick rotation, NEW in `41fb820a5`) |
| Value flow | On-chain tx: `ReservationRouter.notifyStaleReservedDeposit` |
| Concurrency guard | Dedicated single goroutine; panic recovery in wiring wrapper: Yes |
| Phase C delta | The refund-deadline gate inside `CheckStaleReservedDeposit` now judges on chain head time (`reservation_stale_deposit_watch.go:272-281`); the node clock (`now`) is only the backoff token |

### `spv.ReservationStaleDepositWatcher.CheckStaleReservedDeposit()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_stale_deposit_watch.go:252`, exported check method |
| Caller | `pollTick()` or external keeper invocation |
| Parameters | `depositKey *big.Int` `(user-controlled)`, `refundDeadline uint32` `(chain-derived)`, `now uint32` `(operator-config; backoff only - deadline gate uses chain head time)` |
| Call chain | `→ reservationChainHeadTime() → spvChain.ReservedDepositWallet() → spvChain.GetReservation() → spvChain.GetReservationAction() → spvChain.NotifyStaleReservedDeposit()` |
| State modified | `notifiedAt[depositKeyStr]`, `attempted[depositKeyStr]` |
| Value flow | On-chain tx: `ReservationRouter.notifyStaleReservedDeposit` |
| Concurrency guard | Sequential within the watcher goroutine: Yes |

### `spv.reservationStrandingWatcher.checkReservationStrandingForWallet()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_stranding_watch.go`, unexported watcher method |
| Caller | (a) Live `OnWalletClosed` handler (`reservation_wiring.go`), (b) startup catch-up scan, (c) startup second-chance retry, (d) action-timeout watcher recheck drain (`reservation_action_timeout_watch.go:755`, enqueued before notify - G2-F18), (e) Closing-wallet re-queue on transient failure (G2-F19, lines 841-872) |
| Parameters | `walletPublicKeyHash [20]byte` `(chain-derived)` |
| Call chain | `→ withStrandingRetry() → spvChain.WalletReservations() → spvChain.WalletTerminationCause() → spvChain.GetReservation() → (Closing form: spvChain.GetLatestBlockTimestamp) → spvChain.NotifyReservationStranded()` |
| State modified | None (stateless helper; recheck decisions returned to the caller) |
| Value flow | On-chain tx: `ReservationRouter.notifyReservationStranded` |
| Concurrency guard | Stateless; idempotent on Bridge; Closing form reads chain head time and hard-fails the pass on read error (line 177-184): Yes |
| Phase C delta | NEW dissolution-eligible gate: a Closing wallet strands a reservation only once `chain head >= reservation.DissolutionEligibleAt` (lines 256-258, G2-F17) |

### `spv.subscribeReservationWalletClosed` (handler)

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_wiring.go`, event subscription callback |
| Caller | Host chain `WalletClosed` event emitted by Bridge |
| Parameters | `event *tbtc.WalletClosedEvent` `(chain-derived)` |
| Call chain | `→ resolveWalletPublicKeyHash() → watcher.checkReservationStrandingForWallet() → spvChain.NotifyReservationStranded()` |
| State modified | None |
| Value flow | On-chain tx: `ReservationRouter.notifyReservationStranded` |
| Concurrency guard | Dedicated goroutine per event with `ctx.Done()` exit check: Yes |

### `spv.maintainReservationProofs` (background loop entry)

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/maintainer/spv/reservation_proof_loop.go`, unexported, spawned as a goroutine by `spv.Initialize` when `ReservationProofsEnabled` |
| Caller | Operator CLI / `spv.Initialize` background start |
| Call chain | `→ runReservationProofLoop() → proveReservationAcceptanceActions() / proveReservationReanchorActions() → submitReservationActionProof() → spvChain.SubmitReservationAcceptanceProof() / SubmitReservationReanchorProof()` |
| State modified | `reservationProofScanState` (cursors, pending maps, backoff map, wallet tx cache - evicted at end of each pass for wallets no longer pending) |
| Value flow | On-chain tx: `ReservationRouter.submitReservationAcceptanceProof` / `submitReservationReanchorProof` |
| Concurrency guard | Single goroutine; one proof per reservation key per pass; bounded backoff (32 passes max): Yes |
| Phase C delta | Late-window eviction and abandon judgments now on chain head time (`reservation_proof_loop.go:466,494-498,528,939,1143-1172`); wallet-tx cache eviction added (G2-L5) |

### `ethereum.TbtcChain.GetLatestBlockTimestamp()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc_chain_time.go:15`, exported view method (NEW in this head) |
| Caller | All reservation deadline gates: `spv.Chain`/`tbtc.BridgeChain` consumers - proof loop (`reservationChainHeadTime`), both watchers, stranding Closing form, `tbtcpg` generators (`reservation_acceptance.go:433`, `reservation_reanchor.go:227`), follower re-anchor target-age check (`reservation.go:893`) |
| Parameters | none (reads the latest header) |
| Call chain | `→ tc.currentBlock() → time.Unix(header.Time)` with bounded timeout |
| State modified | None |
| Value flow | None (view) |
| Concurrency guard | Stateless RPC read: Yes |
| Trust | Bounded untrusted data source: a lagging or reorging head only defers deadline decisions; zero time on error disables the decision for the pass |

### `ethereum.TbtcChain.OnReservationAcceptanceRequested()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported event subscription method |
| Caller | Host chain `ReservationAcceptanceRequested` event filter on Bridge |
| Parameters | `handler func(*ReservationAcceptanceRequestedEvent)` `(operator-config)` |
| Call chain | `→ tc.reservationRouter.ReservationAcceptanceRequestedEvent().OnEvent(onEvent)` |
| State modified | Active event filter subscription in go-ethereum client |
| Value flow | None |
| Concurrency guard | Managed by go-ethereum event subscription worker: Yes |

### `ethereum.TbtcChain.OnReservationReanchorRequested()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported event subscription method |
| Caller | Host chain `ReservationReanchorRequested` event filter on Bridge |
| Parameters | `handler func(*ReservationReanchorRequestedEvent)` `(operator-config)` |
| Call chain | `→ tc.reservationRouter.ReservationReanchorRequestedEvent().OnEvent(onEvent)` |
| State modified | Active event filter subscription |
| Value flow | None |
| Concurrency guard | go-ethereum subscription worker: Yes |

### `ethereum.TbtcChain.PastReservationAcceptanceRequestedEvents()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported event log query method |
| Caller | SPV proof loop, action-timeout watcher, acceptance proposal generator |
| Parameters | `filter *ReservationAcceptanceRequestedEventFilter` `(operator-config)` |
| Call chain | `→ tc.reservationRouter.PastReservationAcceptanceRequestedEvents()` in 10,000-block chunks; chunk errors preserve the cursor |
| State modified | None |
| Value flow | None |
| Concurrency guard | Stateless RPC log query: Yes |

### `ethereum.TbtcChain.PastReservationReanchorRequestedEvents()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported event log query method |
| Caller | SPV proof loop, action-timeout watcher, reanchor proposal generator |
| Parameters | `filter *ReservationReanchorRequestedEventFilter` `(operator-config)` |
| Call chain | `→ tc.reservationRouter.PastReservationReanchorRequestedEvents()` |
| State modified | None |
| Value flow | None |
| Concurrency guard | Stateless RPC log query: Yes |

### `ethereum.TbtcChain.NotifyMovingFundsBelowDust()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported transaction submitter |
| Caller | `ReservationReanchorTask.notifyMovingFundsBelowDustIfEligible` (`pkg/tbtcpg/reservation_reanchor.go:1455`) |
| Parameters | `walletPublicKeyHash [20]byte` `(chain-derived)`, `mainUtxo *bitcoin.UnspentTransactionOutput` `(chain-derived)` |
| Call chain | `→ gasEstimate (20% ceiling integer margin) → tc.bridge.NotifyMovingFundsBelowDust()` |
| State modified | None in Go client (on-chain updates Bridge wallet state to Closing) |
| Value flow | On-chain tx: `Bridge.notifyMovingFundsBelowDust` |
| Concurrency guard | Sequential transaction dispatch: Yes |

### `ethereum.TbtcChain.WalletReservations()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go:1305`-area, exported key discovery query |
| Caller | Stranding watcher (`checkReservationStrandingForWallet`), `ReservationReanchorTask.Run` |
| Parameters | `walletPublicKeyHash [20]byte` `(chain-derived)` |
| Call chain | `→ WalletReservationsCount() (cheap short-circuit at line 1317) → earliestWalletRegistrationBlock() → PastReservationAcceptanceRequestedEvents() → PastReservationReanchorRequestedEvents() → resolveCustodiedReservationKeys()` |
| State modified | `walletRegistrationBlockCache` (`sync.Map`) |
| Value flow | None |
| Concurrency guard | `sync.Map` cache: Yes |
| Phase C delta | Failed registration lookups now fail the query instead of falling back to a genesis scan; one unreadable reservation no longer aborts the rest (`a29a69690`) |

### `ethereum.TbtcChain.WalletTerminationCause()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/chain/ethereum/tbtc.go`, exported event analysis query |
| Caller | Stranding watcher |
| Parameters | `walletPublicKeyHash [20]byte` `(chain-derived)` |
| Call chain | `→ earliestWalletRegistrationBlock() → PastMovingFundsTimedOutEvents() → ... → resolveWalletTerminationCause()` |
| State modified | `walletRegistrationBlockCache` |
| Value flow | None |
| Concurrency guard | `sync.Map` cache: Yes |

### `tbtc.FindDepositRevealedEvent()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/tbtc/deposit_reveal_lookup.go:41`, exported search function |
| Caller | Deposit sweep proposal validator (`pkg/tbtc/deposit_sweep.go`), `FindDepositRevealedEventByRevealTime` |
| Parameters | `source` `(chain-derived)`, `walletPublicKeyHash`, `fundingTxHash`, `fundingOutputIndex`, `fromBlock`, `toBlock` |
| Call chain | `→ source.PastDepositRevealedEvents()` in 10,000-block chunks |
| State modified | None |
| Value flow | None |
| Concurrency guard | Stateless pure function: Yes |

### `tbtc.FindDepositRevealedEventByRevealTime()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/tbtc/deposit_reveal_lookup.go:100`, exported search function |
| Caller | `reservationAnchorAction.execute` (`pkg/tbtc/reservation.go:625`, inside a 30 s RPC timeout wrapper), acceptance generator |
| Parameters | `depositRevealedAt time.Time` `(chain-derived)`, `currentBlock` `(proposal startBlock)`, `averageBlockTime` `(operator-config)` |
| Call chain | `→ FindDepositRevealedEvent()` (with 4x outer-margin widening retry) |
| State modified | None |
| Value flow | None |
| Concurrency guard | Stateless pure function; caller-bounded: Yes |

### `tbtc.unmarshalCoordinationProposal()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/tbtc/marshaling.go:219`, unmarshaler dispatcher |
| Caller | `coordinationMessage.Unmarshal` processing incoming P2P coordination messages |
| Parameters | `actionType uint32` `(peer-provided)`, `payload []byte` `(peer-provided)` |
| Call chain | `→ ParseWalletActionType() → proposal.Unmarshal(payload)` |
| State modified | None (returns the unmarshaled proposal or an error) |
| Value flow | None |
| Concurrency guard | Stateless: Yes |

### `tbtc.ReservationAnchorProposal.Unmarshal()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/tbtc/marshaling.go:525`, exported struct unmarshaler |
| Caller | `unmarshalCoordinationProposal` |
| Parameters | `data []byte` `(peer-provided)` |
| Call chain | `→ proto.Unmarshal()` + strict bounds: fee > 0 and <= 8 bytes (505), nonce != 0 (540), funding-tx hash exactly 32 bytes and non-zero (543) |
| State modified | Struct fields |
| Value flow | None |
| Concurrency guard | Stateless: Yes |

### `tbtc.ReservationReanchorProposal.Unmarshal()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/tbtc/marshaling.go:598`, exported struct unmarshaler |
| Caller | `unmarshalCoordinationProposal` |
| Parameters | `data []byte` `(peer-provided)` |
| Call chain | `→ proto.Unmarshal()` + strict bounds: key <= 32 bytes and positive (607,637), nonce != 0 (613), fee > 0 and <= 8 bytes (578,645), target PKH exactly 20 bytes and non-zero (625,632) |
| State modified | Struct fields |
| Value flow | None |
| Concurrency guard | Stateless: Yes |

### `clientinfo.Initialize` (HTTP metrics endpoint)

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/clientinfo/clientinfo.go:39`-area, HTTP handler initialization |
| Caller | Prometheus scraper / HTTP GET `/metrics` on configured `clientinfo.Port` |
| Parameters | HTTP request `(user-controlled)` |
| Call chain | `→ clientinfo.Registry.Handler() → PerformanceMetrics.Gather()` |
| State modified | None (reads atomic gauges and counters) |
| Value flow | None |
| Concurrency guard | `sync.RWMutex` protected metric maps: Yes |
| Phase C delta | pprof REMOVED: the `EnablePprof` setting and `registerPprofHandlers()` no longer exist (`d8dacd993`); the metrics server serves `/metrics` only, and a regression test pins that `/debug/pprof` is not served |

### `crypto.UnmarshalPublicKey()`

| Aspect | Detail |
|--------|--------|
| Visibility / Invocation | `pkg/crypto/ephemeral/private_key.go`-area, exported key point parser |
| Caller | `tecdsa.generateSymmetricKeys` (`pkg/tecdsa/dkg/protocol.go`, `pkg/tecdsa/signing/protocol.go`) and the ephemeral key map unmarshalers |
| Parameters | `data []byte` `(peer-provided)` |
| Call chain | `→ btcec.ParsePubKey()` - the DKG/signing unmarshalers now validate every ephemeral key map entry as a 33-byte compressed secp256k1 point on the ECDH curve; one malformed entry fails the whole message (`c5688f92c`) |
| State modified | None |
| Value flow | None |
| Concurrency guard | Stateless; invalid point errors wrap to `ErrInvalidPublicKey` (member marked inactive): Yes |

---

## Role-Gated

Entry points restricted by smart contract roles (`onlySpvMaintainer`), wallet threshold group membership, or coordination leadership.

### SPV Maintainer (`onlySpvMaintainer`)

| Component & Function | Location | Caller / Trigger | Parameters & Trust | Call Chain | State Modified | Value Flow |
|---|---|---|---|---|---|---|
| `spv.maintainReservationProofs` | `pkg/maintainer/spv/reservation_proof_loop.go` | Background goroutine spawned by `spv.Initialize` | `ctx`, `config` `(operator-config)` | `→ runReservationProofLoop() → proveReservationAcceptanceActions() / proveReservationReanchorActions() → Submit*Proof()` | `reservationProofScanState` | On-chain tx: `submitReservationAcceptanceProof` / `submitReservationReanchorProof` |
| `spv.SubmitReservationAcceptanceProof` | `pkg/maintainer/spv/reservation_acceptance_proof.go:23` | `proveReservationAcceptanceActions` | `transactionHash`, `requiredConfirmations`, `reservationKey`, `requestNonce` `(chain-derived)` | `→ submitReservationActionProof() → spvChain.SubmitReservationAcceptanceProof()` | Prometheus counters | On-chain tx via `Bridge.fallback` |
| `spv.SubmitReservationReanchorProof` | `pkg/maintainer/spv/reservation_reanchor_proof.go:24` | `proveReservationReanchorActions` | same shape | `→ submitReservationActionProof() → spvChain.SubmitReservationReanchorProof()` | Prometheus counters | On-chain tx via `Bridge.fallback` |
| `spv.SubmitDepositSweepProof` | `pkg/maintainer/spv/deposit_sweep.go` | `maintainSpv` control loop | `(chain-derived)` | `→ bitcoin.AssembleSpvProof() → spvChain.SubmitDepositSweepProofWithReimbursement()` | Prometheus metrics | On-chain tx |
| `spv.SubmitRedemptionProof` | `pkg/maintainer/spv/redemptions.go` | `maintainSpv` control loop | `(chain-derived)` | `→ bitcoin.AssembleSpvProof() → spvChain.SubmitRedemptionProofWithReimbursement()` | Prometheus metrics | On-chain tx |
| `spv.SubmitMovingFundsProof` | `pkg/maintainer/spv/moving_funds.go` | `maintainSpv` control loop | `(chain-derived)` | `→ spvChain.SubmitMovingFundsProofWithReimbursement()` | None | On-chain tx |
| `spv.SubmitMovedFundsSweepProof` | `pkg/maintainer/spv/moved_funds_sweep.go` | `maintainSpv` control loop | `(chain-derived)` | `→ spvChain.SubmitMovedFundsSweepProofWithReimbursement()` | None | On-chain tx |
| `ethereum.TbtcChain.SubmitReservationAcceptanceProof` | `pkg/chain/ethereum/tbtc.go:868`-area | `spv.SubmitReservationAcceptanceProof` | `(chain-derived)` | `→ gasEstimate (20% margin) → tc.reservationRouter.SubmitReservationAcceptanceProof()` | None | On-chain tx |
| `ethereum.TbtcChain.SubmitReservationReanchorProof` | `pkg/chain/ethereum/tbtc.go` | `spv.SubmitReservationReanchorProof` | `(chain-derived)` | `→ gasEstimate → tc.reservationRouter.SubmitReservationReanchorProof()` | None | On-chain tx |

### Wallet Coordination Leader

| Component & Function | Location | Caller / Trigger | Parameters & Trust | Call Chain | State Modified | Value Flow |
|---|---|---|---|---|---|---|
| `tbtc.coordinationExecutor.coordinate` | `pkg/tbtc/coordination.go:412`-area | Coordination window trigger | `window *coordinationWindow` `(chain-derived)` | `→ executeLeaderRoutine() → proposalGenerator.Generate() → broadcastChannel.Send()` | Coordination metrics | libp2p `coordinationMessage` |
| `tbtcpg.ProposalGenerator.Generate` | `pkg/tbtcpg/tbtcpg.go:135`-area | `executeLeaderRoutine` | `request *CoordinationProposalRequest` `(group-derived)` | `→ tasks[taskIndex].Run(request)` | None | Candidate proposal |
| `tbtcpg.ReservationAcceptanceTask.Run` | `pkg/tbtcpg/reservation_acceptance.go:191`-area | `ProposalGenerator.Generate` | `(group-derived)` | `→ chain.GetLatestBlockTimestamp() → findReservationAcceptanceCandidates() → fetchReservationAcceptanceFundingTxs() → proposeReservationAcceptance() → ValidateReservationAnchorProposal()` | `revealCache` | None |
| `tbtcpg.ReservationReanchorTask.Run` | `pkg/tbtcpg/reservation_reanchor.go:129` | `ProposalGenerator.Generate` | `(group-derived)` | `→ chain.GetLatestBlockTimestamp() → target search (caps + age gate) → RequestReservationReanchor() → ValidateReservationReanchorProposal()` | Search cache | On-chain tx: `requestReservationReanchor`, `notifyMovingFundsBelowDust` |
| `ethereum.TbtcChain.RequestReservationReanchor` | `pkg/chain/ethereum/tbtc.go:840`-area | `ReservationReanchorTask.ProposeReservationReanchor` | `(chain-derived)` | `→ gasEstimate → tc.reservationRouter.RequestReservationReanchor()` | None | On-chain tx |
| `tbtcpg.DepositSweepTask.Run` | `pkg/tbtcpg/deposit_sweep.go:47` | `ProposalGenerator.Generate` | `(group-derived)` | `→ FindDeposits() → ProposeDepositsSweep() → ValidateDepositSweepProposal()` | None | None |
| `tbtcpg.RedemptionTask.Run` | `pkg/tbtcpg/redemptions.go` | `ProposalGenerator.Generate` | `(group-derived)` | `→ ProposeRedemption() → EstimateRedemptionFee()` | None | None |
| `tbtcpg.MovingFundsTask.Run` | `pkg/tbtcpg/moving_funds.go:50` | `ProposalGenerator.Generate` | `(group-derived)` | `→ FindDeposits() → EstimateMovingFundsFee() → SubmitMovingFundsCommitment()` | None | On-chain tx: commitment |

### Wallet Signing Group Follower / Member

| Component & Function | Location | Caller / Trigger | Parameters & Trust | Call Chain | State Modified | Value Flow |
|---|---|---|---|---|---|---|
| `tbtc.coordinationExecutor.executeFollowerRoutine` | `pkg/tbtc/coordination.go` | Window trigger | `leader`, `coordinationBlock`, `actionsAllowed` `(chain-derived)` | `→ broadcastChannel.Recv() → validateMessage() → ParseWalletActionType()` | Coordination fault records | None |
| `tbtc.node.handleReservationAnchorProposal` | `pkg/tbtc/node_proposals.go:302` | `processCoordinationResult` | `proposal *ReservationAnchorProposal` `(peer-provided)` | `→ newReservationAnchorAction() → walletDispatcher.dispatch(action)` | Enqueued action | None |
| `tbtc.node.handleReservationReanchorProposal` | `pkg/tbtc/node_proposals.go:363` | `processCoordinationResult` | `proposal *ReservationReanchorProposal` `(peer-provided)` | `→ newReservationReanchorAction() → walletDispatcher.dispatch(action)` | Enqueued action | None |
| `tbtc.node.handleDepositSweepProposal` | `pkg/tbtc/node_proposals.go:100` | `processCoordinationResult` | `(peer-provided)` | `→ newDepositSweepAction() → walletDispatcher.dispatch(action)` | Enqueued action | None |
| `tbtc.reservationAnchorAction.execute` | `pkg/tbtc/reservation.go:497`-area | `walletDispatcher` worker | `(peer-provided)` | `→ GetDepositRequest() → FindDepositRevealedEventByRevealTime() → GetReservationAction() → ValidateReservationAnchorProposal() → rejectIfProposedWalletTxFeeBelowProtocolMin() → Assemble → signTransaction() → broadcastTransaction()` | None | BTC tx signed & broadcast |
| `tbtc.reservationReanchorAction.execute` | `pkg/tbtc/reservation.go:692`-area | `walletDispatcher` worker | `(peer-provided)` | `→ GetReservation() → GetReservationAction() → ValidateReservationReanchorProposal() → GetLatestBlockTimestamp() + IsReanchorTargetAged() → rejectIfProposedWalletTxFeeBelowProtocolMin() → Assemble → sign → broadcast` | None | BTC tx signed & broadcast |
| `tbtc.depositSweepAction.execute` | `pkg/tbtc/deposit_sweep.go:204` | `walletDispatcher` worker | `(peer-provided)` | `→ ValidateDepositSweepProposal() → assemble → sign → broadcast` | Duration metrics | BTC tx |
| `tbtc.ValidateDepositSweepProposal` | `pkg/tbtc/deposit_sweep.go:382` | `depositSweepAction.execute` | `(peer-provided)` | `→ ReservationParameters() → IsReservedDeposit() → chain.ValidateDepositSweepProposal() → rejectIfProposedWalletTxFeeBelowProtocolMin() (705)` | None | None |
| `ethereum.TbtcChain.ValidateReservationAnchorProposal` | `pkg/chain/ethereum/tbtc.go:491` | anchor execute, acceptance generator | `(group-derived)`, `(peer-provided)` | `→ tc.walletProposalValidator.ValidateReservationAnchorProposal()` | None | On-chain view |
| `ethereum.TbtcChain.ValidateReservationReanchorProposal` | `pkg/chain/ethereum/tbtc.go:582` | reanchor execute, reanchor generator | `(group-derived)`, `(peer-provided)` | `→ tc.walletProposalValidator.ValidateReservationReanchorProposal()` | None | On-chain view |
| `tecdsa.dkg.generateSymmetricKeys` | `pkg/tecdsa/dkg/protocol.go:61` | tECDSA DKG round | `(peer-provided)` | `→ ephemeral key map unmarshaler (out-of-range point rejection, c5688f92c) → ecdh()` | `symmetricKeys` | None |
| `tecdsa.signing.generateSymmetricKeys` | `pkg/tecdsa/signing/protocol.go:61` | tECDSA signing round | `(peer-provided)` | same pattern | `symmetricKeys` | None |

### Bitcoin Difficulty Maintainer (Authorized Submitter)

| Component & Function | Location | Caller / Trigger | Parameters & Trust | Call Chain | Value Flow |
|---|---|---|---|---|---|
| `btcdiff.bitcoinDifficultyMaintainer.proveEpochs` | `pkg/maintainer/btcdiff/bitcoin_difficulty.go` | Maintainer control loop | `ctx` `(operator-config)` | `→ verifySubmissionEligibility() → proveNextEpoch() → Retarget() / RetargetWithRefund()` | On-chain tx: `LightRelay.retarget` |
| `btcdiff.bitcoinDifficultyMaintainer.startControlLoop` | same file | `btcdiff.Initialize` | `ctx` | `→ proveEpochs()` | None |

### Random Beacon Group Member

| Component & Function | Location | Caller / Trigger | Parameters & Trust | Call Chain | Value Flow |
|---|---|---|---|---|---|
| `beacon.node.JoinDKGIfEligible` | `pkg/beacon/node.go` | On-chain `DKGStarted` event | `seed` `(chain-derived)` | `→ setBroadcastChannelFilter() → dkgExecutor.Execute()` | On-chain tx: `submitDkgResult` |
| `beacon.node.GenerateRelayEntry` | `pkg/beacon/node.go` | On-chain `RelayEntryRequested` event | `previousEntry` `(chain-derived)` | `→ signatureAggregator.Sign()` | On-chain tx: `submitRelayEntry` |

---

## Admin-Only

Entry points restricted to the node operator CLI, process configuration, and local credentials.

### Lifecycle & Initialization Functions

| Component & Function | Location | Trigger / Caller | Parameters & Trust | Call Chain | Value Flow |
|---|---|---|---|---|---|
| `cmd.start` | `cmd/start.go:67`-area | Operator CLI `keep-core start` | `cmd` `(operator-config)` | `→ electrum.Connect() → ConnectEthereum() → tbtc.Initialize() → spv.WireReservationWatchers(pairedFlagEnabled)` | Daemon subsystems, watcher goroutines |
| `cmd.maintainers` | `cmd/maintainer.go:49` | Operator CLI `keep-core maintainer` | `cmd` `(operator-config)` | `→ maintainer.Initialize()` | SPV + btcdiff control loops |
| `tbtc.Initialize` | `pkg/tbtc/tbtc.go:246`-area | `cmd.start` | `config` `(operator-config)` | `→ applyWalletTxFeePolicy() (hard-fails a floor below the 5 sat/vB protocol minimum) → newNode() → runCoordinationLayer()` | Fee policy vars, node subsystems |
| `spv.Initialize` | `pkg/maintainer/spv/spv.go:66`-area | `cmd.maintainers` | `config`, `spvChain`, `btcChain` | `→ maintainReservationProofs() → WireReservationWatchers() → startControlLoop()` | Proof loops, watcher goroutines |
| `spv.WireReservationWatchers` | `pkg/maintainer/spv/reservation_wiring.go` | `cmd.start` / `spv.Initialize` | `walletClosedChain`, `spvChain`, `pairedFlagEnabled`, `metricsRecorder`, `network` | `→ subscribeReservationWalletClosed() → startup catch-up scans → staleDepositWatcher.Run() → actionTimeoutWatcher.Run() → misconfig self-check goroutine` | 3 watcher loops + self-check |
| `maintainer.Initialize` | `pkg/maintainer/maintainer.go:14` | `cmd.maintainers` | `config` | `→ btcdiff.Initialize() → spv.Initialize()` | Maintainer process |
| `btcdiff.Initialize` | `pkg/maintainer/btcdiff/bitcoin_difficulty.go:51` | `maintainer.Initialize` | `config` | `→ startControlLoop()` | Difficulty maintainer loop |
| `clientinfo.Initialize` | `pkg/clientinfo/clientinfo.go` | `cmd.start` / `cmd.maintainers` | `cfg` `(operator-config)` | `→ clientinfo.NewRegistry() → EnableServer(port)` | Binds the `/metrics` HTTP port (pprof removed) |
| `checkabi.main` | `pkg/chain/ethereum/tbtc/gen/checkabi/main.go` | Makefile / developer CLI | `os.Args` | `→ json.Marshal() → fmt.Println()` | Canonical JSON to stdout |

### Configuration Flags & Environment Variables

| Flag / Setting | Default | Bounds / Validation | Purpose & Operational Impact |
|---|---|---|---|
| `--tbtc.walletTxSatPerVByteFloor` | `5 sat/vByte` | Integer >= 0; a non-zero floor below `ProtocolMinWalletTxSatPerVByteFee` (5) hard-fails startup (`pkg/tbtc/tbtc.go:211`) | Minimum fee rate for wallet Bitcoin txs; sets the leader floor and the follower warn threshold |
| `--tbtc.walletTxFeeBufferPercent` | `25%` | Integer >= 0 | Percentage buffer over the raw estimate |
| `--tbtc.reservationsEnabled` / `--maintainer.spv.reservationProofsEnabled` | `false` / `false` | Both must be set; mispairing warns in-process and now fails the out-of-band deploy check `scripts/check-reservation-flags.sh` (`bc9bb78fe`) | The dual opt-in gate for the whole reservation feature |
| `--tbtc.transactionMonitor.*` | as in first run | unchanged | Stuck-tx monitoring (node-clock based alerts) |
| `--spv.maxProofHeaders` | `144` | Integer > 0 (0 normalizes to 144) | Bounds consecutive DIFF1 header absorption in SPV proofs |
| `--spv.historyDepth`, `--spv.transactionLimit`, `--spv.restartBackoffTime`, `--spv.idleBackoffTime` | unchanged | > 0 (validated at startup, `pkg/maintainer/spv/config.go:124-152`) | Maintainer loop tuning |
| `clientinfo.EnablePprof` | REMOVED | The setting and `/debug/pprof/*` endpoints were deleted (`d8dacd993`); `docs/profiling.md` deleted with them | No more runtime profiling surface on the metrics port |
