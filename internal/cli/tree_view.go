package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Tree drawing characters
const (
	treeBranch = "├──"
	treeCorner = "└──"
	treeSpace  = "   "
	treeIndent = "│  "
)

const tiltfileResource = "(Tiltfile)"

// treeViewCmd displays resources in a tree structure based on their dependencies.
type treeViewCmd struct {
	streams      genericiooptions.IOStreams
	blockersOnly bool
	noColor      bool
	dedupe       bool
}

var _ tiltCmd = &treeViewCmd{}

func newTreeViewCmd(streams genericiooptions.IOStreams) *treeViewCmd {
	return &treeViewCmd{
		streams: streams,
	}
}

func (c *treeViewCmd) name() model.TiltSubcommand { return "tree-view" }

func (c *treeViewCmd) outputText(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(c.streams.Out, format, a...)
}

func (c *treeViewCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree-view",
		Short: "Display resources in a tree structure based on dependencies",
		Long: `Display resources in a tree structure showing their dependency relationships.

By default, shows all resources organized by their resource_deps relationships
as defined in the Tiltfile. Resources with no dependencies appear as root nodes,
with resources that depend on them shown as children.

Use --blockers to show only resources that are currently pending/blocked,
filtered to show the root blockers and their dependents.`,
		Example: `  # Show full resource dependency tree
  tilt alpha tree-view

  # Show only blocked resources and their blockers
  tilt alpha tree-view --blockers`,
	}

	addConnectServerFlags(cmd)
	cmd.Flags().BoolVar(&c.blockersOnly, "blockers", false, "Show only blocked resources and their root blockers")
	cmd.Flags().BoolVar(&c.noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&c.dedupe, "dedupe", false, "Deduplicate resources with multiple parents (show subtrees only once, with [also depends on: ...] annotations)")

	return cmd
}

func (c *treeViewCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)
	cmdTags := engineanalytics.CmdTags(map[string]string{
		"blockers": fmt.Sprintf("%t", c.blockersOnly),
	})
	a.Incr("cmd.tree-view", cmdTags.AsMap())
	defer a.Flush(time.Second)

	// Fetch UIResources for status information
	ctrlclient, err := newClient(ctx)
	if err != nil {
		return err
	}

	var uirs v1alpha1.UIResourceList
	err = ctrlclient.List(ctx, &uirs)
	if err != nil {
		return err
	}

	// Fetch Session to get Tiltfile path
	var sessions v1alpha1.SessionList
	if err := ctrlclient.List(ctx, &sessions); err != nil {
		return err
	}

	tiltfilePath := ""
	if len(sessions.Items) > 0 {
		tiltfilePath = sessions.Items[0].Spec.TiltfilePath
	}

	// Fetch engine state for static resource dependencies
	deps := c.fetchResourceDependencies()

	if len(uirs.Items) == 0 {
		c.outputText("No resources found.\n")
		return nil
	}

	// Build dependency graph
	graph := buildDependencyGraph(&uirs, deps)

	// Prepare tree configuration based on mode
	var treeConfig treeConfig
	if c.blockersOnly {
		treeConfig = c.prepareBlockersTree(graph, tiltfilePath)
	} else {
		treeConfig = c.prepareFullTree(graph, tiltfilePath)
	}

	// Print header if any
	if treeConfig.header != "" {
		c.outputText("%s\n\n", treeConfig.header)
	}

	// Handle empty tree
	if len(treeConfig.roots) == 0 {
		c.outputText("%s\n", treeConfig.emptyMessage)
		return nil
	}

	c.printTree(treeConfig)

	return nil
}

// dependencyGraph holds the processed dependency relationships
type dependencyGraph struct {
	resourceByName map[string]*v1alpha1.UIResource // resource name -> resource
	deps           resourceDependencies            // child -> parents mapping
	childrenOf     map[string][]string             // parent -> children mapping
	hasParent      map[string]bool                 // resources that have at least one parent
}

// buildDependencyGraph creates a dependency graph from resources and their dependencies
func buildDependencyGraph(uirs *v1alpha1.UIResourceList, deps resourceDependencies) dependencyGraph {
	resourceByName := make(map[string]*v1alpha1.UIResource)
	for i := range uirs.Items {
		res := &uirs.Items[i]
		resourceByName[res.Name] = res
	}

	childrenOf := make(map[string][]string)
	hasParent := make(map[string]bool)
	for childName, parents := range deps {
		for _, parentName := range parents {
			if _, exists := resourceByName[parentName]; exists {
				childrenOf[parentName] = append(childrenOf[parentName], childName)
				hasParent[childName] = true
			}
		}
	}

	return dependencyGraph{
		resourceByName: resourceByName,
		deps:           deps,
		childrenOf:     childrenOf,
		hasParent:      hasParent,
	}
}

// treeConfig holds configuration for tree printing
type treeConfig struct {
	roots             []*treeNode                     // root nodes to print
	resourceByName    map[string]*v1alpha1.UIResource // all resources
	deps              resourceDependencies            // child -> parents mapping
	filterDeps        map[string]bool                 // if set, filter "also depends on" to these
	header            string                          // header to print before tree
	emptyMessage      string                          // message if no roots
	circularResources []string                        // resources in circular dependencies (not reachable from roots)
}

// treeNode represents a node in the tree
type treeNode struct {
	name          string
	displayName   string // can differ from name (e.g., Tiltfile path)
	updateStatus  string
	runtimeStatus string
	children      []*treeNode
}

// resourceDependencies maps resource name -> list of dependencies (resources it depends on)
type resourceDependencies map[string][]string

// fetchResourceDependencies fetches the static resource dependencies from the engine dump.
func (c *treeViewCmd) fetchResourceDependencies() resourceDependencies {
	body := apiGet("dump/engine")
	defer func() {
		_ = body.Close()
	}()

	var engineState struct {
		ManifestTargets map[string]struct {
			Manifest struct {
				ResourceDependencies []string `json:"ResourceDependencies"`
			} `json:"Manifest"`
		} `json:"ManifestTargets"`
	}

	if err := json.NewDecoder(body).Decode(&engineState); err != nil {
		cmdFail(fmt.Errorf("failed to decode engine state: %v", err))
	}

	deps := make(resourceDependencies)
	for name, target := range engineState.ManifestTargets {
		if len(target.Manifest.ResourceDependencies) > 0 {
			deps[name] = target.Manifest.ResourceDependencies
		}
	}

	return deps
}

func (c *treeViewCmd) prepareFullTree(graph dependencyGraph, tiltfilePath string) treeConfig {
	// Find root resources (resources with no dependencies, excluding Tiltfile)
	var roots []string
	for name := range graph.resourceByName {
		if !graph.hasParent[name] && name != tiltfileResource {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)

	// Detect circular dependencies: resources that have parents but are not reachable from roots
	reachable := make(map[string]bool)
	for _, rootName := range roots {
		c.collectDescendants(rootName, graph.childrenOf, reachable)
	}
	var circularResources []string
	for name := range graph.resourceByName {
		if name != tiltfileResource && graph.hasParent[name] && !reachable[name] {
			circularResources = append(circularResources, name)
		}
	}
	sort.Strings(circularResources)

	graph.childrenOf[tiltfileResource] = roots

	tiltfileDisplayName := "Tiltfile"
	if tiltfilePath != "" {
		tiltfileDisplayName = "Tiltfile: " + tiltfilePath
	}

	// Build tree starting from Tiltfile
	rootNode := c.buildTreeNode(tiltfileResource, graph.resourceByName, graph.childrenOf, make(map[string]bool))
	rootNode.displayName = tiltfileDisplayName

	return treeConfig{
		roots:             []*treeNode{rootNode},
		resourceByName:    graph.resourceByName,
		deps:              graph.deps,
		filterDeps:        nil, // show all deps in "also depends on"
		emptyMessage:      "No resources found.",
		circularResources: circularResources,
	}
}

func (c *treeViewCmd) prepareBlockersTree(graph dependencyGraph, tiltfilePath string) treeConfig {
	notOK := make(map[string]bool)
	for name, res := range graph.resourceByName {
		if c.isResourceNotOK(res) {
			notOK[name] = true
		}
	}

	if len(notOK) == 0 {
		return treeConfig{
			roots:        nil,
			emptyMessage: "No blocked resources found. All resources are OK.",
		}
	}

	// Find root blockers: not-OK resources whose parents are all OK
	var rootBlockers []string
	for name := range notOK {
		hasNotOKParent := false
		for _, parentName := range graph.deps[name] {
			if notOK[parentName] {
				hasNotOKParent = true
				break
			}
		}
		if !hasNotOKParent {
			rootBlockers = append(rootBlockers, name)
		}
	}

	if len(rootBlockers) == 0 {
		return treeConfig{
			roots:        nil,
			emptyMessage: "No root blockers found.",
		}
	}

	// Collect all displayed resources for filtering "also depends on"
	displayedResources := make(map[string]bool)
	for _, rootName := range rootBlockers {
		c.collectDescendants(rootName, graph.childrenOf, displayedResources)
	}

	// Build root nodes
	var roots []*treeNode
	for _, blockerName := range rootBlockers {
		root := c.buildTreeNode(blockerName, graph.resourceByName, graph.childrenOf, make(map[string]bool))
		roots = append(roots, root)
	}

	// Build header
	header := "Blocked resources (root blockers and their dependents):"
	if tiltfilePath != "" {
		header = fmt.Sprintf("Blocked resources for Tiltfile: %s", tiltfilePath)
	}

	return treeConfig{
		roots:          roots,
		resourceByName: graph.resourceByName,
		deps:           graph.deps,
		filterDeps:     displayedResources,
		header:         header,
		emptyMessage:   "No root blockers found.",
	}
}

// isResourceNotOK checks if a resource has a problematic status
func (c *treeViewCmd) isResourceNotOK(res *v1alpha1.UIResource) bool {
	updateStatus := strings.ToLower(string(res.Status.UpdateStatus))
	runtimeStatus := strings.ToLower(string(res.Status.RuntimeStatus))

	updateBad := updateStatus != "ok" && updateStatus != "not_applicable" && updateStatus != "none" && updateStatus != ""
	runtimeBad := runtimeStatus != "ok" && runtimeStatus != "not_applicable" && runtimeStatus != "none" && runtimeStatus != ""

	return updateBad || runtimeBad
}

// buildTreeNode recursively builds a tree node
func (c *treeViewCmd) buildTreeNode(
	name string,
	resourceByName map[string]*v1alpha1.UIResource,
	childrenOf map[string][]string,
	visited map[string]bool,
) *treeNode {
	node := &treeNode{name: name, displayName: name}

	if res := resourceByName[name]; res != nil {
		node.updateStatus = string(res.Status.UpdateStatus)
		node.runtimeStatus = string(res.Status.RuntimeStatus)
	}

	// Check for circular dependency
	if visited[name] {
		node.displayName = name + " (circular)"
		return node
	}
	visited[name] = true

	// Add children
	children := childrenOf[name]
	sort.Strings(children)
	for _, childName := range children {
		childVisited := make(map[string]bool)
		for k, v := range visited {
			childVisited[k] = v
		}
		child := c.buildTreeNode(childName, resourceByName, childrenOf, childVisited)
		node.children = append(node.children, child)
	}

	return node
}

type treeStats struct {
	total    int
	ok       int
	pending  int
	building int
	errors   int
}

type treePrintState struct {
	expanded  map[string]bool // tracks which nodes have been expanded (for dedupe)
	stats     treeStats       // stats collected during printing
	config    treeConfig      // tree configuration
	seenNodes map[string]bool // tracks unique nodes for stats (not affected by dedupe)
}

func (c *treeViewCmd) printTree(config treeConfig) {
	state := &treePrintState{
		expanded:  make(map[string]bool),
		seenNodes: make(map[string]bool),
		config:    config,
	}

	for i, root := range config.roots {
		if i > 0 {
			c.outputText("\n")
		}
		c.printNode(root, "", true, true, "", state)
	}

	// Print circular dependencies section if any
	if len(config.circularResources) > 0 {
		c.outputText("\nCircular dependencies:\n")
		for _, name := range config.circularResources {
			// Count these in stats
			state.stats.total++
			if res, ok := config.resourceByName[name]; ok {
				node := &treeNode{
					name:          name,
					displayName:   name,
					updateStatus:  string(res.Status.UpdateStatus),
					runtimeStatus: string(res.Status.RuntimeStatus),
				}
				statusText, colorKind := c.getStatusText(node)
				switch colorKind {
				case colorOK:
					state.stats.ok++
				case colorWarning:
					if strings.ToLower(node.updateStatus) == "in_progress" {
						state.stats.building++
					} else {
						state.stats.pending++
					}
				case colorError:
					state.stats.errors++
				}
				nodeText := name
				if statusText != "" {
					nodeText = name + " " + statusText
				}
				c.outputText("  %s\n", c.colorize(nodeText, colorKind))
			} else {
				c.outputText("  %s\n", name)
			}
		}
	}

	c.printStats(state.stats)
}

// printNode recursively prints a tree node
func (c *treeViewCmd) printNode(node *treeNode, prefix string, isLast bool, isRoot bool, parentName string, state *treePrintState) {
	// Build the line prefix
	var linePrefix string
	if isRoot {
		linePrefix = ""
	} else if isLast {
		linePrefix = prefix + treeCorner + " "
	} else {
		linePrefix = prefix + treeBranch + " "
	}

	statusText, colorKind := c.getStatusText(node)

	// Collect stats (only count each unique node once, skip Tiltfile)
	if !state.seenNodes[node.name] && node.name != tiltfileResource {
		state.seenNodes[node.name] = true
		state.stats.total++
		switch colorKind {
		case colorOK:
			state.stats.ok++
		case colorWarning:
			if strings.ToLower(node.updateStatus) == "in_progress" {
				state.stats.building++
			} else {
				state.stats.pending++
			}
		case colorError:
			state.stats.errors++
		}
	}

	nodeText := node.displayName
	if statusText != "" {
		nodeText = node.displayName + " " + statusText
	}

	alsoText := ""
	if c.dedupe && state.expanded[node.name] {
		alsoText = c.buildAlsoDependsOn(node.name, parentName, state.config)
	}

	c.outputText("%s%s%s\n", linePrefix, c.colorize(nodeText, colorKind), alsoText)

	alreadyExpanded := state.expanded[node.name]
	shouldShowChildren := !c.dedupe || !alreadyExpanded

	state.expanded[node.name] = true

	var childPrefix string
	if isRoot {
		childPrefix = ""
	} else if isLast {
		childPrefix = prefix + treeSpace + " "
	} else {
		childPrefix = prefix + treeIndent + " "
	}

	if shouldShowChildren {
		for i, child := range node.children {
			isLastChild := i == len(node.children)-1
			c.printNode(child, childPrefix, isLastChild, false, node.name, state)
		}
	}
}

// buildAlsoDependsOn builds the "[also depends on: ...]" text
func (c *treeViewCmd) buildAlsoDependsOn(resourceName string, parentName string, config treeConfig) string {
	allDeps := config.deps[resourceName]

	var deps []string
	for _, depName := range allDeps {
		if depName == parentName {
			continue
		}
		if config.filterDeps == nil || config.filterDeps[depName] {
			deps = append(deps, depName)
		}
	}

	if len(deps) == 0 {
		return ""
	}

	var depParts []string
	for _, depName := range deps {
		var depKind colorKind
		if res, ok := config.resourceByName[depName]; ok {
			node := &treeNode{
				updateStatus:  string(res.Status.UpdateStatus),
				runtimeStatus: string(res.Status.RuntimeStatus),
			}
			_, depKind = c.getStatusText(node)
		}
		depParts = append(depParts, c.colorize(depName, depKind))
	}

	bracket := c.colorize("[also depends on: ", colorDependsOn)
	closeBracket := c.colorize("]", colorDependsOn)

	return " " + bracket + strings.Join(depParts, ", ") + closeBracket
}

func (c *treeViewCmd) printStats(stats treeStats) {
	c.outputText("\n---\n")

	var parts []string
	parts = append(parts, fmt.Sprintf("%d total", stats.total))

	if stats.ok > 0 {
		parts = append(parts, c.colorize(fmt.Sprintf("%d ok", stats.ok), colorOK))
	}
	if stats.building > 0 {
		parts = append(parts, c.colorize(fmt.Sprintf("%d building", stats.building), colorWarning))
	}
	if stats.pending > 0 {
		parts = append(parts, c.colorize(fmt.Sprintf("%d pending", stats.pending), colorWarning))
	}
	if stats.errors > 0 {
		parts = append(parts, c.colorize(fmt.Sprintf("%d errors", stats.errors), colorError))
	}

	c.outputText("Resources: %s\n", strings.Join(parts, ", "))
}

// collectDescendants adds the resource and all descendants to the set
func (c *treeViewCmd) collectDescendants(name string, childrenOf map[string][]string, set map[string]bool) {
	if set[name] {
		return
	}
	set[name] = true
	for _, child := range childrenOf[name] {
		c.collectDescendants(child, childrenOf, set)
	}
}

func (c *treeViewCmd) getStatusText(node *treeNode) (string, colorKind) {
	update := strings.ToLower(node.updateStatus)
	runtime := strings.ToLower(node.runtimeStatus)

	// Error states take priority
	if update == "error" && runtime == "error" {
		return "(error)", colorError
	}
	if update == "error" {
		return "(build error)", colorError
	}
	if runtime == "error" {
		return "(runtime error)", colorError
	}

	if update == "in_progress" {
		return "(building)", colorWarning
	}

	if update == "pending" && runtime == "pending" {
		return "(pending)", colorWarning
	}
	if update == "pending" {
		return "(build pending)", colorWarning
	}
	if runtime == "pending" {
		return "(starting)", colorWarning
	}

	if runtime == "unknown" {
		return "(unknown)", colorWarning
	}

	// None states (manual trigger, not started)
	if update == "none" || runtime == "none" {
		return "(not started)", colorWarning
	}

	updateOK := update == "ok" || update == "not_applicable" || update == ""
	runtimeOK := runtime == "ok" || runtime == "not_applicable" || runtime == ""

	if updateOK && runtimeOK {
		return "(ok)", colorOK
	}

	return "", colorOK
}

type colorKind int

const (
	colorOK colorKind = iota
	colorWarning
	colorError
	colorDependsOn
)

func (c *treeViewCmd) colorize(text string, kind colorKind) string {
	if c.noColor {
		return text
	}

	switch kind {
	case colorOK:
		return color.GreenString(text)
	case colorWarning:
		return color.YellowString(text)
	case colorError:
		return color.RedString(text)
	case colorDependsOn:
		return color.CyanString(text)
	default:
		return text
	}
}
