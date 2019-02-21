package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/frontend/dockerfile/command"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
)

type AST struct {
	result *parser.Result
}

func ParseAST(df Dockerfile) (AST, error) {
	result, err := parser.Parse(bytes.NewBufferString(string(df)))
	if err != nil {
		return AST{}, errors.Wrap(err, "dockerfile.ParseAST")
	}

	return AST{
		result: result,
	}, nil
}

// Find all images referenced in this dockerfile and call the visitor function.
// If the visitor function returns a new image, subsitute that image into the dockerfile.
func (a AST) traverseImageRefs(visitor func(node *parser.Node, ref reference.Named) reference.Named) error {
	return a.Traverse(func(node *parser.Node) error {
		switch node.Value {
		case command.From:
			if node.Next == nil {
				return nil
			}
			ref, err := reference.ParseNormalizedNamed(node.Next.Value)
			if err != nil {
				return nil // drop the error, we don't care about malformed images
			}
			newRef := visitor(node, ref)
			if newRef != nil {
				node.Next.Value = newRef.String()
			}

		case command.Copy:
			if len(node.Flags) == 0 {
				return nil
			}

			inst, err := instructions.ParseInstruction(node)
			if err != nil {
				return nil // ignore parsing error
			}

			copyCmd, ok := inst.(*instructions.CopyCommand)
			if !ok {
				return nil
			}

			ref, err := reference.ParseNormalizedNamed(copyCmd.From)
			if err != nil {
				return nil // drop the error, we don't care about malformed images
			}

			newRef := visitor(node, ref)
			if newRef != nil {
				for i, flag := range node.Flags {
					if strings.HasPrefix(flag, "--from=") {
						node.Flags[i] = fmt.Sprintf("--from=%s", newRef.String())
					}
				}
			}
		}

		return nil
	})
}

func (a AST) InjectImageDigest(ref reference.NamedTagged) (bool, error) {
	modified := false
	err := a.traverseImageRefs(func(node *parser.Node, toReplace reference.Named) reference.Named {
		if toReplace.Name() == ref.Name() {
			modified = true
			return ref
		}
		return nil
	})
	return modified, err
}

// Post-order traversal of the Dockerfile AST.
// Halts immediately on error.
func (a AST) Traverse(visit func(*parser.Node) error) error {
	return a.traverseNode(a.result.AST, visit)
}

func (a AST) traverseNode(node *parser.Node, visit func(*parser.Node) error) error {
	for _, c := range node.Children {
		err := a.traverseNode(c, visit)
		if err != nil {
			return err
		}
	}
	return visit(node)
}

func (a AST) Print() (Dockerfile, error) {
	buf := bytes.NewBuffer(nil)
	currentLine := 1
	for _, node := range a.result.AST.Children {
		for currentLine < node.StartLine {
			_, err := buf.Write([]byte("\n"))
			if err != nil {
				return "", err
			}
			currentLine++
		}

		err := a.printNode(node, buf)
		if err != nil {
			return "", err
		}

		currentLine = node.StartLine + 1
		if node.Next != nil && node.Next.StartLine != 0 {
			currentLine = node.Next.StartLine + 1
		}
	}
	return Dockerfile(buf.String()), nil
}

// Loosely adapted from
// https://github.com/jessfraz/dockfmt/blob/master/format.go
func (a AST) printNode(node *parser.Node, writer io.Writer) error {
	var v string

	// format per directive
	switch node.Value {
	// all the commands that use parseMaybeJSON
	// https://github.com/moby/buildkit/blob/2ec7d53b00f24624cda0adfbdceed982623a93b3/frontend/dockerfile/parser/parser.go#L152
	case command.Cmd, command.Entrypoint, command.Run, command.Shell:
		v = fmtCmd(node)
	case command.Label:
		v = fmtLabel(node)
	default:
		v = fmtDefault(node)
	}

	_, err := fmt.Fprintln(writer, v)
	if err != nil {
		return err
	}
	return nil
}

func getCmd(n *parser.Node) []string {
	if n == nil {
		return nil
	}

	cmd := []string{strings.ToUpper(n.Value)}
	if len(n.Flags) > 0 {
		cmd = append(cmd, n.Flags...)
	}

	return append(cmd, getCmdArgs(n)...)
}

func getCmdArgs(n *parser.Node) []string {
	if n == nil {
		return nil
	}

	cmd := []string{}
	for node := n.Next; node != nil; node = node.Next {
		cmd = append(cmd, node.Value)
		if len(node.Flags) > 0 {
			cmd = append(cmd, node.Flags...)
		}
	}

	return cmd
}

func fmtCmd(node *parser.Node) string {
	if node.Attributes["json"] {
		cmd := []string{strings.ToUpper(node.Value)}
		if len(node.Flags) > 0 {
			cmd = append(cmd, node.Flags...)
		}

		encoded := []string{}
		for _, c := range getCmdArgs(node) {
			encoded = append(encoded, fmt.Sprintf("%q", c))
		}
		return fmt.Sprintf("%s [%s]", strings.Join(cmd, " "), strings.Join(encoded, ", "))
	}

	cmd := getCmd(node)
	return strings.Join(cmd, " ")
}

func fmtDefault(node *parser.Node) string {
	cmd := getCmd(node)
	return strings.Join(cmd, " ")
}

func fmtLabel(node *parser.Node) string {
	cmd := getCmd(node)
	assignments := []string{cmd[0]}
	for i := 1; i < len(cmd); i += 2 {
		if i+1 < len(cmd) {
			assignments = append(assignments, fmt.Sprintf("%s=%s", cmd[i], cmd[i+1]))
		} else {
			assignments = append(assignments, fmt.Sprintf("%s", cmd[i]))
		}
	}
	return strings.Join(assignments, " ")
}
