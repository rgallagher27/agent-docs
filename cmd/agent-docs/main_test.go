package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantStdout  string
		wantErr     bool
		errContains string
	}{
		{name: "no args prints usage", args: nil, wantStdout: "Usage:"},
		{name: "help prints usage", args: []string{"help"}, wantStdout: "Usage:"},
		{name: "--help prints usage", args: []string{"--help"}, wantStdout: "Usage:"},
		{name: "-h prints usage", args: []string{"-h"}, wantStdout: "Usage:"},
		{name: "version prints version", args: []string{"version"}, wantStdout: "dev"},
		{name: "--version prints version", args: []string{"--version"}, wantStdout: "dev"},
		{name: "-v prints version", args: []string{"-v"}, wantStdout: "dev"},
		{name: "unknown command errors", args: []string{"banana"}, wantErr: true, errContains: "unknown command"},
		{name: "serve is not implemented yet", args: []string{"serve"}, wantErr: true, errContains: "not implemented"},
		{name: "fetch is not implemented yet", args: []string{"fetch"}, wantErr: true, errContains: "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(tt.args, &stdout, &stderr)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
				t.Errorf("err = %v, want containing %q", err, tt.errContains)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want containing %q", stdout.String(), tt.wantStdout)
			}
		})
	}
}
