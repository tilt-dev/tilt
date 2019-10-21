package dockerfile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/command"
	"github.com/moby/buildkit/frontend/dockerfile/parser"

	"github.com/windmilleng/tilt/pkg/model"
)

var ErrAddInDockerfile = fmt.Errorf("base Dockerfile contains an ADD/COPY, " +
	"which is not currently supported -- move this to an add() call in your Tiltfile")

type Dockerfile string

func (d Dockerfile) Empty() bool {
	return d.String() == ""
}

// DockerfileFromExisting creates a new Dockerfile that uses the supplied image
// as its base image with a FROM statement. This is necessary for iterative
// Docker builds.
func FromExisting(existing reference.NamedTagged) Dockerfile {
	return Dockerfile(fmt.Sprintf("FROM %s", existing.String()))
}

func (d Dockerfile) Append(df Dockerfile) Dockerfile {
	return d.Join(string(df))
}

func (d Dockerfile) Join(s string) Dockerfile {
	return Dockerfile(fmt.Sprintf("%s\n%s", d, s))
}

func (d Dockerfile) WithLabel(label Label, val LabelValue) Dockerfile {
	return d.Join(fmt.Sprintf("LABEL %q=%q", label, val))
}

func (d Dockerfile) AddAll() Dockerfile {
	return d.Join("ADD . /")
}

func (d Dockerfile) Run(cmd model.Cmd) Dockerfile {
	return d.Join(cmd.RunStr())
}

func (d Dockerfile) Entrypoint(cmd model.Cmd) Dockerfile {
	return d.Join(cmd.EntrypointStr())
}

func (d Dockerfile) RmPaths(pathsToRm []string) Dockerfile {
	if len(pathsToRm) == 0 {
		return d
	}

	// Add 'rm' statements; if changed path was deleted locally, remove if from container
	rmCmd := strings.Builder{}
	rmCmd.WriteString("rm -rf")
	for _, p := range pathsToRm {
		rmCmd.WriteString(fmt.Sprintf(" %s", p))
	}
	return d.Join(fmt.Sprintf("RUN %s", rmCmd.String()))
}

func (d Dockerfile) traverse(visit func(node *parser.Node) error) error {
	ast, err := ParseAST(d)
	if err != nil {
		return err
	}

	return ast.Traverse(visit)
}

// If possible, split this dockerfile into two parts:
// a base dockerfile (without any adds/copys) and a "iterative" dockerfile.
// Useful for constructing the directory cache.
// Returns false if we can't split it.
func (d Dockerfile) SplitIntoBaseDockerfile() (Dockerfile, Dockerfile, bool) {
	// TODO(nick): Right now, we just check for the first ADD/COPY
	// and split after than. This is Good Enough (tm) for cache dirs.
	// In the future, we would need to understand multi-stage builds.
	//
	// TODO(nick): This would be easier if we could serialize df parser nodes
	// back into strings, but I haven't found an easy off-the-shelf
	// library for doing that.
	startLine := -1
	err := d.traverse(func(node *parser.Node) error {
		switch node.Value {
		case command.Add, command.Copy:
			if startLine == -1 {
				startLine = node.StartLine
			}
		}
		return nil
	})
	if err != nil {
		return "", "", false
	}

	// If there is no ADD, we're not sure what we're dealing with.
	if startLine == -1 {
		return "", "", false
	}

	lines := strings.Split(string(d), "\n")

	// line numbers in dockerfile nodes are 1-based instead of 0-based
	baseDf := strings.Join(lines[:startLine-1], "\n")
	restDf := strings.Join(lines[startLine-1:], "\n")
	return Dockerfile(baseDf), Dockerfile(restDf), true
}

func (d Dockerfile) ValidateBaseDockerfile() error {
	return d.traverse(func(node *parser.Node) error {
		switch node.Value {
		case command.Add:
			return ErrAddInDockerfile
		case command.Copy:
			for _, flag := range node.Flags {
				if strings.HasPrefix(flag, "--from=") {
					return nil
				}
			}
			return ErrAddInDockerfile
		}
		return nil
	})
}

// Find all images referenced in this dockerfile.
func (d Dockerfile) FindImages() ([]reference.Named, error) {
	result := []reference.Named{}
	ast, err := ParseAST(d)
	if err != nil {
		return nil, err
	}

	err = ast.traverseImageRefs(func(node *parser.Node, ref reference.Named) reference.Named {
		result = append(result, ref)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DeriveSyncs finds ADD statements in a Dockerfile and turns them into Tilt model.Sync's.
// Relative paths in an ADD statement are relative to the build context (passed as an arg)
// and will appear in the Sync as an abs path.
//
// TODO(nick): This code has several bugs and needs to be revisited.
// In particular, it doesn't properly handle
// 1) multi-stage builds
// 2) ADD/COPY flags
// 3) workdirs
// 4) relative dirs vs absolute dirs
//
// Should probably be reworked to only return local paths,
// as part of a rethink of slimming down the docker build
// context in a way that's not docker-compose specific.
func (d Dockerfile) BUGGY_DeriveSyncs(context string) ([]model.Sync, error) {
	var nodes []*parser.Node
	err := d.traverse(func(node *parser.Node) error {
		switch node.Value {
		case command.Add, command.Copy:
			nodes = append(nodes, node)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	syncs := make([]model.Sync, len(nodes))
	for i, n := range nodes {
		m, err := nodeToSync(n, context)
		if err != nil {
			return nil, err
		}
		syncs[i] = m
	}
	return syncs, nil
}

func nodeToSync(node *parser.Node, context string) (model.Sync, error) {
	cmd := node.Value
	if !(cmd == command.Add || cmd == command.Copy) {
		return model.Sync{}, fmt.Errorf("nodeToSyncs works on ADD/COPY nodes; got '%s'", cmd)
	}

	srcNode := node.Next
	dstNode := srcNode.Next

	src := srcNode.Value
	if !filepath.IsAbs(src) {
		src = filepath.Join(context, src)
	}

	// TODO(maia): do we support relative ContainerPaths in syncs?
	// If not, need to either a. make absolute or b. error out here.

	return model.Sync{
		LocalPath:     src,
		ContainerPath: dstNode.Value,
	}, nil
}

func (d Dockerfile) String() string {
	return string(d)
}
