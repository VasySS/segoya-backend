package main

import "testing"

func TestValidateCommand(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		command string
		args    []string
		valid   bool
	}{
		{name: "bundled import", command: playAreasImportFlag, valid: true},
		{name: "bundled verification", command: playAreasVerifyFlag, valid: true},
		{name: "backfill", command: playAreasBackfillFlag, valid: true},
		{name: "up", command: upFlag, valid: true},
		{name: "up to version", command: upToFlag, args: []string{"12"}, valid: true},
		{name: "removed download", command: "play-areas-download"},
		{name: "import rejects directory", command: playAreasImportFlag, args: []string{"boundaries"}},
		{name: "verify rejects directory", command: playAreasVerifyFlag, args: []string{"boundaries"}},
		{name: "backfill rejects arguments", command: playAreasBackfillFlag, args: []string{"extra"}},
		{name: "unknown command", command: "typo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := validateCommand(tc.command, tc.args); (err == nil) != tc.valid {
				t.Errorf("validate %s %v: error=%v, expected valid=%t", tc.command, tc.args, err, tc.valid)
			}
		})
	}
}
