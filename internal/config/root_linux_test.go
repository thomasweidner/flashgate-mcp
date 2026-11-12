//go:build linux

package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestLinuxMountProjection(t *testing.T) {
	const base = "10 9 8:1 / / rw - ext4 /dev/root rw\n"
	for _, test := range []struct {
		name, entries, source, root string
		rejected                    bool
	}{
		{"separate", "", "/home/operator/config.toml", "/home/operator/data", false},
		{"bind directory root", "11 10 8:1 /home/operator/private /data rw - ext4 /dev/root rw\n", "/home/operator/private/config.toml", "/data", true},
		{"bind file below root", "11 10 8:1 /home/operator/config.toml /data/alias.toml rw - ext4 /dev/root rw\n", "/home/operator/config.toml", "/data", true},
		{"source is bind alias", "11 10 8:1 /data/config.toml /operator/config.toml rw - ext4 /dev/root rw\n", "/operator/config.toml", "/data", true},
		{"unrelated submount", "11 10 8:1 /other /data/other rw - ext4 /dev/root rw\n", "/home/operator/config.toml", "/data", false},
		{"escaped mountpoint", "11 10 8:1 /home/operator /data/with\\040space rw - ext4 /dev/root rw\n", "/home/operator/config.toml", "/data", true},
		{"unprovable remote subtree", "11 10 8:2 / /data/network rw - cifs //host/share rw\n", "/home/operator/config.toml", "/data", true},
		{"unprovable overlay", "11 10 0:1 / /data rw - overlay overlay rw\n", "/home/operator/config.toml", "/data", true},
		{"ambiguous stacked source", "11 10 8:2 / /home rw - ext4 /dev/other rw\n12 10 8:3 / /home rw - ext4 /dev/other rw\n", "/home/operator/config.toml", "/data", true},
		{"subvolume alias", "11 10 0:25 / /volume rw - btrfs /dev/volume rw\n12 10 0:26 /home /data/mounted rw - btrfs /dev/volume rw\n", "/volume/home/config.toml", "/data", true},
		{"invalid table", "malformed\n", "/home/operator/config.toml", "/data", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkLinuxMounts(strings.NewReader(base+test.entries), test.source, test.root)
			if errors.Is(err, security.ErrProtectedPath) != test.rejected {
				t.Fatalf("isolation result = %v, want rejection %t", err, test.rejected)
			}
		})
	}
}
