package manifest

import "testing"

func TestResolveArgsDoesNotReparseInsertedJSON(t *testing.T) {
	cmd := Command{
		Args:   []string{"prioritize", "--dataset-json", "{params.dataset_json}"},
		Params: []string{"dataset_json"},
	}

	args, err := cmd.ResolveArgs(map[string]string{
		"dataset_json": `{"artifact_type":"dataset.raw.v1","tables":{"clients":[{"id":"1"}]}}`,
	}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveArgs returned error: %v", err)
	}
	if got := args[2]; got != `{"artifact_type":"dataset.raw.v1","tables":{"clients":[{"id":"1"}]}}` {
		t.Fatalf("unexpected dataset json arg: %s", got)
	}
}
