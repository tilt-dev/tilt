package xdg

// XDG Base Directory environment variables.
const (
	envDataHome   = "XDG_DATA_HOME"
	envDataDirs   = "XDG_DATA_DIRS"
	envConfigHome = "XDG_CONFIG_HOME"
	envConfigDirs = "XDG_CONFIG_DIRS"
	envCacheHome  = "XDG_CACHE_HOME"
	envRuntimeDir = "XDG_RUNTIME_DIR"
	envStateHome  = "XDG_STATE_HOME"
)

type baseDirectories struct {
	dataHome   string
	data       []string
	configHome string
	config     []string
	cacheHome  string
	runtime    string

	// Non-standard directories.
	stateHome    string
	fonts        []string
	applications []string
}

func (bd baseDirectories) dataFile(relPath string) (string, error) {
	return createPath(relPath, append([]string{bd.dataHome}, bd.data...))
}

func (bd baseDirectories) configFile(relPath string) (string, error) {
	return createPath(relPath, append([]string{bd.configHome}, bd.config...))
}

func (bd baseDirectories) cacheFile(relPath string) (string, error) {
	return createPath(relPath, []string{bd.cacheHome})
}

func (bd baseDirectories) runtimeFile(relPath string) (string, error) {
	return createPath(relPath, []string{bd.runtime})
}

func (bd baseDirectories) stateFile(relPath string) (string, error) {
	return createPath(relPath, []string{bd.stateHome})
}

func (bd baseDirectories) searchDataFile(relPath string) (string, error) {
	return searchFile(relPath, append([]string{bd.dataHome}, bd.data...))
}

func (bd baseDirectories) searchConfigFile(relPath string) (string, error) {
	return searchFile(relPath, append([]string{bd.configHome}, bd.config...))
}

func (bd baseDirectories) searchCacheFile(relPath string) (string, error) {
	return searchFile(relPath, []string{bd.cacheHome})
}

func (bd baseDirectories) searchRuntimeFile(relPath string) (string, error) {
	return searchFile(relPath, []string{bd.runtime})
}

func (bd baseDirectories) searchStateFile(relPath string) (string, error) {
	return searchFile(relPath, []string{bd.stateHome})
}
