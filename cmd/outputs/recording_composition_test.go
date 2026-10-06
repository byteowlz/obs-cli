package outputs

import (
	"strings"
	"testing"
)

func TestRecordingSessionCompositionFlags(t *testing.T) {
	cmd := newRecordingSessionCommand()
	for _, name := range []string{"screen", "composition"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil || flag.DefValue != "" || flag.Value.Type() != "string" {
			t.Fatalf("optional %s flag missing or has unexpected default", name)
		}
	}
	if err := cmd.ParseFlags([]string{"--screen", "00000000-0000-0000-0000-000000000001", "--composition", "16:9"}); err != nil {
		t.Fatal(err)
	}
	mode, _ := cmd.Flags().GetString("composition")
	if mode != "16:9" {
		t.Fatal("composition flag not parsed")
	}
}

// Invalid flag values are rejected by the pure preflight before the nil client
// can be used. These tests cannot establish a connection to real OBS.
func TestRecordingSessionInvalidFlagsNeverReachOBS(t *testing.T) {
	for _, tc := range []struct {
		flags     []string
		errorText string
	}{
		{[]string{"--screen", "not-a-uuid"}, "--screen must be"},
		{[]string{"--composition", "stretch"}, "composition must be"},
	} {
		cmd := newRecordingSessionCommand()
		if err := cmd.ParseFlags(tc.flags); err != nil {
			t.Fatal(err)
		}
		err := cmd.RunE(cmd, []string{"demo"})
		if err == nil || !strings.Contains(err.Error(), tc.errorText) {
			t.Fatalf("flag preflight: %v", err)
		}
	}
}
