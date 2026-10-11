package config_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/config"
)

func TestAlternatesJSONRoundTrip(t *testing.T) {
	in := config.Context{Name: "dc", URL: "https://a.example", Alternates: []string{"https://b.example", "https://c.example"}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var got config.Context
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != in.Name || got.URL != in.URL || len(got.Alternates) != 2 {
		t.Fatalf("round trip = %+v want %+v", got, in)
	}
	for i, u := range in.Alternates {
		if got.Alternates[i] != u {
			t.Fatalf("alternate %d = %q want %q", i, got.Alternates[i], u)
		}
	}
}

func TestMarshalOmitsAlternatesWhenEmpty(t *testing.T) {
	raw, err := json.Marshal(config.Context{Name: "dc", URL: "https://a.example"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "alternates") {
		t.Fatalf("marshal without alternates must omit key: %s", raw)
	}
}

func TestValidateRejectsFTPAlternate(t *testing.T) {
	c := config.Context{Name: "dc", URL: "https://a.example", Alternates: []string{"ftp://x"}}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "ftp://x") {
		t.Fatalf("validate alternate ftp = %v, want error mentioning URL", err)
	}
}

func TestAddTrimsTrailingSlashAlternates(t *testing.T) {
	f := &config.File{}
	in := config.Context{Name: "dc", URL: "https://a.example/", Alternates: []string{"https://b.example/"}}
	if err := f.Add(in); err != nil {
		t.Fatal(err)
	}
	got := f.Contexts["dc"]
	if len(got.Alternates) != 1 || got.Alternates[0] != "https://b.example" {
		t.Fatalf("alternates = %q want [https://b.example]", got.Alternates)
	}
	if got.URL != "https://a.example" {
		t.Fatalf("url = %q want https://a.example", got.URL)
	}
}
