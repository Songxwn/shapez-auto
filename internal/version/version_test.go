package version

import (
	"os"
	"strings"
	"testing"
)

func TestVersionSync(t *testing.T) {
	iss, err := os.ReadFile("../../installer/shapez-auto.iss")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(iss), "AppVersion="+Current) {
		t.Fatalf("installer version is not %s", Current)
	}
	js, err := os.ReadFile("../../assets/shapez-auto.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `version: "`+Current+`"`) {
		t.Fatalf("mod version is not %s", Current)
	}
}
