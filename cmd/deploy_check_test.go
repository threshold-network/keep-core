package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckReservationFlagsScript exercises the deploy-check shell script
// against sample start and maintainer config files. The check flags
// mismatches where tbtc.reservationsEnabled is true in the start config but
// maintainer.spv.reservationProofsEnabled is false in the maintainer config
// (anchors would be proposed with no SPV proof submission, stranding them).
//
// The assertion that fails on the old code: no deploy-check existed, so
// operators had to manually diff the two config files to catch the
// misconfiguration. The check now exits non-zero on a start=true,
// maintainer=false mismatch.
func TestCheckReservationFlagsScript(t *testing.T) {
	scriptPath, err := filepath.Abs("../scripts/check-reservation-flags.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("deploy-check script not found at %s: %v", scriptPath, err)
	}

	// Each subtest writes a pair of start + maintainer config files, runs
	// the script, and asserts on the exit code and (for the error case)
	// the error message.
	cases := []struct {
		name           string
		startTOML      string
		maintainerTOML string
		wantExit       int
		wantInStderr   string // empty = no error message expected
	}{
		{
			name: "both enabled is OK",
			startTOML: `[tbtc]
reservationsEnabled = true
`,
			maintainerTOML: `[maintainer.spv]
reservationProofsEnabled = true
`,
			wantExit: 0,
		},
		{
			name: "both disabled is OK",
			startTOML: `[tbtc]
reservationsEnabled = false
`,
			maintainerTOML: `[maintainer.spv]
reservationProofsEnabled = false
`,
			wantExit: 0,
		},
		{
			name: "start enabled, maintainer disabled is flagged",
			startTOML: `[tbtc]
reservationsEnabled = true
`,
			maintainerTOML: `[maintainer.spv]
reservationProofsEnabled = false
`,
			wantExit:     1,
			wantInStderr: "inconsistent reservation flags",
		},
		{
			name: "start disabled, maintainer enabled is OK (valid split)",
			startTOML: `[tbtc]
reservationsEnabled = false
`,
			maintainerTOML: `[maintainer.spv]
reservationProofsEnabled = true
`,
			wantExit: 0,
		},
		{
			name: "missing maintainer flag defaults to false (OK if start off)",
			startTOML: `[tbtc]
reservationsEnabled = false
`,
			maintainerTOML: `[maintainer.spv]
historyDepth = 50400
`,
			wantExit: 0,
		},
		{
			name: "missing maintainer flag flagged when start enabled",
			startTOML: `[tbtc]
reservationsEnabled = true
`,
			maintainerTOML: `[maintainer.spv]
historyDepth = 50400
`,
			wantExit:     1,
			wantInStderr: "inconsistent reservation flags",
		},
		{
			name: "dotted keys and quoted values are read",
			startTOML: `tbtc.reservationsEnabled = "true"
`,
			maintainerTOML: `maintainer.spv.reservationProofsEnabled = false
`,
			wantExit:     1,
			wantInStderr: "inconsistent reservation flags",
		},
		{
			name: "trailing comments and key case are ignored",
			startTOML: `[tbtc]
ReservationsEnabled = true # enabled
`,
			maintainerTOML: `[maintainer.spv]
reservationProofsEnabled = true # enabled
`,
			wantExit: 0,
		},
		{
			name: "same key name in another table is not mistaken for the flag",
			startTOML: `[tbtc]
reservationsEnabled = true
`,
			maintainerTOML: `[maintainer.other]
reservationProofsEnabled = true
[maintainer.spv]
reservationProofsEnabled = false
`,
			wantExit:     1,
			wantInStderr: "inconsistent reservation flags",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			startConfig := filepath.Join(tmpDir, "start.conf")
			maintainerConfig := filepath.Join(tmpDir, "maintainer.conf")
			if err := os.WriteFile(startConfig, []byte(tc.startTOML), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(maintainerConfig, []byte(tc.maintainerTOML), 0644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command(scriptPath,
				"--start-config", startConfig,
				"--maintainer-config", maintainerConfig,
			)
			out, err := cmd.CombinedOutput()
			output := string(out)

			gotExit := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					gotExit = exitErr.ExitCode()
				} else {
					t.Fatalf("unexpected error: %v\noutput: %s", err, output)
				}
			}

			if gotExit != tc.wantExit {
				t.Errorf(
					"exit code mismatch: got %d, want %d\noutput: %s",
					gotExit, tc.wantExit, output,
				)
			}
			if tc.wantInStderr != "" && !strings.Contains(output, tc.wantInStderr) {
				t.Errorf(
					"output missing %q\noutput: %s",
					tc.wantInStderr, output,
				)
			}
		})
	}
}

// TestCheckReservationFlagsScript_MissingConfig verifies the script exits 3
// when a config file is missing.
func TestCheckReservationFlagsScript_MissingConfig(t *testing.T) {
	scriptPath, err := filepath.Abs("../scripts/check-reservation-flags.sh")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	startConfig := filepath.Join(tmpDir, "start.conf")
	if err := os.WriteFile(startConfig, []byte("[tbtc]\nreservationsEnabled = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	maintainerConfig := filepath.Join(tmpDir, "missing.conf") // does not exist

	cmd := exec.Command(scriptPath,
		"--start-config", startConfig,
		"--maintainer-config", maintainerConfig,
	)
	out, err := cmd.CombinedOutput()
	output := string(out)

	gotExit := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			// The script could not be executed at all (absent, not
			// executable, bad interpreter). Report that instead of letting
			// gotExit stay 0, which would otherwise look like a clean run.
			t.Fatalf("cannot run deploy-check script at %s: %v", scriptPath, err)
		}
		gotExit = exitErr.ExitCode()
	}

	if gotExit != 3 {
		t.Errorf("expected exit code 3 for missing config; got %d\noutput: %s",
			gotExit, output)
	}
	if !strings.Contains(output, "not found") {
		t.Errorf("output missing 'not found' message\noutput: %s", output)
	}
}
