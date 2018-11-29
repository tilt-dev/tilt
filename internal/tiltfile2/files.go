package tiltfile2

import ()

type gitRepo struct {
	basePath             string
	gitignoreContents    string
	dockerignoreContents string
}

func (t *Tiltfile) newGitRepo(path string) (gitRepo, error) {
	absPath := t.absPath(path)
	_, err := os.Stat(absPath)
	if err != nil {
		return gitRepo{}, fmt.Errorf("Reading path %s: %v", path, err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
		return gitRepo{}, fmt.Errorf("%s isn't a valid git repo: it doesn't have a .git/ directory", absPath)
	}

	gitignoreContents, err := ioutil.ReadFile(filepath.Join(absPath, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		return gitRepo{}, err
	}

	dockerignoreContents, err := ioutil.ReadFile(filepath.Join(absPath, ".dockerignore"))
	if err != nil {
		if !os.IsNotExist(err) {
			return gitRepo{}, err
		}
	}

	return gitRepo{absPath, string(gitignoreContents), string(dockerignoreContents)}, nil
}

var _ skylark.Value = gitRepo{}

func (gr gitRepo) String() string {
	return fmt.Sprintf("[gitRepo] '%v'", gr.basePath)
}

func (gr gitRepo) Type() string {
	return "gitRepo"
}

func (gr gitRepo) Freeze() {}

func (gr gitRepo) Truth() skylark.Bool {
	return gr.basePath != "" || gr.gitignoreContents != "" || gr.dockerignoreContents != ""
}

func (gitRepo) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: gitRepo")
}

func (gr gitRepo) Attr(name string) (skylark.Value, error) {
	switch name {
	case "path":
		return skylark.NewBuiltin(name, gr.path), nil
	default:
		return nil, nil
	}

}

func (gr gitRepo) AttrNames() []string {
	return []string{"path"}
}

func (gr gitRepo) path(thread *skylark.Thread, fn *skylark.Builtin, args skylark.Tuple, kwargs []skylark.Tuple) (skylark.Value, error) {
	var path string
	err := skylark.UnpackArgs(fn.Name(), args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	return gr.makeLocalPath(path), nil
}

func (gr gitRepo) makeLocalPath(path string) localPath {
	return localPath{filepath.Join(gr.basePath, path), gr}
}

type localPath struct {
	path string
	repo gitRepo
}

func (t *Tiltfile) localPathFromSkylarkValue(v skylark.Value) (localPath, error) {
	switch v := v.(type) {
	case localPath:
		return v, nil
	case gitRepo:
		return v.makeLocalPath("."), nil
	case skylark.String:
		return t.localPathFromString(string(v))
	default:
		return localPath{}, fmt.Errorf(" Expected local path. Actual type: %T", v)
	}
}

func (t *Tiltfile) localPathFromString(path string) (localPath, error) {
	absPath := t.absPath(path)
	_, err := os.Stat(absPath)
	if err != nil {
		return localPath{}, fmt.Errorf("Reading path %s: %v", path, err)
	}

	absDirPath := filepath.Dir(absPath)
	_, err = os.Stat(filepath.Join(absDirPath, ".git"))
	if err != nil && !os.IsNotExist(err) {
		return localPath{}, fmt.Errorf("Reading path %s: %v", path, err)
	}

	hasGitDir := !os.IsNotExist(err)
	repo := gitRepo{}

	if hasGitDir {
		repo, err = t.newGitRepo(absDirPath)
		if err != nil {
			return localPath{}, err
		}
	}

	return localPath{
		path: absPath,
		repo: repo,
	}, nil
}

var _ skylark.Value = localPath{}

func (lp localPath) String() string {
	return lp.path
}

func (localPath) Type() string {
	return "localPath"
}

func (localPath) Freeze() {}

func (localPath) Hash() (uint32, error) {
	return 0, errors.New("unhashable type: localPath")
}

func (lp localPath) Truth() skylark.Bool {
	return lp != localPath{}
}
