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
		Version     string `json:"version"`
		Permissions struct {
			Library struct {
				Filesystem bool `json:"filesystem"`
			} `json:"library"`
		} `json:"permissions"`
		Config struct {
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
	if m.Permissions.Library.Filesystem {
		t.Fatal("plugin must not request music-library filesystem access")
	}
	if m.Version != Version {
		t.Fatal("manifest and runtime versions differ")
	}
	if m.Config.UI.Type != "VerticalLayout" || len(m.Config.UI.Elements) != 3 {
		t.Fatal("configuration needs information outside its tabs")
	}
	if m.Config.UI.Elements[0].Type != "Group" || m.Config.UI.Elements[0].Label != "Info" || m.Config.UI.Elements[1].Type != "Categorization" || m.Config.UI.Elements[2].Label != "Library access" {
		t.Fatal("generated file and access information must surround the tabs")
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
	for _, label := range []string{"General", "Releases", "Notifications", "Calendar"} {
		if !labels[label] {
			t.Fatal("missing settings category", label)
		}
	}
	for _, removed := range []string{"username", "contact", "export_library_id", "include_secondary"} {
		if _, exists := m.Config.Schema.Properties[removed]; exists {
			t.Fatal("removed field remains", removed)
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
	for _, option := range append(append([]ReleaseTypeOption{}, PrimaryReleaseTypes...), SecondaryReleaseTypes...) {
		if m.Config.Schema.Properties[option.Key].Type != "boolean" {
			t.Fatal("release selector missing from manifest", option.Key)
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
