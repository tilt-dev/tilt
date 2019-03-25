package ospath

// Calculate a display name for a file by figuring it out what basedir it's relative
// to and trimming the basedir prefix off the front
func FileDisplayName(baseDirs []string, f string) string {
	ret := f
	for _, baseDir := range baseDirs {
		short, isChild := Child(baseDir, f)
		if isChild && len(short) < len(ret) {
			ret = short
		}
	}
	return ret
}

// Calculate display name for list of files.
func FileListDisplayNames(baseDirs []string, files []string) []string {
	var ret []string
	for _, f := range files {
		ret = append(ret, FileDisplayName(baseDirs, f))
	}
	return ret
}
