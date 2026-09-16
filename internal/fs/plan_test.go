package fs

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type planFileSystem struct {
	calls  []string
	failAt string
}

func (f *planFileSystem) List(string) ([]Entry, error)              { panic("unexpected List") }
func (f *planFileSystem) Read(string, int64) ([]byte, error)        { panic("unexpected Read") }
func (f *planFileSystem) Stat(string) (Metadata, error)             { panic("unexpected Stat") }
func (f *planFileSystem) Write(path string, _ []byte, _ bool) error { return f.record("write:" + path) }
func (f *planFileSystem) Mkdir(path string) (bool, error)           { return true, f.record("mkdir:" + path) }
func (f *planFileSystem) Delete(path string, _ bool) error          { return f.record("delete:" + path) }
func (f *planFileSystem) Move(source, target string, _ bool) error {
	return f.record("move:" + source + ":" + target)
}
func (f *planFileSystem) Copy(source, target string, _ bool) error {
	return f.record("copy:" + source + ":" + target)
}
func (f *planFileSystem) record(call string) error {
	f.calls = append(f.calls, call)
	if call == f.failAt {
		return ErrNotFound
	}
	return nil
}

func TestPlanExecutorExecutesClosedOperationSet(t *testing.T) {
	filesystem := &planFileSystem{}
	executor, err := NewPlanExecutor(filesystem, 5)
	if err != nil {
		t.Fatal(err)
	}
	operations := []PlanOperation{{Kind: PlanCreateDirectory, Path: "work"}, {Kind: PlanWriteFile, Path: "work/a", Content: []byte("a")}, {Kind: PlanCopyPath, Source: "work/a", Target: "work/b"}, {Kind: PlanMovePath, Source: "work/b", Target: "work/c"}, {Kind: PlanDeletePath, Path: "work/c"}}
	results, err := executor.Execute(context.Background(), operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(operations) || !results[0].Created {
		t.Fatalf("unexpected results: %#v", results)
	}
	want := []string{"mkdir:work", "write:work/a", "copy:work/a:work/b", "move:work/b:work/c", "delete:work/c"}
	if !reflect.DeepEqual(filesystem.calls, want) {
		t.Fatalf("calls = %#v, want %#v", filesystem.calls, want)
	}
}

func TestPlanExecutorPrevalidatesBeforeMutation(t *testing.T) {
	filesystem := &planFileSystem{}
	executor, _ := NewPlanExecutor(filesystem, 2)
	for _, operations := range [][]PlanOperation{nil, {{Kind: PlanWriteFile, Path: "a"}, {Kind: "shell", Path: "b"}}, {{Kind: PlanWriteFile, Path: "a"}, {Kind: PlanDeletePath, Path: "b"}, {Kind: PlanDeletePath, Path: "c"}}, {{Kind: PlanCopyPath, Source: "a", Target: "b", Recursive: true}}} {
		if _, err := executor.Execute(context.Background(), operations); !errors.Is(err, ErrInvalidPlan) {
			t.Fatalf("error = %v", err)
		}
		if len(filesystem.calls) != 0 {
			t.Fatalf("invalid plan mutated filesystem: %#v", filesystem.calls)
		}
	}
}

func TestPlanExecutorStopsAndReportsPartialCompletion(t *testing.T) {
	filesystem := &planFileSystem{failAt: "write:b"}
	executor, _ := NewPlanExecutor(filesystem, 3)
	results, err := executor.Execute(context.Background(), []PlanOperation{{Kind: PlanWriteFile, Path: "a"}, {Kind: PlanWriteFile, Path: "b"}, {Kind: PlanWriteFile, Path: "c"}})
	var executionError *PlanExecutionError
	if !errors.As(err, &executionError) || executionError.Index != 1 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %#v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if want := []string{"write:a", "write:b"}; !reflect.DeepEqual(filesystem.calls, want) {
		t.Fatalf("calls = %#v", filesystem.calls)
	}
}

func TestPlanExecutorHonorsCancellationBetweenOperations(t *testing.T) {
	filesystem := &planFileSystem{}
	executor, _ := NewPlanExecutor(filesystem, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := executor.Execute(ctx, []PlanOperation{{Kind: PlanDeletePath, Path: "a"}})
	if !errors.Is(err, context.Canceled) || len(filesystem.calls) != 0 {
		t.Fatalf("error=%v calls=%#v", err, filesystem.calls)
	}
}
