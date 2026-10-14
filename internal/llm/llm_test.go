package llm

import "testing"

func TestModelsURL(t *testing.T) {
	got, err := ModelsURL("https://api.example.com/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.example.com/v1/models" {
		t.Fatalf("got %s", got)
	}
	got, err = ModelsURL("https://api.example.com/v1/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.example.com/v1/models" {
		t.Fatalf("got %s", got)
	}
	if _, err := ModelsURL("ftp://api.example.com/v1"); err == nil {
		t.Fatal("expected scheme error")
	}
}

func TestParseModelIDs(t *testing.T) {
	raw := []byte(`{"data":[{"id":"beta"},{"id":"alpha"},{"id":"alpha"},{"name":"gamma"}]}`)
	got, err := ParseModelIDs(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v", got)
		}
	}
	if _, err := ParseModelIDs([]byte(`{"models":[]}`)); err == nil {
		t.Fatal("expected empty list error")
	}
}
