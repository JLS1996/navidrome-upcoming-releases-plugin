package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type uiElement struct {
	Type    string `json:"type"`
	Scope   string `json:"scope"`
	Label   string `json:"label"`
	Options struct {
		Format string `json:"format"`
	} `json:"options"`
	Elements []uiElement `json:"elements"`
}

// Navidrome's AJV validator rejects format=password. Its UI renderer expects
// masking in the control's options, independently from validation constraints.
func TestManifestCredentialControlsRenderWithoutUnknownFormats(t *testing.T) {
	var m struct {
		Version string `json:"version"`
		Config  struct {
			Schema struct {
				Properties map[string]struct {
					Type   string `json:"type"`
					Format string `json:"format"`
				} `json:"properties"`
			} `json:"schema"`
			UI uiElement `json:"uiSchema"`
		} `json:"config"`
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != Version {
		t.Fatal("manifest and runtime versions differ")
	}
	if m.Config.UI.Type != "Categorization" {
		t.Fatal("configuration needs clearly categorized tabs")
	}
	controls := map[string]string{}
	labels := map[string]bool{}
	var walk func(uiElement)
	walk = func(node uiElement) {
		switch node.Type {
		case "Control":
			if _, exists := controls[node.Scope]; exists {
				t.Fatal("duplicate control", node.Scope)
			}
			controls[node.Scope] = node.Options.Format
		case "Category":
			labels[node.Label] = true
		case "Categorization", "Group", "VerticalLayout", "Label":
		default:
			t.Fatal("unsupported UI layout", node.Type)
		}
		for _, child := range node.Elements {
			walk(child)
		}
	}
	walk(m.Config.UI)
	for _, label := range []string{"Library", "Releases", "Local files", "Notifications", "Calendar"} {
		if !labels[label] {
			t.Fatal("missing settings category", label)
		}
	}
	for name, p := range m.Config.Schema.Properties {
		if p.Format != "" {
			t.Fatalf("property %s uses a format that must be validated against Navidrome's AJV first", name)
		}
		if _, ok := controls["#/properties/"+name]; !ok {
			t.Fatal("field is missing from configuration form", name)
		}
	}
	for _, name := range []string{"ntfy_access_token", "caldav_app_password", "google_client_secret", "google_refresh_token"} {
		if m.Config.Schema.Properties[name].Type != "string" || controls["#/properties/"+name] != "password" {
			t.Fatal("credential must stay a masked string control", name)
		}
	}
	if len(controls) != len(m.Config.Schema.Properties) {
		t.Fatal("configuration form contains an unknown field")
	}
}
