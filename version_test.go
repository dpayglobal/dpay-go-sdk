package dpay

import (
	"regexp"
	"strings"
	"testing"
)

func TestVersionIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version = %q, want semver", Version)
	}
}

func TestUserAgent(t *testing.T) {
	got := userAgent()
	if !strings.HasPrefix(got, "dpay-go-sdk/"+Version+" go/go") {
		t.Fatalf("userAgent() = %q", got)
	}
}
