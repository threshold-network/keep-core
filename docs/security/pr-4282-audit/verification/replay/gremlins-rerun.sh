#!/usr/bin/env bash
# Re-run the 3 gremlins campaigns at coefficient 8 when host load permits.
# Sequential (one at a time) to reduce self-contention. GOCACHE on disk.
set -u
# kc-mutation was removed on 2026-10-15; recreate it first with
# git worktree add --detach <checkout> fe73e9799
cd <checkout>
export GOCACHE=<gocache> GOMODCACHE=<gomodcache>
run() { # $1=label $2=pkg $3=exclusions...
  local label=$1 pkg=$2; shift 2
  echo "[$(date +%T)] start $label" >> out/gremlins-rerun.log
  timeout 2700 gremlins unleash "$pkg" "$@" --workers 4 --timeout-coefficient 8 -o "out/gremlins_${label}_c8.json" >> out/gremlins-rerun.log 2>&1
  echo "[$(date +%T)] done $label rc=$?" >> out/gremlins-rerun.log
}
run spv pkg/maintainer/spv -E 'chain.go$' -E 'config.go$' -E 'deposit_sweep.go$' -E 'health.go$' -E 'moved_funds_sweep.go$' -E 'moving_funds.go$' -E 'redemptions.go$' -E 'spv.go$'
run pg pkg/tbtcpg -E 'chain.go$' -E 'deposit_sweep.go$' -E 'fee.go$' -E 'heartbeat.go$' -E 'moved_funds_sweep.go$' -E 'moving_funds.go$' -E 'redemptions.go$' -E 'tbtcpg.go$'
run tb pkg/tbtc -E 'chain.go$' -E 'coordination.go$' -E 'coordination_window_metrics.go$' -E 'deduplicator.go$' -E 'deposit.go$' -E 'deposit_reveal_lookup.go$' -E 'deposit_sweep.go$' -E 'dkg.go$' -E 'dkg_loop.go$' -E 'dkg_submit.go$' -E 'heartbeat.go$' -E 'inactivity.go$' -E 'marshaling.go$' -E 'moved_funds_sweep.go$' -E 'moving_funds.go$' -E 'node.go$' -E 'node_block.go$' -E 'node_coordination.go$' -E 'node_executors.go$' -E 'node_proposals.go$' -E 'node_wallet_closure.go$' -E 'parameters.go$' -E 'proposal_fee_check.go$' -E 'redemption.go$' -E 'registry.go$' -E 'reservation_target_age.go$' -E 'signing.go$' -E 'signing_done.go$' -E 'signing_loop.go$' -E 'tbtc.go$' -E 'transaction_monitor.go$' -E 'transaction_monitor_config.go$' -E 'wallet.go$'
echo "[$(date +%T)] all done" >> out/gremlins-rerun.log
