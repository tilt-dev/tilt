package k8scontext

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.starlark.net/starlark"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Global isolated kubeconfig management - Tilt never modifies system kubeconfig
var (
	// Mutex to protect access to isolated kubeconfig
	isolatedKubeconfigMutex sync.RWMutex
	// Path to Tilt's isolated kubeconfig copy
	isolatedKubeconfigPath string
	// Original system kubeconfig path for reference
	originalKubeconfigPath string
	// Whether isolation has been initialized
	isolationInitialized bool

	// Thread-local context map for immediate Tiltfile responses
	currentContextMutex sync.RWMutex
	currentContextMap   = make(map[*starlark.Thread]k8s.KubeContext)
)

// Implements functions for dealing with the Kubernetes context.
// Exposes an API for other plugins to get and validate the allowed k8s context.
type Plugin struct {
	context   k8s.KubeContext
	namespace k8s.Namespace
	env       clusterid.Product
}

func NewPlugin(context k8s.KubeContext, namespace k8s.Namespace, env clusterid.Product) Plugin {
	return Plugin{
		context:   context,
		namespace: namespace,
		env:       env,
	}
}

// K8sContextAction is dispatched to notify the system about a context switch
type K8sContextAction struct {
	Context string
}

func (K8sContextAction) Action() {}

func (e Plugin) NewState() interface{} {
	return State{context: e.context, env: e.env}
}

func (e Plugin) OnStart(env *starkit.Environment) error {
	// Initialize kubeconfig isolation on first use
	if err := initializeKubeconfigIsolation(); err != nil {
		return fmt.Errorf("failed to initialize kubeconfig isolation: %v", err)
	}

	err := env.AddBuiltin("allow_k8s_contexts", e.allowK8sContexts)
	if err != nil {
		return err
	}

	err = env.AddBuiltin("k8s_context", e.k8sContext)
	if err != nil {
		return err
	}

	err = env.AddBuiltin("k8s_namespace", e.k8sNamespace)
	if err != nil {
		return err
	}
	return nil
}

func (e Plugin) k8sContext(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contextName starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"context?", &contextName,
	); err != nil {
		return nil, err
	}

	// If no context argument provided, return current context
	if contextName == nil {
		// Check if there's an updated context for this thread
		currentContextMutex.RLock()
		threadContext, exists := currentContextMap[thread]
		currentContextMutex.RUnlock()

		if exists {
			return starlark.String(threadContext), nil
		}

		// Initialize with plugin's context if not set yet
		currentContextMutex.Lock()
		currentContextMap[thread] = e.context
		currentContextMutex.Unlock()

		return starlark.String(e.context), nil
	}

	// Convert context name to string
	contextStr, ok := contextName.(starlark.String)
	if !ok {
		return nil, fmt.Errorf("k8s_context context must be a string; found a %T", contextName)
	}

	newContext := k8s.KubeContext(contextStr)

	// Switch to the new context (isolated - never touches system kubeconfig)
	err := e.switchKubeContextIsolated(thread, string(newContext))
	if err != nil {
		return nil, fmt.Errorf("failed to switch kubernetes context to %s: %v", newContext, err)
	}

	return starlark.String(newContext), nil
}

func (e Plugin) k8sNamespace(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(e.namespace), nil
}

func (e Plugin) allowK8sContexts(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contexts starlark.Value
	if err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
		"contexts", &contexts,
	); err != nil {
		return nil, err
	}

	newContexts := []k8s.KubeContext{}
	for _, c := range value.ValueOrSequenceToSlice(contexts) {
		switch val := c.(type) {
		case starlark.String:
			newContexts = append(newContexts, k8s.KubeContext(val))
		default:
			return nil, fmt.Errorf("allow_k8s_contexts contexts must be a string or a sequence of strings; found a %T", val)

		}
	}

	err := starkit.SetState(thread, func(existing State) State {
		return State{
			context: existing.context,
			env:     existing.env,
			allowed: append(newContexts, existing.allowed...),
		}
	})

	return starlark.None, err
}

// initializeKubeconfigIsolation creates an isolated copy of the system kubeconfig
// that Tilt will use exclusively, never modifying the original
func initializeKubeconfigIsolation() error {
	isolatedKubeconfigMutex.Lock()
	defer isolatedKubeconfigMutex.Unlock()

	if isolationInitialized {
		return nil
	}

	// Load the current system kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("failed to load system kubeconfig: %v", err)
	}

	// Store original path for reference
	originalKubeconfigPath = loadingRules.GetDefaultFilename()

	// Create isolated kubeconfig file
	rand.Seed(time.Now().UnixNano())
	suffix := fmt.Sprintf("%d", rand.Int63())
	isolatedPath := filepath.Join(os.TempDir(), fmt.Sprintf("tilt-isolated-kubeconfig-%s", suffix))

	// Write the copy to isolated file
	if err := clientcmd.WriteToFile(rawConfig, isolatedPath); err != nil {
		return fmt.Errorf("failed to create isolated kubeconfig: %v", err)
	}

	isolatedKubeconfigPath = isolatedPath
	isolationInitialized = true

	// Set KUBECONFIG environment variable to point to our isolated copy
	// This ensures ALL kubernetes client libraries in Tilt use our isolated config
	os.Setenv("KUBECONFIG", isolatedPath)

	return nil
}

// switchKubeContextIsolated changes context only in Tilt's isolated kubeconfig
// The system kubeconfig is never touched
func (e Plugin) switchKubeContextIsolated(thread *starlark.Thread, contextName string) error {
	isolatedKubeconfigMutex.Lock()
	defer isolatedKubeconfigMutex.Unlock()

	if !isolationInitialized {
		return fmt.Errorf("kubeconfig isolation not initialized")
	}

	// Load our isolated kubeconfig
	isolatedConfig, err := clientcmd.LoadFromFile(isolatedKubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load isolated kubeconfig: %v", err)
	}

	// Validate that the target context exists
	if _, exists := isolatedConfig.Contexts[contextName]; !exists {
		return fmt.Errorf("context %q does not exist in kubeconfig", contextName)
	}

	// Validate that we can create a client config for this context
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: isolatedKubeconfigPath}
	contextConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	_, err = contextConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create client config for context %q: %v", contextName, err)
	}

	// If already on desired context, no-op
	if isolatedConfig.CurrentContext == contextName {
		return nil
	}

	// Change the current-context ONLY in our isolated kubeconfig
	isolatedConfig.CurrentContext = contextName
	if err := clientcmd.WriteToFile(*isolatedConfig, isolatedKubeconfigPath); err != nil {
		return fmt.Errorf("failed to update isolated kubeconfig with new context %q: %v", contextName, err)
	}

	// Update internal Tiltfile state for consistency
	err = starkit.SetState(thread, func(state State) State {
		state.context = k8s.KubeContext(contextName)
		return state
	})
	if err != nil {
		return fmt.Errorf("failed to update k8s context state: %v", err)
	}

	// Update thread-local context for immediate access
	currentContextMutex.Lock()
	currentContextMap[thread] = k8s.KubeContext(contextName)
	currentContextMutex.Unlock()

	// Invalidate cached K8s clients to force refresh with new context
	if provider := k8s.GetGlobalConnectionProvider(); provider != nil {
		// Try to cast to the type that has cache invalidation
		if cacheManager, ok := provider.(interface{ Delete(types.NamespacedName) }); ok {
			clusterKey := types.NamespacedName{Name: "default"} // Use default cluster
			cacheManager.Delete(clusterKey)
		}
	}

	return nil
}

var _ starkit.StatefulPlugin = &Plugin{}

type State struct {
	context k8s.KubeContext
	env     clusterid.Product
	allowed []k8s.KubeContext
}

func (s State) KubeContext() k8s.KubeContext {
	return s.context
}

// Returns whether we're allowed to deploy to this kubecontext.
//
// Checks against a manually specified list and a baked-in list
// with known dev cluster names.
//
// Currently, only the tiltfile executor knows about "allowed" kubecontexts.
//
// We don't keep this information around after tiltfile execution finishes.
//
// This is incompatible with the overall technical direction of tilt as an
// apiserver.  Objects registered via the API (like KubernetesApplys) don't get
// this protection. And it's currently only limited to the main Tiltfile.
//
// A more compatible solution would be to have api server objects
// for the kubecontexts that tilt is aware of, and ways to mark them safe.
func (s State) IsAllowed(tf *v1alpha1.Tiltfile) bool {
	if tf.Name != model.MainTiltfileManifestName.String() {
		return true
	}

	if s.env == k8s.ProductNone || s.env.IsDevCluster() {
		return true
	}

	for _, c := range s.allowed {
		if c == s.context {
			return true
		}
	}

	return false
}

func MustState(model starkit.Model) State {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(model starkit.Model) (State, error) {
	var state State
	err := model.Load(&state)
	return state, err
}

// GetIsolatedKubeconfigPath returns the path to Tilt's isolated kubeconfig
// This is used by local() commands and other components that need KUBECONFIG path
func GetIsolatedKubeconfigPath() string {
	isolatedKubeconfigMutex.RLock()
	defer isolatedKubeconfigMutex.RUnlock()
	return isolatedKubeconfigPath
}

// GetTempKubeconfigPath is kept for compatibility - redirects to isolated path
func GetTempKubeconfigPath() string {
	return GetIsolatedKubeconfigPath()
}

// CleanupIsolatedKubeconfig removes Tilt's isolated kubeconfig file
func CleanupIsolatedKubeconfig() {
	isolatedKubeconfigMutex.Lock()
	defer isolatedKubeconfigMutex.Unlock()

	if isolatedKubeconfigPath != "" {
		os.Remove(isolatedKubeconfigPath)
		isolatedKubeconfigPath = ""
		// Restore original KUBECONFIG or unset if it was using default
		if originalKubeconfigPath != "" {
			os.Setenv("KUBECONFIG", originalKubeconfigPath)
		} else {
			os.Unsetenv("KUBECONFIG")
		}
		isolationInitialized = false
	}
}
