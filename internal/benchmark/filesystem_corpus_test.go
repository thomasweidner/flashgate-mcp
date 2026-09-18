package benchmark

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type filesystemCorpus struct {
	SchemaVersion string `json:"schemaVersion"`
	Seed          int64  `json:"seed"`
	Fixtures      struct {
		SmallFiles struct {
			Count        int `json:"count"`
			BytesPerFile int `json:"bytesPerFile"`
		} `json:"smallFiles"`
		LargeFiles []generatedCorpusFile `json:"largeFiles"`
		DeepTree   struct {
			Depth         int `json:"depth"`
			FilesPerLevel int `json:"filesPerLevel"`
		} `json:"deepTree"`
		WideTree struct {
			Directories       int `json:"directories"`
			FilesPerDirectory int `json:"filesPerDirectory"`
		} `json:"wideTree"`
		BinaryFiles []generatedCorpusFile `json:"binaryFiles"`
	} `json:"fixtures"`
	CrossVolumeCases []struct {
		Name                    string `json:"name"`
		Source                  string `json:"source"`
		Target                  string `json:"target"`
		RequiresDistinctVolumes bool   `json:"requiresDistinctVolumes"`
	} `json:"crossVolumeCases"`
}

type generatedCorpusFile struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	Content   string `json:"content"`
}

func TestRepresentativeFilesystemCorpusContract(t *testing.T) {
	path := filepath.Join("..", "..", "benchmarks", "filesystem-corpus.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read filesystem corpus: %v", err)
	}

	var corpus filesystemCorpus
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatalf("decode filesystem corpus: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("filesystem corpus contains trailing JSON value: %v", err)
	}

	if corpus.SchemaVersion != "flashgate-filesystem-corpus/v1" {
		t.Fatalf("schemaVersion=%q, want flashgate-filesystem-corpus/v1", corpus.SchemaVersion)
	}
	if corpus.Seed < 0 || corpus.Fixtures.SmallFiles.Count < 1 || corpus.Fixtures.SmallFiles.BytesPerFile < 1 {
		t.Fatal("small-file fixture and seed must be deterministic positive values")
	}
	if corpus.Fixtures.DeepTree.Depth < 16 || corpus.Fixtures.DeepTree.FilesPerLevel < 1 {
		t.Fatal("deep-tree fixture must exercise at least 16 levels")
	}
	if corpus.Fixtures.WideTree.Directories*corpus.Fixtures.WideTree.FilesPerDirectory < 500 {
		t.Fatal("wide-tree fixture must contain at least 500 files")
	}
	assertGeneratedCorpusFiles(t, "largeFiles", corpus.Fixtures.LargeFiles, 1024*1024)
	assertGeneratedCorpusFiles(t, "binaryFiles", corpus.Fixtures.BinaryFiles, 1)

	if len(corpus.CrossVolumeCases) < 2 {
		t.Fatal("corpus must define file and directory cross-volume cases")
	}
	crossVolumeNames := make(map[string]bool, len(corpus.CrossVolumeCases))
	for _, fixture := range corpus.CrossVolumeCases {
		if fixture.Name == "" || !fixture.RequiresDistinctVolumes {
			t.Fatalf("invalid cross-volume fixture: %+v", fixture)
		}
		if !strings.HasPrefix(fixture.Source, "primary/") || !strings.HasPrefix(fixture.Target, "secondary/") {
			t.Fatalf("cross-volume paths must bind distinct logical roots: %+v", fixture)
		}
		crossVolumeNames[fixture.Name] = true
	}
	if !crossVolumeNames["file-move"] || !crossVolumeNames["directory-move"] {
		t.Fatal("corpus must define both file-move and directory-move cross-volume cases")
	}
}

func assertGeneratedCorpusFiles(t *testing.T, name string, files []generatedCorpusFile, minimumSize int64) {
	t.Helper()
	if len(files) == 0 {
		t.Fatalf("%s must not be empty", name)
	}
	seen := make(map[string]struct{}, len(files))
	allowedContent := map[string]bool{
		"deterministic-text":             true,
		"deterministic-binary":           true,
		"repeated-byte-sequence-0-255":   true,
		"deterministic-binary-with-nuls": true,
	}
	for _, fixture := range files {
		if fixture.Name == "" || filepath.IsAbs(fixture.Name) || filepath.Base(fixture.Name) != fixture.Name {
			t.Fatalf("%s contains unsafe fixture name %q", name, fixture.Name)
		}
		if _, duplicate := seen[fixture.Name]; duplicate {
			t.Fatalf("%s contains duplicate fixture name %q", name, fixture.Name)
		}
		seen[fixture.Name] = struct{}{}
		if fixture.SizeBytes < minimumSize || !allowedContent[fixture.Content] {
			t.Fatalf("%s contains incomplete fixture %+v", name, fixture)
		}
	}
}
