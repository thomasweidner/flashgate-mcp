package main

import "strings"

func checkCatalogDispositions(backlog string) []finding {
	const marker = "**BL-110 disposition.** "
	const contract = "docs/planning/implementation-contract.md#bl-110-effective-tool-catalog"
	var findings []finding
	for i, line := range documentLines(backlog, false) {
		fields := strings.SplitN(line, "|", 5)
		if len(fields) != 5 || fields[0] != "" {
			continue
		}
		id := strings.TrimSpace(fields[1])
		if !adapterOwnerID.MatchString(id) {
			continue
		}
		status := strings.Trim(strings.TrimSpace(fields[2]), "*")
		if status != "Planned" && status != "In Progress" && status != "Blocked" {
			continue
		}
		acceptance := stripInlineCode(fields[4])
		if id == "BL-110" && strings.Contains(acceptance, contract) {
			continue
		}
		_, disposition, found := strings.Cut(acceptance, marker)
		disposition = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(disposition), "|"))
		if !found || disposition == "" || strings.Count(acceptance, marker) != 1 {
			findings = append(findings, finding{
				Path: "BACKLOG.md", Line: i + 1, Rule: "CATALOG_DISPOSITION",
				Detail: "Expected one BL-110 disposition in acceptance for " + id,
			})
		}
	}
	return findings
}
