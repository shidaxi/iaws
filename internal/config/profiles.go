package config

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DefaultRegions is a common list for region selection.
var DefaultRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"eu-west-1", "eu-west-2", "eu-central-1", "ap-northeast-1",
	"ap-northeast-2", "ap-southeast-1", "ap-southeast-2", "ap-south-1",
}

// RegionsWithPreferredFirst returns a region list with preferred (e.g. profile's region) first, then DefaultRegions without duplicate.
func RegionsWithPreferredFirst(preferred string) []string {
	seen := make(map[string]bool)
	var out []string
	if preferred != "" {
		seen[preferred] = true
		out = append(out, preferred)
	}
	for _, r := range DefaultRegions {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// profilesFromReader parses config content and returns profile names (including "default").
func profilesFromReader(r io.Reader) ([]string, error) {
	var names []string
	seen := map[string]bool{}
	seen["default"] = true
	names = append(names, "default")
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[profile ") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "[profile "), "]")
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names, sc.Err()
}

// ProfilesFromConfig reads ~/.aws/config and returns profile names (including "default").
// Recent profiles (from SaveRecentProfile) are placed first.
func ProfilesFromConfig() ([]string, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".aws", "config")
	f, err := os.Open(path)
	if err != nil {
		return []string{"default"}, nil
	}
	defer f.Close()
	profiles, err := profilesFromReader(f)
	if err != nil {
		return profiles, err
	}
	recent := loadRecentProfiles()
	if len(recent) == 0 {
		return profiles, nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, r := range recent {
		seen[r] = true
		out = append(out, r)
	}
	for _, p := range profiles {
		if !seen[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

const (
	recentProfilesDir  = ".config/iaws"
	recentProfilesFile = "recent_profiles.json"
	maxRecentProfiles  = 5
)

func recentProfilesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, recentProfilesDir, recentProfilesFile)
}

func loadRecentProfiles() []string {
	b, err := os.ReadFile(recentProfilesPath())
	if err != nil {
		return nil
	}
	var profiles []string
	if json.Unmarshal(b, &profiles) != nil {
		return nil
	}
	return profiles
}

// SaveRecentProfile records a profile as recently used, keeping the list short.
func SaveRecentProfile(profile string) {
	recent := loadRecentProfiles()
	var out []string
	out = append(out, profile)
	for _, p := range recent {
		if p != profile {
			out = append(out, p)
		}
	}
	if len(out) > maxRecentProfiles {
		out = out[:maxRecentProfiles]
	}
	dir := filepath.Dir(recentProfilesPath())
	os.MkdirAll(dir, 0755)
	b, _ := json.Marshal(out)
	os.WriteFile(recentProfilesPath(), b, 0644)
}
