package config

import (
	"strings"
	"testing"
)

func TestProfilesFromReader(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   []string
	}{
		{
			name:   "empty",
			config: "",
			want:   []string{"default"},
		},
		{
			name: "single profile",
			config: `[profile foo]
region = us-east-1
`,
			want: []string{"default", "foo"},
		},
		{
			name: "multiple profiles",
			config: `[profile a]
region = us-east-1
[profile b]
region = eu-west-1
[profile c]
region = ap-northeast-1
`,
			want: []string{"default", "a", "b", "c"},
		},
		{
			name: "duplicate profile",
			config: `[profile x]
[profile x]
`,
			want: []string{"default", "x"},
		},
		{
			name: "default section not parsed as profile",
			config: `[default]
region = us-east-1
[profile myprofile]
`,
			want: []string{"default", "myprofile"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := profilesFromReader(strings.NewReader(tt.config))
			if err != nil {
				t.Fatalf("profilesFromReader: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Errorf("len(got) = %d, want %d; got %v", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestProfilesFromConfig(t *testing.T) {
	// don't modify real home dir; just verify no error and at least contains default
	got, err := ProfilesFromConfig()
	if err != nil {
		t.Fatalf("ProfilesFromConfig: %v", err)
	}
	if len(got) < 1 {
		t.Fatal("expected at least one profile")
	}
	if got[0] != "default" {
		t.Errorf("first profile = %q, want \"default\"", got[0])
	}
}

func TestDefaultRegions(t *testing.T) {
	if len(DefaultRegions) == 0 {
		t.Fatal("DefaultRegions should not be empty")
	}
	seen := make(map[string]bool)
	for _, r := range DefaultRegions {
		if seen[r] {
			t.Errorf("duplicate region %q", r)
		}
		seen[r] = true
	}
}
