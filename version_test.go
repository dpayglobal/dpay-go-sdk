package dpay

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVersionIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) {
		t.Fatalf("Version = %q, want semver", Version)
	}
}

func TestVersionIsTheLatestChangelogRelease(t *testing.T) {
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	latest := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\]`).FindSubmatch(changelog)
	if latest == nil || string(latest[1]) != Version {
		t.Fatalf("Version = %q, the latest CHANGELOG.md release is %q", Version, latest)
	}
}

func TestUserAgent(t *testing.T) {
	got := userAgent()
	if !strings.HasPrefix(got, "dpay-go-sdk/"+Version+" go/go") {
		t.Fatalf("userAgent() = %q", got)
	}
}
