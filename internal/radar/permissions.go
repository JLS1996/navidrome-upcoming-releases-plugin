package radar

import (
	"encoding/json"
	"fmt"
	"sort"
)

// AccessibleLibraries intersects plugin grants with the user's current grants.
// Call before using cached data, not only during daily artist discovery.
func AccessibleLibraries(raw string, granted []int) ([]int, error) {
	var envelope struct {
		Response struct {
			Status  string `json:"status"`
			Folders struct {
				Items []struct {
					ID int `json:"id"`
				} `json:"musicFolder"`
			} `json:"musicFolders"`
		} `json:"subsonic-response"`
	}
	if json.Unmarshal([]byte(raw), &envelope) != nil || envelope.Response.Status != "ok" {
		return nil, fmt.Errorf("unable to verify the configured user's current library access")
	}
	allowed := map[int]bool{}
	for _, f := range envelope.Response.Folders.Items {
		allowed[f.ID] = true
	}
	out := []int{}
	seen := map[int]bool{}
	for _, id := range granted {
		if allowed[id] && !seen[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	sort.Ints(out)
	return out, nil
}
