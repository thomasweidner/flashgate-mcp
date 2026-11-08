package benchmark

// Counters use wire and filesystem semantics documented in benchmarks/README.md.
type Counters struct {
	RequestBytes  uint64
	ResponseBytes uint64
	ResultBytes   uint64
	ReadBytes     uint64
	WrittenBytes  uint64
	ScannedBytes  uint64
	Entries       uint64
	Calls         int
}

func approxTokensBytes4(utf8Bytes uint64) uint64 {
	return utf8Bytes/4 + (utf8Bytes%4+3)/4
}
