package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type runtimeTarget struct {
	os   string
	arch string
}

type runtimePackage struct {
	Dir        string
	Name       string
	DepOnly    bool
	Standard   bool
	Incomplete bool
	GoFiles    []string
	SFiles     []string
}

// Match the four CGO-free release configurations. Product roles currently
// share one entry point. Development and test entry points are outside it.
func checkRuntime(root string) []finding {
	var findings []finding
	for _, target := range []runtimeTarget{
		{"windows", "amd64"}, {"windows", "arm64"},
		{"linux", "amd64"}, {"linux", "arm64"},
	} {
		findings = append(findings, checkRuntimeTarget(root, target)...)
	}
	return findings
}

func checkRuntimeTarget(root string, target runtimeTarget) []finding {
	label := target.os + "/" + target.arch
	graphFailure := finding{Rule: "RUNTIME_GRAPH", Detail: label + ": cannot resolve a complete product dependency graph."}
	root, err := filepath.Abs(root)
	if err != nil {
		return []finding{graphFailure}
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return []finding{graphFailure}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-mod=vendor", "-deps",
		"-json=Dir,Name,DepOnly,Standard,Incomplete,GoFiles,SFiles", "./cmd/server")
	command.Dir = root
	command.Env = append(command.Environ(),
		"GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0",
		"GOAMD64=v1", "GOARM64=v8.0", "GOEXPERIMENT=", "GOFLAGS=",
		"GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off")
	output := limitBuffer{limit: 8 * 1024 * 1024}
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		return []finding{graphFailure}
	}

	var findings []finding
	mainPackages := 0
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		var pkg runtimePackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil || pkg.Incomplete {
			return []finding{graphFailure}
		}
		if !pkg.DepOnly && pkg.Name == "main" {
			mainPackages++
		}
		if pkg.Standard {
			continue
		}
		if len(pkg.SFiles) != 0 {
			findings = append(findings, finding{Rule: "RUNTIME_SOURCE", Detail: label + ": product assembly needs a reviewed boundary."})
		}
		for _, file := range pkg.GoFiles {
			findings = append(findings, inspectRuntimeFile(root, filepath.Join(pkg.Dir, file), label)...)
		}
	}
	if mainPackages != 1 {
		return []finding{graphFailure}
	}
	return findings
}

func inspectRuntimeFile(root, file, target string) []finding {
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		return []finding{{Rule: "RUNTIME_SOURCE", Detail: target + ": cannot resolve product source."}}
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return []finding{{Rule: "RUNTIME_SOURCE", Detail: target + ": product source resolves outside the checkout."}}
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return []finding{{Path: filepath.ToSlash(rel), Rule: "RUNTIME_SOURCE", Detail: target + ": product source is not a regular file."}}
	}
	data, err := readDocument(resolved)
	if err != nil {
		return []finding{{Path: filepath.ToSlash(rel), Rule: "RUNTIME_SOURCE", Detail: target + ": cannot read product source."}}
	}
	return inspectRuntimeSource(filepath.ToSlash(rel), target, data)
}

func inspectRuntimeSource(name, target string, source []byte) []finding {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, name, source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return []finding{{Path: name, Rule: "RUNTIME_SOURCE", Detail: target + ": cannot parse product source."}}
	}
	var findings []finding
	add := func(pos token.Pos, detail string) {
		findings = append(findings, finding{
			Path: name, Line: files.Position(pos).Line, Rule: "RUNTIME_LAUNCH",
			Detail: target + ": " + detail,
		})
	}
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			add(spec.Pos(), "Cannot resolve product import.")
			continue
		}
		switch importPath {
		case "os/exec", "golang.org/x/sys/execabs", "plugin", "C":
			add(spec.Pos(), "Product import requires a reviewed execution or loading boundary: "+importPath)
		case "os", "syscall", "golang.org/x/sys/unix", "golang.org/x/sys/windows":
			qualifier := path.Base(importPath)
			if spec.Name != nil {
				qualifier = spec.Name.Name
			}
			if qualifier == "." {
				add(spec.Pos(), "Dot import hides sensitive API use: "+importPath)
			} else {
				imports[qualifier] = importPath
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		importPath := imports[qualifier.Name]
		if sensitiveRuntimeAPI(importPath, selector.Sel.Name) {
			add(selector.Pos(), fmt.Sprintf("Product API requires a reviewed execution or loading boundary: %s.%s", importPath, selector.Sel.Name))
		}
		return true
	})
	for _, group := range file.Comments {
		for _, comment := range group.List {
			fields := strings.Fields(comment.Text)
			if len(fields) != 0 && fields[0] == "//go:linkname" {
				add(comment.Pos(), "Linker alias can bypass the reviewed source boundary.")
			}
		}
	}
	return findings
}

func sensitiveRuntimeAPI(importPath, name string) bool {
	if importPath == "os" {
		return name == "StartProcess"
	}
	if importPath == "" {
		return false
	}
	switch name {
	case "Exec", "ForkExec", "StartProcess", "WinExec",
		"LoadLibrary", "LoadLibraryEx", "LoadDLL", "MustLoadDLL",
		"NewLazyDLL", "NewLazySystemDLL", "GetProcAddress":
		return true
	}
	for _, prefix := range []string{"CreateProcess", "ShellExecute", "Syscall", "RawSyscall"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
