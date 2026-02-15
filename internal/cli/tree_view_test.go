package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestTreeView_NoResources(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	uirs := &v1alpha1.UIResourceList{}
	deps := resourceDependencies{}
	testPrintFullTree(cmd, uirs, deps, "")

	// With empty resources, we still show Tiltfile as root (with 0 total resources in stats)
	output := out.String()
	assert.Contains(t, output, "Tiltfile")
	assert.Contains(t, output, "0 total")
}

func TestTreeView_SingleResource(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("api", "ok", "ok"),
		},
	}
	deps := resourceDependencies{}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	assert.Contains(t, output, "Tiltfile")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "ok")
}

func TestTreeView_SimpleChain(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	// database -> api -> frontend (api depends on database, frontend depends on api)
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("frontend", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"frontend": {"api"},
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	// Check tree structure - (Tiltfile) is root, database is child, then api, then frontend
	assert.Contains(t, output, "Tiltfile")
	assert.Contains(t, output, "database")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "frontend")
	// Check that tree characters are present
	assert.True(t, strings.Contains(output, "└──") || strings.Contains(output, "├──"),
		"Expected tree characters in output: %s", output)
}

func TestTreeView_MultipleRoots(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	// Two independent trees:
	// database -> api
	// redis -> cache
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("redis", "ok", "ok"),
			makeTreeViewResource("cache", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api":   {"database"},
		"cache": {"redis"},
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	assert.Contains(t, output, "database")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "redis")
	assert.Contains(t, output, "cache")
}

func TestTreeView_DiamondDependency(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	// Diamond: database -> api, database -> worker, frontend depends on both api and worker
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("worker", "ok", "ok"),
			makeTreeViewResource("frontend", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"worker":   {"database"},
		"frontend": {"api", "worker"},
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	assert.Contains(t, output, "database")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "worker")
	// frontend should appear under both api and worker
	assert.Equal(t, 2, strings.Count(output, "frontend"),
		"frontend should appear twice in diamond dependency")
}

func TestTreeView_BlockersOnly_NoBlockers(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, blockersOnly: true, noColor: true}

	// All resources are OK, nothing blocked
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("database", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api": {"database"},
	}
	testPrintBlockersTree(cmd, uirs, deps, "")

	assert.Contains(t, out.String(), "No blocked resources found")
}

func TestTreeView_BlockersOnly_SimpleChain(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, blockersOnly: true, noColor: true}

	// database (error) -> api (pending) -> frontend (pending)
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("database", "error", "error"),
			makeTreeViewResource("api", "pending", "pending"),
			makeTreeViewResource("frontend", "pending", "pending"),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"frontend": {"api"},
	}
	testPrintBlockersTree(cmd, uirs, deps, "")

	output := out.String()
	assert.Contains(t, output, "Blocked resources")
	assert.Contains(t, output, "database")
	assert.Contains(t, output, "api")
	assert.Contains(t, output, "frontend")
}

func TestTreeView_BlockersOnly_MultipleBlockers(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, blockersOnly: true, noColor: true}

	// Two root blockers: database (blocks api, worker, frontend), redis (blocks cache)
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("database", "error", ""),
			makeTreeViewResource("redis", "error", "error"),
			makeTreeViewResource("api", "pending", ""),
			makeTreeViewResource("worker", "pending", ""),
			makeTreeViewResource("frontend", "pending", ""),
			makeTreeViewResource("cache", "pending", ""),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"worker":   {"database"},
		"frontend": {"api"},
		"cache":    {"redis"},
	}
	testPrintBlockersTree(cmd, uirs, deps, "")

	output := out.String()
	// database should appear first (blocks more resources: api, worker, frontend = 3)
	// redis blocks only cache = 1
	dbIndex := strings.Index(output, "database")
	redisIndex := strings.Index(output, "redis")
	assert.True(t, dbIndex < redisIndex,
		"database should appear before redis (blocks more resources)")
}

func TestTreeView_StatusFormatting(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true}

	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("api", "pending", "pending"),
		},
	}
	deps := resourceDependencies{}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	// With new status formatting, "pending"+"pending" becomes "(pending)"
	assert.Contains(t, output, "(pending)")
}

func TestBuildTreeNode_Basic(t *testing.T) {
	cmd := &treeViewCmd{noColor: true}
	resourceByName := map[string]*v1alpha1.UIResource{
		"root": {
			ObjectMeta: metav1.ObjectMeta{Name: "root"},
			Status: v1alpha1.UIResourceStatus{
				UpdateStatus:  "ok",
				RuntimeStatus: "ok",
			},
		},
	}
	childrenOf := map[string][]string{}

	node := cmd.buildTreeNode("root", resourceByName, childrenOf)

	assert.Equal(t, "root", node.name)
	assert.Equal(t, "ok", node.updateStatus)
	assert.Empty(t, node.children)
}

func TestBuildTreeNode_WithChildren(t *testing.T) {
	cmd := &treeViewCmd{noColor: true}
	resourceByName := map[string]*v1alpha1.UIResource{
		"parent": {
			ObjectMeta: metav1.ObjectMeta{Name: "parent"},
			Status:     v1alpha1.UIResourceStatus{UpdateStatus: "ok"},
		},
		"child1": {
			ObjectMeta: metav1.ObjectMeta{Name: "child1"},
			Status:     v1alpha1.UIResourceStatus{UpdateStatus: "pending"},
		},
		"child2": {
			ObjectMeta: metav1.ObjectMeta{Name: "child2"},
			Status:     v1alpha1.UIResourceStatus{UpdateStatus: "pending"},
		},
	}
	childrenOf := map[string][]string{
		"parent": {"child1", "child2"},
	}

	node := cmd.buildTreeNode("parent", resourceByName, childrenOf)

	assert.Equal(t, "parent", node.name)
	require.Len(t, node.children, 2)
	assert.Equal(t, "child1", node.children[0].name)
	assert.Equal(t, "child2", node.children[1].name)
}

func TestGetStatusText(t *testing.T) {
	cmd := &treeViewCmd{noColor: true}

	tests := []struct {
		name     string
		node     *treeNode
		expected string
	}{
		{
			name:     "empty status",
			node:     &treeNode{},
			expected: "(ok)",
		},
		{
			name: "both ok",
			node: &treeNode{
				updateStatus:  "ok",
				runtimeStatus: "ok",
			},
			expected: "(ok)",
		},
		{
			name: "update ok runtime not_applicable",
			node: &treeNode{
				updateStatus:  "ok",
				runtimeStatus: "not_applicable",
			},
			expected: "(ok)",
		},
		{
			name: "build error",
			node: &treeNode{
				updateStatus:  "error",
				runtimeStatus: "ok",
			},
			expected: "(build error)",
		},
		{
			name: "runtime error",
			node: &treeNode{
				updateStatus:  "ok",
				runtimeStatus: "error",
			},
			expected: "(runtime error)",
		},
		{
			name: "both error",
			node: &treeNode{
				updateStatus:  "error",
				runtimeStatus: "error",
			},
			expected: "(error)",
		},
		{
			name: "building",
			node: &treeNode{
				updateStatus:  "in_progress",
				runtimeStatus: "pending",
			},
			expected: "(building)",
		},
		{
			name: "build pending",
			node: &treeNode{
				updateStatus:  "pending",
				runtimeStatus: "ok",
			},
			expected: "(build pending)",
		},
		{
			name: "runtime starting",
			node: &treeNode{
				updateStatus:  "ok",
				runtimeStatus: "pending",
			},
			expected: "(starting)",
		},
		{
			name: "both pending",
			node: &treeNode{
				updateStatus:  "pending",
				runtimeStatus: "pending",
			},
			expected: "(pending)",
		},
		{
			name: "not started",
			node: &treeNode{
				updateStatus:  "none",
				runtimeStatus: "none",
			},
			expected: "(not started)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := cmd.getStatusText(tc.node)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTreeView_ColorOutput(t *testing.T) {
	// Temporarily force colors on (the library disables them when not writing to a terminal)
	originalNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = originalNoColor }()

	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: false} // Colors enabled

	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("healthy", "ok", "ok"),
			makeTreeViewResource("broken", "error", "error"),
			makeTreeViewResource("starting", "ok", "pending"),
		},
	}
	deps := resourceDependencies{}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()

	// ANSI color codes
	green := "\x1b[32m"
	red := "\x1b[31m"
	yellow := "\x1b[33m"
	reset := "\x1b[0m"

	// Check that green is used for ok resources
	assert.Contains(t, output, green+"healthy (ok)"+reset,
		"healthy resource should be green")

	// Check that red is used for error resources
	assert.Contains(t, output, red+"broken (error)"+reset,
		"broken resource should be red")

	// Check that yellow is used for pending/starting resources
	assert.Contains(t, output, yellow+"starting (starting)"+reset,
		"starting resource should be yellow")
}

func TestTreeView_Default(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true, dedupe: false}

	// Complex dependency: database -> api -> frontend, database -> worker -> frontend
	// frontend has children: mobile
	// By default (no dedupe), mobile should appear twice (under both frontends)
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("worker", "ok", "ok"),
			makeTreeViewResource("frontend", "ok", "ok"),
			makeTreeViewResource("mobile", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"worker":   {"database"},
		"frontend": {"api", "worker"},
		"mobile":   {"frontend"},
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	t.Logf("Default output:\n%s", output)

	// By default, mobile should appear twice (under both frontends)
	assert.Equal(t, 2, strings.Count(output, "mobile"),
		"mobile should appear twice by default")

	// By default, "also depends on" should NOT be shown
	assert.NotContains(t, output, "[also depends on:",
		"also depends on should not appear by default")
}

func TestTreeView_Dedupe(t *testing.T) {
	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: true, dedupe: true}

	// Same setup as default test
	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("api", "ok", "ok"),
			makeTreeViewResource("worker", "ok", "ok"),
			makeTreeViewResource("frontend", "ok", "ok"),
			makeTreeViewResource("mobile", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api":      {"database"},
		"worker":   {"database"},
		"frontend": {"api", "worker"},
		"mobile":   {"frontend"},
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()
	t.Logf("Dedupe output:\n%s", output)

	// frontend should appear twice (under api and worker)
	assert.Equal(t, 2, strings.Count(output, "frontend"),
		"frontend should appear twice")

	// With --dedupe, mobile should appear only once (deduplication)
	assert.Equal(t, 1, strings.Count(output, "mobile"),
		"mobile should appear only once with --dedupe")

	// frontend should have "also depends on" since it has multiple dependencies
	assert.Contains(t, output, "[also depends on:",
		"frontend should show also depends on with --dedupe")
}

func TestTreeView_AlsoDependsOnColored(t *testing.T) {
	// Force colors on
	originalNoColor := color.NoColor
	color.NoColor = false
	defer func() { color.NoColor = originalNoColor }()

	var out bytes.Buffer
	streams := genericiooptions.IOStreams{Out: &out}
	cmd := &treeViewCmd{streams: streams, noColor: false, dedupe: true}

	uirs := &v1alpha1.UIResourceList{
		Items: []v1alpha1.UIResource{
			makeTreeViewResource("(Tiltfile)", "ok", "not_applicable"),
			makeTreeViewResource("database", "ok", "ok"),
			makeTreeViewResource("cache", "error", "error"),
			makeTreeViewResource("api", "ok", "ok"),
		},
	}
	deps := resourceDependencies{
		"api": {"database", "cache"}, // api depends on both database (ok) and cache (error)
	}
	testPrintFullTree(cmd, uirs, deps, "")

	output := out.String()

	// Check that cyan is used for brackets
	cyan := "\x1b[36m"
	assert.Contains(t, output, cyan+"[also depends on: ",
		"brackets should be cyan")

	// Check that dependency names are colored by status
	green := "\x1b[32m"
	red := "\x1b[31m"
	assert.Contains(t, output, green+"database",
		"database should be green in also depends on")
	assert.Contains(t, output, red+"cache",
		"cache should be red in also depends on")
}

// Helper function to create test UIResources
func makeTreeViewResource(name, updateStatus, runtimeStatus string) v1alpha1.UIResource {
	return v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.UIResourceStatus{
			UpdateStatus:  v1alpha1.UpdateStatus(updateStatus),
			RuntimeStatus: v1alpha1.RuntimeStatus(runtimeStatus),
		},
	}
}

func testPrintFullTree(cmd *treeViewCmd, uirs *v1alpha1.UIResourceList, deps resourceDependencies, tiltfilePath string) {
	graph := buildDependencyGraph(uirs, deps)
	config := cmd.prepareFullTree(graph, tiltfilePath)

	if len(config.roots) == 0 {
		cmd.outputText("%s\n", config.emptyMessage)
		return
	}

	cmd.printTree(config)
}

func testPrintBlockersTree(cmd *treeViewCmd, uirs *v1alpha1.UIResourceList, deps resourceDependencies, tiltfilePath string) {
	graph := buildDependencyGraph(uirs, deps)
	config := cmd.prepareBlockersTree(graph, tiltfilePath)

	if config.header != "" {
		cmd.outputText("%s\n\n", config.header)
	}

	if len(config.roots) == 0 {
		cmd.outputText("%s\n", config.emptyMessage)
		return
	}

	cmd.printTree(config)
}
