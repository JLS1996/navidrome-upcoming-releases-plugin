package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
			UI struct {
				Type     string `json:"type"`
				Elements []struct {
					Type    string `json:"type"`
					Scope   string `json:"scope"`
					Options struct {
						Format string `json:"format"`
					} `json:"options"`
				} `json:"elements"`
			} `json:"uiSchema"`
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
	if m.Config.UI.Type != "VerticalLayout" {
		t.Fatal("configuration controls need a supported root layout")
	}
	controls := map[string]string{}
	for _, control := range m.Config.UI.Elements {
		if control.Type != "Control" {
			t.Fatal("unsupported configuration control")
		}
		if _, exists := controls[control.Scope]; exists {
			t.Fatal("duplicate configuration control", control.Scope)
		}
		controls[control.Scope] = control.Options.Format
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
