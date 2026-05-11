package sabio

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type qaFixture struct {
	Version       int              `json:"version"`
	Framework     string           `json:"framework"`
	Profile       string           `json:"profile"`
	DB            string           `json:"db"`
	Conversations []qaConversation `json:"conversations"`
}

type qaConversation struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Turns []qaTurn `json:"turns"`
}

type qaTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func TestCobranzaChileQAFixtureContract(t *testing.T) {
	data, err := os.ReadFile("data/qa_cobranza_chile_ideal.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture qaFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 2 {
		t.Fatalf("unexpected fixture version: %d", fixture.Version)
	}
	if fixture.Framework != "sabio" {
		t.Fatalf("unexpected framework: %s", fixture.Framework)
	}
	if fixture.Profile != "cobranza-chile" {
		t.Fatalf("unexpected profile: %s", fixture.Profile)
	}
	if fixture.DB == "" {
		t.Fatal("missing db field")
	}
	if len(fixture.Conversations) == 0 {
		t.Fatal("expected conversations")
	}
	seen := map[string]bool{}
	for _, conv := range fixture.Conversations {
		if strings.TrimSpace(conv.ID) == "" {
			t.Fatal("conversation without id")
		}
		if seen[conv.ID] {
			t.Fatalf("duplicate conversation id: %s", conv.ID)
		}
		seen[conv.ID] = true
		if len(conv.Turns) < 2 {
			t.Fatalf("%s must have at least one user+ideal pair", conv.ID)
		}
		for _, turn := range conv.Turns {
			if turn.Role != "user" && turn.Role != "ideal" {
				t.Fatalf("%s has unknown role: %s", conv.ID, turn.Role)
			}
			if strings.TrimSpace(turn.Content) == "" {
				t.Fatalf("%s has empty content for role %s", conv.ID, turn.Role)
			}
		}
	}
}

func TestSabioContractDoesNotAdvertiseLegacyBM25(t *testing.T) {
	paths := []string{"AXIOMS.md", "framework.manifest.json"}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ToLower(string(data))
		for _, forbidden := range []string{"bm25", "semantic_legacy", "indexa.store.v1", "vector_store"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s still advertises legacy path %q", path, forbidden)
			}
		}
	}
}
