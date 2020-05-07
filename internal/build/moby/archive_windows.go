package moby

// Adapted from
// https://github.com/moby/moby/blob/ecb898dcb9065c8e9bcf7bb79fd160dea1c859b8/pkg/archive/archive_windows.go

/*
   Copyright 2013-2018 Docker, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"os"
)

// chmodTarEntry is used to adjust the file permissions used in tar header based
// on the platform the archival is done.
func ChmodTarEntry(perm os.FileMode) os.FileMode {
	// perm &= 0755 // this 0-ed out tar flags (like link, regular file, directory marker etc.)
	permPart := perm & os.ModePerm
	noPermPart := perm &^ os.ModePerm
	// Add the x bit: make everything +x from windows
	permPart |= 0111
	permPart &= 0755

	return noPermPart | permPart
}
