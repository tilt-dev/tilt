package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
)

// Collects logs from deployed containers.
type PodLogManager struct {
	kClient k8s.Client
	dd      *DeployDiscovery
	store   *store.Store

	// TODO(nick): This should really be part of a reactive state store.
	watches map[docker.ImgNameAndTag]PodLogWatch
	mu      sync.Mutex
}

func NewPodLogManager(kClient k8s.Client, dd *DeployDiscovery, store *store.Store) *PodLogManager {
	return &PodLogManager{
		kClient: kClient,
		dd:      dd,
		store:   store,
		watches: make(map[docker.ImgNameAndTag]PodLogWatch),
	}
}

func (m *PodLogManager) PostProcessBuild(ctx context.Context, name model.ManifestName, result, previousResult store.BuildResult) {
	if previousResult.HasImage() && (!result.HasImage() || result.Image != previousResult.Image) {
		m.tearDownWatcher(previousResult)
	}

	if !result.HasImage() {
		// This is normal if the previous build failed.
		return
	}

	m.dd.EnsureDeployInfoFetchStarted(ctx, result.Image, result.Namespace)
	m.setUpWatcher(ctx, name, result)
}

func (m *PodLogManager) setUpWatcher(ctx context.Context, name model.ManifestName, result store.BuildResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := docker.ToImgNameAndTag(result.Image)
	w, ok := m.watches[key]
	if ok {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	w = PodLogWatch{ctx: ctx, cancel: cancel}
	m.watches[key] = w

	go m.consumeLogs(name, result, w)
}

func (m *PodLogManager) consumeLogs(name model.ManifestName, result store.BuildResult, watch PodLogWatch) {
	defer m.tearDownWatcher(result)

	deployInfo, err := m.dd.DeployInfoForImageBlocking(watch.ctx, result.Image)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	} else if deployInfo.Empty() {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: pod not found", name)
		return
	}

	pID := deployInfo.podID
	containerName := deployInfo.containerName
	ns := result.Namespace
	readCloser, err := m.kClient.ContainerLogs(watch.ctx, pID, containerName, ns)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
	defer func() {
		_ = readCloser.Close()
	}()

	logWriter := logger.Get(watch.ctx).Writer(logger.InfoLvl)
	prefixLogWriter := output.NewPrefixedWriter(fmt.Sprintf("[%s] ", name), logWriter)
	actionWriter := PodLogActionWriter{
		store:        m.store,
		manifestName: name,
		podID:        pID,
	}
	actionBufWriter := bufio.NewWriter(actionWriter)
	defer func() {
		_ = actionBufWriter.Flush()
	}()
	multiWriter := io.MultiWriter(prefixLogWriter, actionBufWriter)

	_, err = io.Copy(multiWriter, readCloser)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
}

func (m *PodLogManager) tearDownWatcher(result store.BuildResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := docker.ToImgNameAndTag(result.Image)
	w, ok := m.watches[key]
	if ok {
		w.cancel()
	}
	delete(m.watches, key)
}

type PodLogWatch struct {
	ctx    context.Context
	cancel func()
}

type PodLogActionWriter struct {
	store        *store.Store
	podID        k8s.PodID
	manifestName model.ManifestName
}

func (w PodLogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(PodLogAction{
		PodID:        w.podID,
		ManifestName: w.manifestName,
		Log:          append([]byte{}, p...),
	})
	return len(p), nil
}
