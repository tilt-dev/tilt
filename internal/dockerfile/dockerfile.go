package dockerfile

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/model"
)

var ErrAddInDockerfile = fmt.Errorf("base Dockerfile contains an ADD/COPY, " +
	"which is not currently supported -- move this to an add() call in your Tiltfile")

type Dockerfile string

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

// If possible, split this dockerfile into two parts:
// a base dockerfile (without any adds/copys) and a "iterative" dockerfile.
// Useful for constructing the directory cache.
// Returns false if we can't split it.
func (d Dockerfile) SplitIntoBaseDockerfile() (Dockerfile, Dockerfile, bool) {
	asString := string(d)
	result, err := parser.Parse(bytes.NewBufferString(asString))
	if err != nil {
		return "", "", false
	}

	// TODO(nick): Right now, we just check for the first ADD/COPY
	// and split after than. This is Good Enough (tm) for cache dirs.
	// In the future, we would need to understand multi-stage builds.
	//
	// TODO(nick): This would be easier if we could serialize df parser nodes
	// back into strings, but I haven't found an easy off-the-shelf
	// library for doing that.
	startLine := -1
	err = traverse(result.AST, func(node *parser.Node) error {
		switch strings.ToUpper(node.Value) {
		case "ADD", "COPY":
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

	lines := strings.Split(asString, "\n")

	// line numbers in dockerfile nodes are 1-based instead of 0-based
	baseDf := strings.Join(lines[:startLine-1], "\n")
	restDf := strings.Join(lines[startLine-1:], "\n")
	return Dockerfile(baseDf), Dockerfile(restDf), true
}

func (d Dockerfile) ValidateBaseDockerfile() error {
	result, err := parser.Parse(bytes.NewBufferString(string(d)))
	if err != nil {
		return errors.Wrap(err, "ValidateBaseDockerfile")
	}

	err = traverse(result.AST, func(node *parser.Node) error {
		switch strings.ToUpper(node.Value) {
		case "ADD", "COPY":
			return ErrAddInDockerfile
		}
		return nil
	})
	return err
}

// DeriveMounts finds ADD statements in a Dockerfile and turns them into Tilt model.Mounts.
// Relative paths in an ADD statement are relative to the build context (passed as an arg)
// and will appear in the Mount as an abs path.
func (d Dockerfile) DeriveMounts(context string) ([]model.Mount, error) {
	result, err := parser.Parse(bytes.NewBufferString(string(d)))
	if err != nil {
		return nil, nil
	}

	var nodes []*parser.Node
	err = traverse(result.AST, func(node *parser.Node) error {
		switch strings.ToUpper(node.Value) {
		case "ADD", "COPY":
			nodes = append(nodes, node)
		}
		return nil
	})

	mounts := make([]model.Mount, len(nodes))
	for i, n := range nodes {
		m, err := nodeToMount(n, context)
		if err != nil {
			return nil, err
		}
		mounts[i] = m
	}
	return mounts, nil
}

func nodeToMount(node *parser.Node, context string) (model.Mount, error) {
	cmd := strings.ToUpper(node.Value)
	if !(cmd == "ADD" || cmd == "COPY") {
		return model.Mount{}, fmt.Errorf("nodeToMounts works on ADD/COPY nodes; got '%s'", cmd)
	}

	srcNode := node.Next
	dstNode := srcNode.Next

	src := srcNode.Value
	if !filepath.IsAbs(src) {
		src = filepath.Join(context, src)
	}

	// TODO(maia): do we support relative ContainerPaths in mounts?
	// If not, need to either a. make absolute or b. error out here.

	return model.Mount{
		LocalPath:     src,
		ContainerPath: dstNode.Value,
	}, nil
}

func (d Dockerfile) String() string {
	return string(d)
}

// Post-order traversal of the Dockerfile AST.
// Halts immediately on error.
func traverse(node *parser.Node, visit func(*parser.Node) error) error {
	for _, c := range node.Children {
		err := traverse(c, visit)
		if err != nil {
			return err
		}
	}
	return visit(node)
}
