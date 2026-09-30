package recipes

import (
  "testing"
)

func sampleWorkflow() map[string]any {
  return map[string]any{
    "1": map[string]any{
      "inputs": map[string]any{
        "text": "prompt",
        "steps": 20,
        "cfg": 7.0,
      },
      "class_type": "CLIPTextEncode",
    },
  }
}

func TestValidateVersionRequiresExposedMappings(t *testing.T) {
  wf := sampleWorkflow()
  err := validateVersion(VersionInput{
    Workflow: wf,
    InputMappings: map[string]string{},
    ExposedParameters: []Parameter{{Name: "prompt", Type: "string", Required: true, Path: "/1/inputs/text"}},
  })
  if err == nil {
    t.Fatal("expected unmapped exposed parameter error")
  }

  err = validateVersion(VersionInput{
    Workflow: wf,
    InputMappings: map[string]string{"prompt": "/1/inputs/text"},
    ExposedParameters: []Parameter{{Name: "prompt", Type: "string", Required: true, Path: "/1/inputs/text"}},
  })
  if err != nil {
    t.Fatalf("unexpected validation error: %v", err)
  }
}

func TestValidateVersionRejectsSecrets(t *testing.T) {
  wf := sampleWorkflow()
  wf["meta"] = map[string]any{"api_key": "secret"}
  err := validateVersion(VersionInput{Workflow: wf})
  if err == nil {
    t.Fatal("expected secret-like workflow key to be rejected")
  }
}

func TestSetPathAndWorkflowHashAreDeterministic(t *testing.T) {
  wf := sampleWorkflow()
  if err := setPath(wf, "/1/inputs/text", "new prompt"); err != nil {
    t.Fatalf("setPath failed: %v", err)
  }
  if wf["1"].(map[string]any)["inputs"].(map[string]any)["text"] != "new prompt" {
    t.Fatal("mapped value was not updated")
  }

  first, err := workflowHash(sampleWorkflow())
  if err != nil {
    t.Fatal(err)
  }
  second, err := workflowHash(sampleWorkflow())
  if err != nil {
    t.Fatal(err)
  }
  if first != second {
    t.Fatalf("hash is not deterministic: %s != %s", first, second)
  }
  if len(first) != 64 {
    t.Fatalf("expected sha256 hex length, got %d", len(first))
  }
}
