// Command doccheck validates maintained documentation without changing the source tree.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	result := checkRepository(*root)
	if flag.NArg() != 0 {
		result.Findings = append(result.Findings, finding{Rule: "ARGUMENTS", Detail: "Unexpected positional arguments."})
	}
	result.Status = "PASS"
	if len(result.Findings) != 0 {
		result.Status = "FAIL"
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Documentation report serialization failed.")
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if result.Status != "PASS" {
		os.Exit(1)
	}
}
