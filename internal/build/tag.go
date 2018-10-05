package build

// The image tag prefix can be customized.
//
// This allows our integration tests to customize
// the prefix so that they can write to a public
// registry without interfering with each other.
var ImageTagPrefix = "tilt-"
