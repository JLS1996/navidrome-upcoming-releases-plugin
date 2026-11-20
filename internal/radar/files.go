package radar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Load(dir string) (State, error) {
	s := State{Schema: 1}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	s = State{} // An existing file must explicitly identify its schema.
	if err = json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("state.json is corrupt; preserve it for recovery before resetting: %w", err)
	}
	if s.Schema != 1 {
		return s, fmt.Errorf("unsupported state schema %d", s.Schema)
	}
	for i := range s.Entries {
		e := &s.Entries[i]
		// Recover entries written by versions that exhausted artist retries
		// during a temporary outage; preserve pages and committed releases.
		status := 0
		if suffix, ok := strings.CutPrefix(e.Problem, "MusicBrainz returned HTTP "); ok {
			if fields := strings.Fields(strings.ReplaceAll(suffix, ";", " ")); len(fields) > 0 {
				status, _ = strconv.Atoi(fields[0])
			}
		}
		if transientStatus(status) || strings.HasPrefix(e.Problem, "MusicBrainz request failed:") {
			if e.Phase == "error" {
				e.Phase = "match"
				if e.ResolvedMBID != "" {
					e.Phase = "releases"
				}
			}
			e.Failures, e.RetryAt, e.Problem = 0, 0, ""
		}
	}
	for _, e := range s.Entries {
		switch e.Phase {
		case "match", "releases", "done", "error", "unresolved":
		default:
			return s, fmt.Errorf("state.json contains an invalid artist phase")
		}
		if e.Offset < 0 || e.Offset > 50000 {
			return s, fmt.Errorf("state.json contains an invalid pagination offset")
		}
	}
	return s, nil
}
func AtomicWrite(dir, name string, b []byte) error { return atomicWrite(dir, name, b, 0600) }
func ExportReport(dir string, b []byte) error {
	return atomicWrite(dir, "upcoming-releases.html", b, 0644)
}
func atomicWrite(dir, name string, b []byte, mode os.FileMode) error {
	// Fixed filenames only: configuration and metadata never become filesystem paths.
	if name != "state.json" && name != "upcoming-releases.html" && name != "music-releases.ics" && name != "integrations.json" {
		return fmt.Errorf("unsupported output file")
	}
	tmp := filepath.Join(dir, name+".tmp")
	// Remove a stale temporary file without following links; exclusive create
	// prevents a raced symlink from redirecting writes to a music file.
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}
func Save(dir string, s State, c Config, now time.Time) error {
	html, err := Render(s, c, now)
	if err != nil {
		return err
	}
	if err = AtomicWrite(dir, "music-releases.ics", CalendarICS(Events(s, c, c.EventNamespace(), now, true), now)); err != nil {
		return err
	}
	s.RenderedAt = now.Unix()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Publish before committing state, so a failed page write will be retried.
	if err = AtomicWrite(dir, "upcoming-releases.html", html); err != nil {
		return err
	}
	return AtomicWrite(dir, "state.json", b)
}

func ExportCalendar(dir string, b []byte) error {
	return atomicWrite(dir, "music-releases.ics", b, 0644)
}
func LoadDeliveries(dir string) (DeliveryState, error) {
	d := DeliveryState{Schema: 1}
	b, err := os.ReadFile(filepath.Join(dir, "integrations.json"))
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	d = DeliveryState{}
	if json.Unmarshal(b, &d) != nil || d.Schema != 1 {
		return d, fmt.Errorf("integration state is corrupt or unsupported; preserve integrations.json before recovery")
	}
	for _, record := range d.Remote {
		if record == nil {
			return d, fmt.Errorf("integration state contains an invalid event record")
		}
		switch record.Phase {
		case "", "get", "write":
		default:
			return d, fmt.Errorf("integration state contains an invalid event phase")
		}
	}
	return d, nil
}
func SaveDeliveries(dir string, d DeliveryState) error {
	d.Schema = 1
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return AtomicWrite(dir, "integrations.json", b)
}
