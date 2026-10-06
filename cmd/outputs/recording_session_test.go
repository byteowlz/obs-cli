package outputs

import (
	"bytes"
	"strings"
	"testing"
)

func TestSessionNameArgumentAndPrompt(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		input       string
		interactive bool
		want        string
		wantPrompt  bool
		wantError   bool
	}{
		{name: "argument", args: []string{"Take one"}, want: "Take one"},
		{name: "noninteractive missing", input: "demo\n", wantError: true},
		{name: "TTY prompt", input: "Take one\n", interactive: true, want: "Take one", wantPrompt: true},
		{name: "CRLF prompt", input: "demo\r\n", interactive: true, want: "demo", wantPrompt: true},
		{name: "EOF with name", input: "demo", interactive: true, want: "demo", wantPrompt: true},
		{name: "EOF empty", interactive: true, wantError: true, wantPrompt: true},
		{name: "blank prompt", input: "\n", interactive: true, wantError: true, wantPrompt: true},
		{name: "unsafe argument", args: []string{"../escape"}, wantError: true},
		{name: "unsafe prompt", input: "a/b\n", interactive: true, wantError: true, wantPrompt: true},
		{name: "long prompt", input: strings.Repeat("a", 500), interactive: true, wantError: true, wantPrompt: true},
		{name: "too many args", args: []string{"one", "two"}, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var prompt bytes.Buffer
			got, err := sessionName(tc.args, strings.NewReader(tc.input), &prompt, tc.interactive)
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("sessionName = %q, %v", got, err)
			}
			if (prompt.Len() > 0) != tc.wantPrompt {
				t.Fatalf("unexpected prompt: %q", prompt.String())
			}
		})
	}
}

func TestRecordingSessionRegistrationAndHelp(t *testing.T) {
	command, _, err := recordingCmd.Find([]string{"session"})
	if err != nil || command == nil || command.Name() != "session" {
		t.Fatalf("session subcommand not registered: %v", err)
	}
	cmd := newRecordingSessionCommand()
	if err := cmd.Args(cmd, []string{"one", "two"}); err == nil {
		t.Fatal("extra positional argument accepted")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Fatalf("omitted name rejected before TTY prompt: %v", err)
	}
	var help bytes.Buffer
	cmd.SetOut(&help)
	if err := cmd.Help(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"session [name]", "~/Movies", "TTY", "pause does not synchronize", "--config"} {
		if !strings.Contains(help.String(), text) {
			t.Fatalf("help missing %q: %s", text, help.String())
		}
	}
	if !strings.Contains(pauseRecordingCmd.Long, "not synchronized") {
		t.Fatal("pause warning missing")
	}
}
