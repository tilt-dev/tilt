package types

import (
	"encoding/json"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/ocischema"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ImageManifest contains info to output for a manifest object.
type ImageManifest struct {
	Ref        *SerializableNamed
	Descriptor ocispec.Descriptor
	Raw        []byte `json:",omitempty"`

	// SchemaV2Manifest is used for inspection
	SchemaV2Manifest *schema2.DeserializedManifest `json:",omitempty"`
	// OCIManifest is used for inspection
	OCIManifest *ocischema.DeserializedManifest `json:",omitempty"`
}

// OCIPlatform creates an OCI platform from a manifest list platform spec
func OCIPlatform(ps *manifestlist.PlatformSpec) *ocispec.Platform {
	if ps == nil {
		return nil
	}
	return &ocispec.Platform{
		Architecture: ps.Architecture,
		OS:           ps.OS,
		OSVersion:    ps.OSVersion,
		OSFeatures:   ps.OSFeatures,
		Variant:      ps.Variant,
	}
}

// PlatformSpecFromOCI creates a platform spec from OCI platform
func PlatformSpecFromOCI(p *ocispec.Platform) *manifestlist.PlatformSpec {
	if p == nil {
		return nil
	}
	return &manifestlist.PlatformSpec{
		Architecture: p.Architecture,
		OS:           p.OS,
		OSVersion:    p.OSVersion,
		OSFeatures:   p.OSFeatures,
		Variant:      p.Variant,
	}
}

// Blobs returns the digests for all the blobs referenced by this manifest
func (i ImageManifest) Blobs() []digest.Digest {
	var digests []digest.Digest
	switch {
	case i.SchemaV2Manifest != nil:
		refs := i.SchemaV2Manifest.References()
		digests = make([]digest.Digest, 0, len(refs))
		for _, descriptor := range refs {
			digests = append(digests, descriptor.Digest)
		}
	case i.OCIManifest != nil:
		refs := i.OCIManifest.References()
		digests = make([]digest.Digest, 0, len(refs))
		for _, descriptor := range refs {
			digests = append(digests, descriptor.Digest)
		}
	}
	return digests
}

// Payload returns the media type and bytes for the manifest
func (i ImageManifest) Payload() (string, []byte, error) {
	// TODO: If available, read content from a content store by digest
	switch {
	case i.SchemaV2Manifest != nil:
		return i.SchemaV2Manifest.Payload()
	case i.OCIManifest != nil:
		return i.OCIManifest.Payload()
	default:
		return "", nil, errors.Errorf("%s has no payload", i.Ref)
	}
}

// References implements the distribution.Manifest interface. It delegates to
// the underlying manifest.
func (i ImageManifest) References() []distribution.Descriptor {
	switch {
	case i.SchemaV2Manifest != nil:
		return i.SchemaV2Manifest.References()
	case i.OCIManifest != nil:
		return i.OCIManifest.References()
	default:
		return nil
	}
}

// NewImageManifest returns a new ImageManifest object. The values for Platform
// are initialized from those in the image
func NewImageManifest(ref reference.Named, desc ocispec.Descriptor, manifest *schema2.DeserializedManifest) ImageManifest {
	raw, err := manifest.MarshalJSON()
	if err != nil {
		raw = nil
	}

	return ImageManifest{
		Ref:              &SerializableNamed{Named: ref},
		Descriptor:       desc,
		Raw:              raw,
		SchemaV2Manifest: manifest,
	}
}

// NewOCIImageManifest returns a new ImageManifest object. The values for
// Platform are initialized from those in the image
func NewOCIImageManifest(ref reference.Named, desc ocispec.Descriptor, manifest *ocischema.DeserializedManifest) ImageManifest {
	raw, err := manifest.MarshalJSON()
	if err != nil {
		raw = nil
	}

	return ImageManifest{
		Ref:         &SerializableNamed{Named: ref},
		Descriptor:  desc,
		Raw:         raw,
		OCIManifest: manifest,
	}
}

// SerializableNamed is a reference.Named that can be serialized and deserialized
// from JSON
type SerializableNamed struct {
	reference.Named
}

// UnmarshalJSON loads the Named reference from JSON bytes
func (s *SerializableNamed) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return errors.Wrapf(err, "invalid named reference bytes: %s", b)
	}
	var err error
	s.Named, err = reference.ParseNamed(raw)
	return err
}

// MarshalJSON returns the JSON bytes representation
func (s *SerializableNamed) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
