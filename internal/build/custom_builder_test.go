package build

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var defaultCluster = &v1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{Name: "default"},
}
var TwoURLRegistry = &v1alpha1.RegistryHosting{
	Host:                     "localhost:1234",
	HostFromContainerRuntime: "registry:1234",
}

func TestCustomBuildSuccess(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("exit 0")
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefTaggedWithDigest(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["localhost:1234/foo_bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("exit 0")
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("foo/bar", TwoURLRegistry), cb.CmdImageSpec, nil)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/foo_bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/foo_bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefWithCustomTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:my-tag"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("exit 0")
	cb.CmdImageSpec.OutputTag = "my-tag"
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("gcr.io/foo/bar", TwoURLRegistry), cb.CmdImageSpec, nil)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/gcr.io_foo_bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/gcr.io_foo_bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	cb := f.customBuild("exit 0")
	cb.CmdImageSpec.OutputMode = v1alpha1.CmdImageOutputRemote
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NoError(f.t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-build-1551202573"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-build-1551202573"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefTaggedIfSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	cb := f.customBuild("exit 0")
	cb.CmdImageSpec.OutputMode = v1alpha1.CmdImageOutputRemote
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("foo/bar", TwoURLRegistry), cb.CmdImageSpec, nil)
	require.NoError(f.t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/foo_bar:tilt-build-1551202573"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/foo_bar:tilt-build-1551202573"), refs.ClusterRef)
}

func TestCustomBuildCmdFails(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	cb := f.customBuild("exit 1")
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	// TODO(dmiller) better error message
	assert.EqualError(t, err, "Custom build command failed: exit status 1")
}

func TestCustomBuildImgNotFound(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	cb := f.customBuild("exit 0")
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	assert.Contains(t, err.Error(), "fake docker client error: object not found")
}

func TestCustomBuildExpectedTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:the-tag"] = types.ImageInspect{ID: string(sha)}

	cb := f.customBuild("exit 0")
	cb.CmdImageSpec.OutputTag = "the-tag"
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuilderExecsRelativeToTiltfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no sh on windows")
	}
	f := newFakeCustomBuildFixture(t)

	f.WriteFile("proj/build.sh", "exit 0")

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("./build.sh")
	cb.CmdImageSpec.Dir = filepath.Join(f.Path(), "proj")
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
}

func TestCustomBuildOutputsToImageRefSuccess(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	myTag := "gcr.io/foo/bar:dev"
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images[myTag] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("echo gcr.io/foo/bar:dev > ref.txt")
	cb.CmdImageSpec.OutputsImageRefTo = f.JoinPath("ref.txt")
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed(myTag), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed(myTag), refs.ClusterRef)
}

func TestCustomBuildOutputsToImageRefMissingImage(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	myTag := "gcr.io/foo/bar:dev"
	cb := f.customBuild(fmt.Sprintf("echo %s > ref.txt", myTag))
	cb.CmdImageSpec.OutputsImageRefTo = f.JoinPath("ref.txt")
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(),
		fmt.Sprintf("fake docker client error: object not found (fakeClient.Images key: %s)", myTag))
}

func TestCustomBuildOutputsToImageRefMalformedImage(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	cb := f.customBuild("echo 999 > ref.txt")
	cb.CmdImageSpec.OutputsImageRefTo = f.JoinPath("ref.txt")
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(),
		fmt.Sprintf("Output image ref in file %s was invalid: Expected reference \"999\" to contain a tag",
			f.JoinPath("ref.txt")))
}

func TestCustomBuildOutputsToImageRefSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	myTag := "gcr.io/foo/bar:dev"
	cb := f.customBuild(fmt.Sprintf("echo %s > ref.txt", myTag))
	cb.CmdImageSpec.OutputsImageRefTo = f.JoinPath("ref.txt")
	cb.CmdImageSpec.OutputMode = v1alpha1.CmdImageOutputRemote
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, nil)
	require.NoError(t, err)
	assert.Equal(f.t, container.MustParseNamed(myTag), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed(myTag), refs.ClusterRef)
}

func TestCustomBuildImageDep(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no sh on windows")
	}

	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild("echo $TILT_IMAGE_0 > image-0.txt")
	cb.CmdImageSpec.ImageMaps = []string{"base"}

	imageMaps := map[ktypes.NamespacedName]*v1alpha1.ImageMap{
		ktypes.NamespacedName{Name: "base"}: &v1alpha1.ImageMap{
			Status: v1alpha1.ImageMapStatus{
				ImageFromLocal: "base:tilt-12345",
			},
		},
	}

	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb.CmdImageSpec, imageMaps)
	require.NoError(t, err)

	assert.Equal(f.t, "base:tilt-12345", strings.TrimSpace(f.ReadFile("image-0.txt")))
}

func TestEnvVars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no sh on windows")
	}

	expectedVars := map[string]string{
		"EXPECTED_REF":      "localhost:1234/foo_bar:tilt-build-1551202573",
		"EXPECTED_REGISTRY": "localhost:1234",
		"EXPECTED_IMAGE":    "foo_bar",
		"EXPECTED_TAG":      "tilt-build-1551202573",
		"REGISTRY_HOST":     "localhost:1234",
	}
	var script []string
	for k, v := range expectedVars {
		script = append(script, fmt.Sprintf(
			`if [ "${%s}" != "%s" ]; then >&2 printf "%s:\n\texpected: %s\n\tactual:   ${%s}\n"; exit 1; fi`,
			k, v, k, v, k))
	}

	f := newFakeCustomBuildFixture(t)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["localhost:1234/foo_bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild(strings.Join(script, "\n"))
	_, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("foo/bar", TwoURLRegistry), cb.CmdImageSpec, nil)
	require.NoError(t, err)
}

func TestEnvVars_ConfigRefWithLocalRegistry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no sh on windows")
	}

	// generally, config refs (value in Tiltfile) are $prod_registry/$image:$tag
	// and Tilt rewrites it to $local_registry/$sanitized_prod_registry_$image
	// however, some users explicitly use the $local_registry in their Tiltfile
	// refs, so instead of producing a redudant and confusing ref like
	// $local_registry/$sanitized_local_registry_$image, it just gets passed
	// through
	expectedVars := map[string]string{
		"EXPECTED_REF":      "localhost:1234/foo/bar:tilt-build-1551202573",
		"EXPECTED_REGISTRY": "localhost:1234",
		"EXPECTED_IMAGE":    "foo/bar",
		"EXPECTED_TAG":      "tilt-build-1551202573",
		"REGISTRY_HOST":     "localhost:1234",
	}
	var script []string
	for k, v := range expectedVars {
		script = append(script, fmt.Sprintf(
			`if [ "${%s}" != "%s" ]; then >&2 printf "%s:\n\texpected: %s\n\tactual:   ${%s}\n"; exit 1; fi`,
			k, v, k, v, k))
	}

	f := newFakeCustomBuildFixture(t)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["localhost:1234/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := f.customBuild(strings.Join(script, "\n"))
	_, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("localhost:1234/foo/bar", TwoURLRegistry), cb.CmdImageSpec, nil)
	require.NoError(t, err)
}

type fakeCustomBuildFixture struct {
	*tempdir.TempDirFixture

	t    *testing.T
	ctx  context.Context
	dCli *docker.FakeClient
	cb   *CustomBuilder
}

func newFakeCustomBuildFixture(t *testing.T) *fakeCustomBuildFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dCli := docker.NewFakeClient()
	clock := fakeClock{
		now: time.Unix(1551202573, 0),
	}

	cb := NewCustomBuilder(dCli, clock)

	return &fakeCustomBuildFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
		dCli:           dCli,
		cb:             cb,
	}
}

func (f *fakeCustomBuildFixture) customBuild(args string) model.CustomBuild {
	return model.CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{
			Args: model.ToHostCmd(args).Argv,
			Dir:  f.Path(),
		},
	}
}

func refSetFromString(s string) container.RefSet {
	sel := container.MustParseSelector(s)
	return container.MustSimpleRefSet(sel)
}

func refSetWithRegistryFromString(ref string, reg *v1alpha1.RegistryHosting) container.RefSet {
	r, err := container.NewRefSet(container.MustParseSelector(ref), reg)
	if err != nil {
		panic(err)
	}
	return r
}
