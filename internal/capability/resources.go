package capability

// AppliesToFilesystemRoot identifies the capabilities reviewed for a named
// filesystem operand. Eligibility conveys no grant or tool availability.
func AppliesToFilesystemRoot(id ID) bool {
	switch id {
	case FilesRead, FilesWrite, SearchRun:
		return true
	default:
		return false
	}
}
