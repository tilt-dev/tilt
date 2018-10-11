package engine

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
)

// Looks up containers after they've been deployed.
type DeployDiscovery struct {
	kCli       k8s.Client
	deployInfo map[docker.ImgNameAndTag]*deployInfoEntry
	mu         sync.Mutex
}

func NewDeployDiscovery(kCli k8s.Client) *DeployDiscovery {
	return &DeployDiscovery{
		kCli:       kCli,
		deployInfo: make(map[docker.ImgNameAndTag]*deployInfoEntry),
	}
}

func (d *DeployDiscovery) EnsureDeployInfoFetchStarted(ctx context.Context, img reference.NamedTagged, ns k8s.Namespace) {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, ok := d.deployInfo[docker.ToImgNameAndTag(img)]
	if !ok {
		deployInfo := newDeployInfoEntry()
		d.deployInfo[docker.ToImgNameAndTag(img)] = deployInfo

		go func() {
			err := d.populateDeployInfo(ctx, img, ns, deployInfo)
			if err != nil {
				// There's a variety of reasons why we might not be able to get the deploy info.
				// The cluster could be in a transient bad state, or the pod
				// could be in a crash loop because the user wrote some code that
				// segfaults. Don't worry too much about it, we'll fall back to an image build.
				logger.Get(ctx).Debugf("failed to get deployInfo: %v", err)
			}
		}()
	}
}

func (d *DeployDiscovery) DeployInfoForImageBlocking(ctx context.Context, img reference.NamedTagged) (DeployInfo, error) {
	d.mu.Lock()
	deployInfo := d.deployInfo[docker.ToImgNameAndTag(img)]
	d.mu.Unlock()

	if deployInfo == nil {
		return DeployInfo{}, nil
	}

	deployInfo.waitUntilReady(ctx)
	return deployInfo.DeployInfo, deployInfo.err
}

// Returns the deploy info that was forgotten, if any.
func (d *DeployDiscovery) ForgetImage(img reference.NamedTagged) DeployInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := docker.ToImgNameAndTag(img)
	deployInfo := d.deployInfo[key]
	delete(d.deployInfo, key)
	if deployInfo == nil {
		return DeployInfo{}
	}
	return deployInfo.DeployInfo
}

func (d *DeployDiscovery) populateDeployInfo(ctx context.Context, image reference.NamedTagged, ns k8s.Namespace, info *deployInfoEntry) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DeployDiscovery-populateDeployInfo")
	defer span.Finish()

	defer func() {
		info.err = err
		info.markReady()
	}()

	// get pod running the image we just deployed.
	//
	// We fetch the pod by the NamedTagged, to ensure we get a pod
	// in the most recent Deployment, and not the pods in the process
	// of being terminated from previous Deployments.
	pods, err := d.kCli.PollForPodsWithImage(
		ctx, image, ns,
		[]k8s.LabelPair{TiltRunLabel()}, podPollTimeoutSynclet)
	if err != nil {
		return errors.Wrapf(err, "PodsWithImage (img = %s)", image)
	}

	// If there's more than one pod, two possible things could be happening:
	// 1) K8s is in a transitiion state.
	// 2) The user is running a configuration where they want multiple replicas
	//    of the same pod (e.g., a cockroach developer testing primary/replica).
	// If this happens, don't bother populating the deployInfo.
	// We want to fallback to image builds rather than managing the complexity
	// of multiple replicas.
	if len(pods) != 1 {
		logger.Get(ctx).Debugf("Found too many pods (%d), skipping container updates: %s", len(pods), image)
		return nil
	}

	pod := &(pods[0])
	pID := k8s.PodIDFromPod(pod)
	nodeID := k8s.NodeIDFromPod(pod)

	// Make sure that the deployed image is ready and not crashlooping.
	cStatus, err := k8s.WaitForContainerReady(ctx, d.kCli, pod, image)
	if err != nil {
		return errors.Wrapf(err, "WaitForContainerReady (pod = %s)", pID)
	}

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		return errors.Wrapf(err, "populateDeployInfo")
	}

	cName := k8s.ContainerNameFromContainerStatus(cStatus)

	logger.Get(ctx).Verbosef("talking to synclet client for pod %s", pID.String())

	info.podID = pID
	info.containerID = cID
	info.containerName = cName
	info.nodeID = nodeID

	return nil
}

type DeployInfo struct {
	podID         k8s.PodID
	containerID   k8s.ContainerID
	containerName k8s.ContainerName
	nodeID        k8s.NodeID
}

func (d DeployInfo) Empty() bool {
	return d == DeployInfo{}
}

type deployInfoEntry struct {
	DeployInfo

	ready chan struct{} // Close this channel when the DeployInfo is populated
	err   error         // error encountered when populating (if any)
}

func newDeployInfoEntry() *deployInfoEntry {
	return &deployInfoEntry{ready: make(chan struct{})}
}

func (di *deployInfoEntry) markReady() { close(di.ready) }
func (di *deployInfoEntry) waitUntilReady(ctx context.Context) {
	select {
	case <-di.ready:
	case <-ctx.Done():
	}
}
