package config

import "testing"

func TestFilterKnownStorageNames(t *testing.T) {
	known := map[string]struct{}{
		"local": {},
		"s3":    {},
	}

	got := filterKnownStorageNames([]string{"local", "missing", "s3", "local", ""}, known)

	if len(got) != 2 || got[0] != "local" || got[1] != "s3" {
		t.Fatalf("expected known storage names to remain once, got %#v", got)
	}
}
