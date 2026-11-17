package config

import (
	"github.com/pelletier/go-toml/v2"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

// sizeValue distinguishes absent optional policy fields from invalid zero.
// It consumes both TOML integer and string values through the same parser.
type sizeValue int64

func (s *sizeValue) UnmarshalTOML(raw []byte) error {
	var value struct {
		Size any `toml:"size"`
	}
	if err := toml.Unmarshal(append([]byte("size = "), raw...), &value); err != nil {
		return err
	}
	n, err := resource.ParseSize(value.Size)
	if err != nil {
		return err
	}
	*s = sizeValue(n)
	return nil
}

type resourceFields struct {
	MaxFileSize   *sizeValue `toml:"max_file_size"`
	MaxWriteSize  *sizeValue `toml:"max_write_size"`
	MaxScanSize   *sizeValue `toml:"max_scan_size"`
	MaxResultSize *sizeValue `toml:"max_result_size"`
	MaxTempSize   *sizeValue `toml:"max_temp_size"`
}

func (f filesFields) limits() resource.Limits {
	return resource.Limits{File: int64(f.MaxFileSize), Write: int64(f.MaxWriteSize), Scan: int64(f.MaxScanSize), Result: int64(f.MaxResultSize), Temp: int64(f.MaxTempSize)}
}

func (r resourceFields) resolve(base, global resource.Limits) (resource.Limits, error) {
	fields := [...]struct {
		src *sizeValue
		dst *int64
	}{
		{r.MaxFileSize, &base.File}, {r.MaxWriteSize, &base.Write},
		{r.MaxScanSize, &base.Scan}, {r.MaxResultSize, &base.Result}, {r.MaxTempSize, &base.Temp},
	}
	for _, f := range fields {
		if f.src != nil {
			*f.dst = int64(*f.src)
		}
	}
	if !base.Within(global) {
		return resource.Limits{}, resource.ErrLimitExceeded
	}
	return base, nil
}
