package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
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

// TODO(nick): Delete this method. Ideally, the state management that this function
// does should be part of the handlePodEvent state reducer, and this class will
// simply manage control flow (i.e., closing the ready channel when the container is ready.)
func (d *DeployDiscovery) populateDeployInfo(ctx context.Context, image reference.NamedTagged, ns k8s.Namespace, info *deployInfoEntry) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "DeployDiscovery-populateDeployInfo")
	defer span.Finish()

	defer func() {
		info.err = err
		info.markReady()
	}()

	nID, pID, cID, cName, err := podInfoForImage(ctx, d.kCli, image, ns)
	if err != nil {
		return err
	}

	info.nodeID = nID
	info.podID = pID
	info.containerID = cID
	info.containerName = cName

	return nil
}

func podInfoForImage(ctx context.Context, kCli k8s.Client, image reference.NamedTagged, ns k8s.Namespace) (k8s.NodeID, k8s.PodID, container.ID, container.Name, error) {
	// get pod running the image we just deployed.
	//
	// We fetch the pod by the NamedTagged, to ensure we get a pod
	// in the most recent Deployment, and not the pods in the process
	// of being terminated from previous Deployments.
	pods, err := kCli.PollForPodsWithImage(
		ctx, image, ns,
		[]k8s.LabelPair{TiltRunLabel()}, podPollTimeoutSynclet)
	if err != nil {
		return "", "", "", "", errors.Wrapf(err, "PodsWithImage (img = %s)", image)
	}

	// If there's more than one pod, two possible things could be happening:
	// 1) K8s is in a transition state.
	// 2) The user is running a configuration where they want multiple replicas
	//    of the same pod (e.g., a cockroach developer testing primary/replica).
	// If this happens, don't bother populating the deployInfo.
	// We want to fall back to image builds rather than managing the complexity
	// of multiple replicas.
	if len(pods) != 1 {
		logger.Get(ctx).Debugf("Found too many pods (%d), skipping container updates: %s", len(pods), image)
		return "", "", "", "", nil
	}

	pod := &(pods[0])
	pID := k8s.PodIDFromPod(pod)
	nID := k8s.NodeIDFromPod(pod)

	// Make sure that the deployed image is ready and not crashlooping.
	cStatus, err := k8s.WaitForContainerReady(ctx, kCli, pod, image)
	if err != nil {
		return "", "", "", "", errors.Wrapf(err, "WaitForContainerReady (pod = %s)", pID)
	}

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		return "", "", "", "", errors.Wrapf(err, "populateDeployInfo")
	} else if cID == "" {
		return "", "", "", "", fmt.Errorf("missing container ID: %+v", cStatus)
	}

	cName := k8s.ContainerNameFromContainerStatus(cStatus)

	return nID, pID, cID, cName, nil
}

type DeployInfo struct {
	nodeID        k8s.NodeID
	podID         k8s.PodID
	containerID   container.ID
	containerName container.Name
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
