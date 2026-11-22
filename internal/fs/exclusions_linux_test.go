package fs

import (
	"errors"
	"os"
	"testing"
)

func TestExclusionUnsupportedLinuxMount(t *testing.T) {
	t.Parallel()
	// procfs is a different mount with non-ordinary directory semantics.
	// Even an accessible directory there cannot establish an ext root proof.
	if _, err := nativeExclusionDirectory("/proc"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unproved mount was accepted: %v", err)
	}
}
