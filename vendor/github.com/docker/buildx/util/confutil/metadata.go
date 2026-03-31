package confutil

import (
	"os"
	"strconv"
)

// MetadataProvenanceMode is the type for setting provenance in the metadata
// file
type MetadataProvenanceMode string

const (
	// MetadataProvenanceModeMin sets minimal provenance (default)
	MetadataProvenanceModeMin MetadataProvenanceMode = "min"
	// MetadataProvenanceModeMax sets full provenance
	MetadataProvenanceModeMax MetadataProvenanceMode = "max"
	// MetadataProvenanceModeDisabled doesn't set provenance
	MetadataProvenanceModeDisabled MetadataProvenanceMode = "disabled"
)

// MetadataProvenance returns the metadata provenance mode from
// BUILDX_METADATA_PROVENANCE environment variable
func MetadataProvenance() MetadataProvenanceMode {
	return ParseMetadataProvenance(os.Getenv("BUILDX_METADATA_PROVENANCE"))
}

// ParseMetadataProvenance parses the metadata provenance mode from a string
func ParseMetadataProvenance(inp string) MetadataProvenanceMode {
	switch inp {
	case "min":
		return MetadataProvenanceModeMin
	case "max":
		return MetadataProvenanceModeMax
	case "disabled":
		return MetadataProvenanceModeDisabled
	}
	if ok, err := strconv.ParseBool(inp); err == nil && !ok {
		return MetadataProvenanceModeDisabled
	}
	return MetadataProvenanceModeMin
}

// MetadataWarningsEnabled returns whether metadata warnings are enabled from
// BUILDX_METADATA_WARNINGS environment variable (default false)
func MetadataWarningsEnabled() bool {
	if ok, err := strconv.ParseBool(os.Getenv("BUILDX_METADATA_WARNINGS")); err == nil {
		return ok
	}
	return false
}
