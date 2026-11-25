package main

import "testing"

func TestCatalogDispositions(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  string
		want int
	}{
		{"planned", "| BL-112 | **Planned** | Compatibility | **BL-110 disposition.** Bind the negotiated revision. |", 0},
		{"history", "| BL-100 | Completed | Rights | Previous acceptance. |", 0},
		{"missing", "| BL-112 | **Blocked** | Compatibility | Track a client. |", 1},
		{"empty", "| BL-112 | **In Progress** | Compatibility | **BL-110 disposition.** |", 1},
		{"duplicate", "| BL-112 | **Planned** | Compatibility | **BL-110 disposition.** A. **BL-110 disposition.** B. |", 1},
		{"example", "| BL-112 | **Planned** | Compatibility | `**BL-110 disposition.** Example.` |", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if findings := checkCatalogDispositions(tc.row); len(findings) != tc.want {
				t.Fatalf("got %v, expected %d findings", findings, tc.want)
			}
		})
	}
}
