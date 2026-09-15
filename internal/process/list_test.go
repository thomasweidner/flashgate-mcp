package process

import (
	"context"
	"testing"
)

func TestLocalListerReturnsSortedBoundedFields(t *testing.T) {
	entries, err := (LocalLister{}).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least the test process")
	}
	for index, entry := range entries {
		if entry.PID == 0 || entry.Name == "" {
			t.Fatalf("entry %d has an empty portable field: %+v", index, entry)
		}
		if index > 0 && entries[index-1].PID >= entry.PID {
			t.Fatalf("entries are not strictly PID ordered: %+v then %+v", entries[index-1], entry)
		}
	}
}

func TestLocalTreeObserverReturnsSortedPortableSnapshot(t *testing.T) {
	snapshot, err := (LocalTreeObserver{}).Tree(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Processes) == 0 {
		t.Fatal("expected at least the test process")
	}
	for index, entry := range snapshot.Processes {
		if entry.PID == 0 || entry.Name == "" || entry.ThreadCount == 0 {
			t.Fatalf("entry %d has missing portable metadata: %+v", index, entry)
		}
		if index > 0 && snapshot.Processes[index-1].PID >= entry.PID {
			t.Fatalf("entries are not strictly PID ordered: %+v then %+v", snapshot.Processes[index-1], entry)
		}
	}
}
