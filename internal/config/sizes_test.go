package config

import (
	"strings"
	"testing"
)

func TestResourceSizeComposition(t *testing.T) {
	base := documentWithRoot(t, t.TempDir())
	doc := base + `max_file_size = '10 MiB'
max_write_size = '8 MiB'
max_scan_size = '10 MB'
max_result_size = '4 MiB'
max_temp_size = '2 MiB'
roots.work.policy = 'small'
roots.work.max_write_size = '2 MiB'
roots.work.max_temp_size = 1000
roots.other.path = '/other'
roots.other.max_file_size = 512
[policy.small]
max_file_size = '1 MiB'
max_write_size = '1 MiB'
max_scan_size = '2 MB'
max_result_size = '512 KiB'
max_temp_size = '1 KiB'
`
	// Use the same native absolute path for the second logical ID.
	doc = strings.Replace(doc, "path = '/other'", "path = "+strings.TrimSpace(strings.Split(base, "roots.work.path = ")[1]), 1)
	cfg, err := Decode([]byte(doc), Direct)
	if err != nil {
		t.Fatal(err)
	}
	roots := cfg.Files().Roots()
	if len(roots) != 2 {
		t.Fatal(roots)
	}
	l := roots[1].Limits
	if l.File != 1<<20 || l.Write != 2<<20 || l.Scan != 2000000 || l.Result != 512<<10 || l.Temp != 1000 {
		t.Fatal("composition", l)
	}
	if roots[0].Limits.File != 512 || roots[0].Limits.Write != 8<<20 {
		t.Fatal("independent root", roots[0])
	}
	roots[1].Limits.File = 9
	if cfg.Files().Roots()[1].Limits.File != 1<<20 {
		t.Fatal("mutable snapshot")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSizeFieldsRejectInvalidAndLegacy(t *testing.T) {
	base := documentWithRoot(t, t.TempDir())
	for _, field := range []string{"max_file_size", "max_write_size", "max_scan_size", "max_result_size", "max_temp_size"} {
		for _, prefix := range []string{"", "roots.work.", "[policy.unused]\n"} {
			for _, value := range []string{"0", "-1", "1.1", "true", "[]", "{}", "'0 B'", "'8,3 GB'", "'9223372036854775808'"} {
				if _, err := Decode([]byte(base+prefix+field+" = "+value+"\n"), Direct); err == nil {
					t.Fatalf("accepted %s%s=%s", prefix, field, value)
				}
			}
		}
		if _, err := Decode([]byte(base+"roots.work."+field+" = '32 MiB'\n"), Direct); err == nil {
			t.Fatal("root expanded", field)
		}
		if _, err := Decode([]byte(base+"[policy.unused]\n"+field+" = '32 MiB'\n"), Direct); err == nil {
			t.Fatal("unused policy expanded", field)
		}
	}
	for _, key := range []string{"max_write_bytes", "max_copy_bytes", "[protocol]\nmax_message_bytes", "[protocol]\nmax_argument_bytes", "[protocol]\nmax_response_bytes", "[logging]\nmax_record_bytes", "[endpoints]\nmax_frame_bytes"} {
		if _, err := Decode([]byte(base+key+" = 1024\n"), Direct); err == nil {
			t.Fatal("legacy alias", key)
		}
	}
	for _, settings := range []string{
		"[protocol]\nmax_message_size = '16 MiB'\nmax_argument_size = '12 MiB'\nmax_response_size = '16MiB'\n",
		"[logging]\nmax_record_size = '4 KiB'\n",
		"[endpoints]\nmax_frame_size = '16 MiB'\n",
	} {
		if _, err := Decode([]byte(base+settings), Direct); err != nil {
			t.Fatal(settings, err)
		}
	}
}
