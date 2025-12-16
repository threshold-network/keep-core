# tBTC Beta Staker Operator Consolidation
## Communication to Node Operators

**Date**: 2025-10-10
**Status**: WalletRegistry Audit Submitted
**Project**: Consolidation from 18 to 3 Operators

---

## Executive Summary

The Threshold DAO is consolidating beta staker operators from **18 nodes to 3 nodes** (1 per provider) to achieve 83% operational cost reduction while maintaining continuous tBTC protocol operation. This communication outlines what you need to know and do during this transition.

**Key Point**: Your cooperation during this 6-8 week process is essential for a smooth, zero-downtime transition.

---

## What's Changing

### Before Consolidation
- **18 active operators** across 3 providers (BOAR, STAKED, P2P)
- Each provider runs 6 redundant nodes
- All operators participate in DKG (Distributed Key Generation)
- All operators can coordinate wallet actions

### After Consolidation
- **3 active operators** (1 per provider)
- 15 operators decommissioned
- Only active operators participate in DKG
- Cost reduction: 83% (~$X annual savings for DAO)

---

## Your Operator Status

### ✅ ACTIVE OPERATORS (Keep Running)

If your operator address is listed below, **your node will remain active**:

| Provider | Operator Address (Keep Active) |
|----------|--------------------------------|
| **STAKED** | `0xf401aae8c639eb1638fd99b90eae8a4c54f9894d` |
| **P2P** | `0xb074a3b960f29a1448a2dd4de95210ca492c18d4` |
| **BOAR** | `0xffb804c2de78576ad011f68a7df63d739b8c8155` |

**Action Required**: Continue running your node throughout the entire consolidation process.

---

### ❌ DEPRECATED OPERATORS (To Be Decommissioned)

If your operator is **NOT** in the active list above, it will be decommissioned during this consolidation.

**15 total operators will be deprecated** (5 from BOAR, 5 from STAKED, 5 from P2P).

**Your Actions**:
1. Keep your node running until your wallet(s) are fully drained (0 BTC)
2. Monitor your wallet BTC balance (dashboard will be provided)
3. Be available for potential manual sweep coordination (see below)
4. Wait for confirmation from Threshold team before decommissioning
5. Decommission node only after explicit approval

---

## Timeline & Key Dates

### Phase 0: Smart Contract Audit & Testing (Current Phase)
**Dates**: 2025-10-10 → ~2025-10-27 (~2 weeks)

- ✅ **Today (2025-10-10)**: WalletRegistry bytecode optimization sent for audit
- ✅ **Already Complete**: Allowlist contract audited
- **Next**: Sepolia testnet deployment for extensive testing
- **Then**: Mainnet deployment preparation

**Operator Action**: None - continue normal operations

---

### Phase 1: Mainnet Deployment & Weight Updates
**Dates**: ~2025-10-27 → ~2025-10-31 (5 days)

**What Happens**:
1. Allowlist contract deployed to Ethereum mainnet
2. Optimized WalletRegistry deployed via upgradeable proxy
3. **Weight updates executed**:
   - 3 active operators: weight = 100% (full participation)
   - 15 deprecated operators: weight = 0% (excluded from new wallets)
4. DKG reconfiguration triggered (only 3 operators in new groups)

**Operator Action**:
- **Active Operators**: Monitor DKG participation
- **Deprecated Operators**: Confirm you are excluded from new DKG groups (expected behavior)

---

### Phase 2: Natural Wallet Draining
**Dates**: ~2025-10-31 → ~2025-12-01 (4 weeks)

**What Happens**:
- BTC automatically leaves deprecated wallets through **natural redemptions**
- Bridge prioritizes oldest wallets first (beta staker wallets)
- Monitoring dashboard tracks draining progress in real-time

**Operator Action**:
- **All Operators**: Monitor your wallet BTC balances via provided dashboard
- **Deprecated Operators**: Watch for your wallets approaching 0 BTC
- **Active Operators**: Continue normal operations

**No manual intervention required in this phase** (unless volumes are low)

---

### Phase 3: Assessment & Potential Manual Sweeps
**Dates**: ~2025-12-01 → ~2025-12-12 (~2 weeks)

**Week 4 Assessment**:
- Evaluate draining progress (target: >50% BTC drained)
- Analyze redemption volume trends
- **Decision**: Continue natural draining OR trigger manual sweeps

**If Manual Sweeps Are Triggered**:

You may be asked to participate in **manual BTC wallet sweeps** if:
- Your wallet retains >20% balance after 4 weeks, OR
- October deadline is <3 weeks away

**Manual Sweep Process** (if needed):

1. **Preparation**:
   - Threshold team identifies stragglers (wallets still holding significant BTC)
   - Calculate Bitcoin transaction fees (~$50-200 per sweep)
   - Construct MovingFunds proposal parameters

2. **Coordination Call**:
   - Scheduled coordination window (every ~3 hours / 900 blocks)
   - **51 out of 100 operators must be online** for threshold signing
   - Confirm your availability during coordination window

3. **Execution** (WHERE BTC MOVES):
   ```
   Leader operator proposes MovingFunds action
     ↓
   51+ operators participate in threshold signing (RFC-12 coordination)
     ↓
   Bitcoin transaction constructed:
     - Input: Deprecated wallet's BTC
     - Output: Active wallet's BTC address (same provider)
     - Fee: Current mempool rate (e.g., 10 sat/vB)
     ↓
   Transaction broadcast to Bitcoin network
     ↓
   Wait 6 confirmations (~1 hour)
     ↓
   BTC now in active wallet
   ```

4. **Verification**:
   - SPV proof submitted to Ethereum Bridge
   - Deprecated wallet marked as drained
   - Operator becomes eligible for removal

**Operator Action (if triggered)**:
- **Deprecated Operators**: Be available for threshold signing coordination
- **Active Operators**: May need to participate in threshold signing as part of the 51/100 requirement

**Cost**: Bitcoin fees (~$50-200) + Ethereum gas (~$100-300) per sweep, paid by DAO treasury

---

### Phase 4: Progressive Operator Removal
**Dates**: ~2025-12-12 → ~2025-12-31 (3 weeks)

**What Happens**:
- Operators removed **progressively** as their wallets reach 0 BTC (not all at once)
- Each operator must have 0 BTC for **1+ week** before removal (safety buffer)
- Three removal batches: Week 6, Week 7, Week 8

**Removal Protocol**:
1. Verify wallet at 0 BTC for 1+ week
2. Confirm no pending coordination actions
3. Threshold team notifies provider
4. Provider confirms readiness
5. Governance removes operator from allowlist
6. Provider decommissions node infrastructure

**Operator Action**:
- **Deprecated Operators**:
  - Monitor your wallet balance (provided dashboard)
  - Wait for explicit decommissioning approval from Threshold team
  - **DO NOT** decommission your node until receiving confirmation
  - Once approved, shut down node and confirm to Threshold team
- **Active Operators**: Continue normal operations, monitor stability

**Batched Removal Schedule**:
- **Week 6** (~2025-12-12): First batch (operators with 0 BTC confirmed 1+ week)
- **Week 7** (~2025-12-19): Second batch (additional drained operators)
- **Week 8** (~2025-12-26): Final cleanup (remaining operators, edge cases)

---

## What You Need to Do

### Immediate Actions (This Week)

1. **Verify Your Operator Status**:
   - Check if your operator address is in the "Active" or "Deprecated" list above
   - Contact Threshold team if unclear

2. **Ensure Contact Information is Current**:
   - Verify you're in the operator coordination channels (Slack/Discord)
   - Update email and emergency contact info

3. **Review Node Infrastructure**:
   - Ensure nodes are healthy and monitoring is operational
   - Verify you can access node logs and status

---

### Ongoing Actions (Throughout Consolidation)

#### For ACTIVE Operators:

1. **Keep Your Node Running**:
   - Maintain 99.5%+ uptime throughout consolidation
   - Monitor DKG participation

2. **Monitor Coordination Windows**:
   - RFC-12 coordination with 3 operators (down from 18)
   - Report any coordination issues immediately

3. **Be Available for Manual Sweeps** (if triggered):
   - Respond to coordination requests
   - Participate in threshold signing (51/100 requirement)

---

#### For DEPRECATED Operators:

1. **Keep Your Node Running** (until approved for decommissioning):
   - **Critical**: Do NOT shut down prematurely
   - Your node may be needed for manual sweeps
   - Your wallet holds valuable BTC that must be drained first

2. **Monitor Your Wallet Balance**:
   - Watch BTC balance decrease over time
   - Alert Threshold team if balance stagnates for >2 weeks

3. **Be Available for Manual Sweep Coordination** (if needed):
   - Check coordination channel daily during Weeks 4-5
   - Respond within 4 hours to coordination requests
   - Ensure your node can participate in threshold signing

4. **Wait for Decommissioning Approval**:
   - You will receive explicit confirmation when:
     - Your wallet is at 0 BTC for 1+ week
     - Operator removal transaction is executed
     - Safe to shut down node
   - **DO NOT** decommission based on 0 BTC alone - wait for team confirmation

5. **Decommission When Approved**:
   - Shut down node infrastructure
   - Confirm decommissioning to Threshold team
   - Provide final node status report

---

## Critical Information

### No T Token Movement Required

**Important**: This consolidation does NOT involve T token staking or movement.

- Original system required T tokens staked via TokenStaking contract
- Post-TIP-92/TIP-100: Allowlist uses **weight-based authorization** (0-100 weights)
- Setting `weight = 0` is just a state change, NOT a fund transfer
- **You do NOT need to move, unstake, or manage any T tokens**

Only **Bitcoin (BTC)** moves during this process (from deprecated wallets to active wallets).

---

### Bitcoin Movement - What Actually Happens

#### Scenario 1: Natural Draining (Preferred)

**No operator coordination needed** - happens automatically:

```
User redeems tBTC
  ↓
Bridge selects oldest wallet (deprecated beta staker wallet)
  ↓
BTC leaves deprecated wallet → goes to user's Bitcoin address
  ↓
Wallet drains naturally over 4-7 weeks
```

**Operators do nothing** - just monitor BTC balance decreasing via dashboard.

---

#### Scenario 2: Manual Sweeps (Fallback)

**Operator coordination IS required** - BTC moved between wallets.

**Per-Provider Bitcoin Movements**:

| Provider | From (Deprecated Wallets) | To (Active Wallet) | BTC Amount |
|----------|---------------------------|-------------------|------------|
| BOAR | 5 old wallet addresses | 0xffb8...8155's BTC wallet | Sum of 5 wallets' BTC |
| STAKED | 5 old wallet addresses | 0xf401...9894d's BTC wallet | Sum of 5 wallets' BTC |
| P2P | 5 old wallet addresses | 0xb074...18d4's BTC wallet | Sum of 5 wallets' BTC |

**Important**: Each provider sweeps their own deprecated wallets to their own active wallet. Your BTC stays within your provider organization.

---

### Threshold Signing Requirements

For manual sweeps to succeed, **51 out of 100 operators must be online simultaneously**.

**With 3 providers × 6 operators each = 18 operators total**:
- Need **~10 operators online** at the same time (51% of 18)
- Each provider should have **3-4 operators available minimum**

**Coordination Windows**:
- Every **900 blocks** (~3 hours on Ethereum)
- If operators miss window → retry next window (3 hours later)

**Your Commitment**: When manual sweep coordination is scheduled, please ensure your node is online and responsive.

---

### Weekly Status Reports

Starting Week 3, you'll receive **weekly updates** with:
- Overall consolidation progress (% BTC drained)
- Your specific wallet status
- Upcoming milestones and actions
- Any coordination requests

---

## Communication Channels

### Primary Channels

1. **Operator Coordination Channel** (Slack/Discord):
   - Daily updates during critical phases
   - Manual sweep coordination requests
   - Emergency communications

2. **Updates**:
   - Weekly status reports
   - Important milestone announcements
   - Decommissioning approvals

3. **Emergency Contact**:
   - Critical issues requiring immediate response
   - Node coordination failures
   - Unexpected BTC balance changes

**Please ensure you're subscribed to all channels and check them daily during Weeks 4-8.**

---

## Frequently Asked Questions (FAQ)

### Q: Do I need to move or unstake T tokens?
**A**: No. This consolidation uses weight-based authorization. No T token movement is required.

---

### Q: When exactly should I shut down my deprecated node?
**A**: Only after you receive **explicit written confirmation** from the Threshold team that:
1. Your wallet has been at 0 BTC for 1+ week
2. Operator removal transaction has been executed on-chain
3. You are approved to decommission

**Do not shut down based on 0 BTC balance alone.**

---

### Q: What happens if I'm unavailable during a manual sweep window?
**A**: The sweep will retry in the next coordination window (~3 hours later). However, repeated unavailability may delay the consolidation timeline. Please make best efforts to be available during Weeks 4-5 if manual sweeps are triggered.

---

### Q: Will I receive compensation for participating in manual sweeps?
**A**: Manual sweep costs (Bitcoin fees + Ethereum gas) are paid by the DAO treasury, not individual operators. No additional compensation is provided beyond your standard node operation agreement.

---

### Q: What if my wallet balance doesn't drain naturally?
**A**: The monitoring dashboard will alert the Threshold team if your wallet isn't draining after 2 weeks. We'll work with you to coordinate manual sweeps during Week 4-5 assessment.

---

### Q: Can I decommission my node early if I see 0 BTC balance?
**A**: **No.** You must wait for explicit approval from Threshold team. The 1-week safety buffer at 0 BTC ensures no pending transactions. Early decommissioning could cause coordination issues.

---

### Q: Will there be any downtime during the consolidation?
**A**: **No.** The entire process is designed for **zero-downtime** tBTC service. Users will not experience any interruption in minting or redemptions.

---

### Q: How will I know if manual sweeps are triggered?
**A**: You'll receive:
1. Notification from Threshold team
2. Message in operator coordination channel
3. Calendar invitation for coordination window
4. Dashboard alert

You'll have at least **48 hours notice** before manual sweep coordination.

---

### Q: What happens if DKG fails with only 3 operators?
**A**: RFC-12 coordination has been battle-tested for 21+ months and supports 3-operator configurations. If any issues arise, we can temporarily rollback weight updates to include more operators. This is a low-probability risk with a proven mitigation plan.

---

## Support & Contact

### Technical Support
- **Email**: [support email - TBD]
- **Slack/Discord**: TBD
- **Emergency**: [emergency contact - TBD]

### Project Manager
- **Name**: [PM name - TBD]
- **Email**: [PM email - TBD]
- **Availability**: Monday-Friday, 9am-5pm UTC

---

## Next Steps for You

### This Week (2025-10-10 → 2025-10-17)

- [ ] Verify your operator status (active or deprecated)
- [ ] Confirm you're in all communication channels
- [ ] Review node health and monitoring
- [ ] Update contact information if needed
- [ ] Acknowledge receipt of this communication (reply to coordinator)

### Week of 2025-10-17

- [ ] Wait for audit completion announcement
- [ ] Monitor for Sepolia testnet deployment
- [ ] Review testnet results (if shared)

### Week of 2025-10-27 (Mainnet Deployment)

- [ ] Monitor for mainnet deployment announcement
- [ ] Verify weight updates on-chain
- [ ] **Active Operators**: Confirm DKG participation increases
- [ ] **Deprecated Operators**: Confirm exclusion from new DKG groups

### Weeks of 2025-10-31 → 2025-12-01 (Natural Draining)

- [ ] Access monitoring dashboard (link will be provided)
- [ ] Monitor your wallet BTC balance weekly
- [ ] Report any anomalies (balance increase, stagnation >2 weeks)

### Week of 2025-12-01 (Assessment)

- [ ] Review Week 4 assessment results
- [ ] **If manual sweeps triggered**: Respond to coordination requests within 4 hours
- [ ] **If natural draining continues**: Continue monitoring

### Weeks of 2025-12-12 → 2025-12-31 (Operator Removal)

- [ ] **Deprecated Operators**: Monitor for decommissioning approval
- [ ] **Deprecated Operators**: Decommission only after explicit confirmation
- [ ] **Active Operators**: Monitor coordination stability with reduced operator set

---

## Summary: Critical Dates & Actions

| Date Range | Phase | Your Action |
|------------|-------|-------------|
| **2025-10-10 → 2025-10-27** | Audit & Testing | None - continue normal operations |
| **2025-10-27 → 2025-10-31** | Mainnet Deployment | Monitor DKG participation changes |
| **2025-10-31 → 2025-12-01** | Natural Draining | Monitor wallet balance via dashboard |
| **2025-12-01 → 2025-12-12** | Assessment & Sweeps | Be available for manual sweep coordination (if triggered) |
| **2025-12-12 → 2025-12-31** | Operator Removal | **Deprecated**: Wait for decommissioning approval; **Active**: Continue operations |

---

**Document Version**: 1.0
**Last Updated**: 2025-10-10
**Next Update**: After audit/testing completion (~2025-10-27)
