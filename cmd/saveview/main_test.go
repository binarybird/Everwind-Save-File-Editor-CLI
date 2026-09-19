package main

import "testing"

func TestParseSetArgs(t *testing.T) {
	tests := []struct {
		name                                   string
		args                                   []string
		wantFile, wantPath, wantValue, wantOut string
		wantErr                                bool
	}{
		{
			name:      "-o at the end (documented syntax)",
			args:      []string{"save.sav", "WorldName", "NewName", "-o", "out.sav"},
			wantFile:  "save.sav",
			wantPath:  "WorldName",
			wantValue: "NewName",
			wantOut:   "out.sav",
		},
		{
			name:      "-o before the positionals",
			args:      []string{"-o", "out.sav", "save.sav", "WorldName", "NewName"},
			wantFile:  "save.sav",
			wantPath:  "WorldName",
			wantValue: "NewName",
			wantOut:   "out.sav",
		},
		{
			name:      "-o in the middle",
			args:      []string{"save.sav", "-o", "out.sav", "WorldName", "NewName"},
			wantFile:  "save.sav",
			wantPath:  "WorldName",
			wantValue: "NewName",
			wantOut:   "out.sav",
		},
		{
			name:    "missing -o",
			args:    []string{"save.sav", "WorldName", "NewName"},
			wantErr: true,
		},
		{
			name:    "-o with no following value",
			args:    []string{"save.sav", "WorldName", "NewName", "-o"},
			wantErr: true,
		},
		{
			name:    "too few positional args",
			args:    []string{"save.sav", "WorldName", "-o", "out.sav"},
			wantErr: true,
		},
		{
			name:    "too many positional args",
			args:    []string{"save.sav", "WorldName", "NewName", "extra", "-o", "out.sav"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, path, value, out, err := parseSetArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSetArgs(%v): expected an error, got file=%q path=%q value=%q out=%q", tc.args, file, path, value, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSetArgs(%v): unexpected error: %v", tc.args, err)
			}
			if file != tc.wantFile || path != tc.wantPath || value != tc.wantValue || out != tc.wantOut {
				t.Errorf("parseSetArgs(%v) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
					tc.args, file, path, value, out, tc.wantFile, tc.wantPath, tc.wantValue, tc.wantOut)
			}
		})
	}
}

func TestParseJSONArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFile string
		wantOut  string
		wantErr  bool
	}{
		{
			name:     "file only, no -o",
			args:     []string{"save.sav"},
			wantFile: "save.sav",
			wantOut:  "",
		},
		{
			name:     "-o at the end",
			args:     []string{"save.sav", "-o", "out.json"},
			wantFile: "save.sav",
			wantOut:  "out.json",
		},
		{
			name:     "-o before the positional",
			args:     []string{"-o", "out.json", "save.sav"},
			wantFile: "save.sav",
			wantOut:  "out.json",
		},
		{
			name:    "-o with no following value",
			args:    []string{"save.sav", "-o"},
			wantErr: true,
		},
		{
			name:    "too many positional args",
			args:    []string{"save.sav", "extra"},
			wantErr: true,
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, out, err := parseJSONArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseJSONArgs(%v): expected an error, got file=%q out=%q", tc.args, file, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONArgs(%v): unexpected error: %v", tc.args, err)
			}
			if file != tc.wantFile || out != tc.wantOut {
				t.Errorf("parseJSONArgs(%v) = (%q, %q), want (%q, %q)", tc.args, file, out, tc.wantFile, tc.wantOut)
			}
		})
	}
}
