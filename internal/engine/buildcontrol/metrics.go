package buildcontrol

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	octag "go.opencensus.io/tag"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Metric aggregations
var keyResourceName = tag.MustNewKey("resource")
var keyHasError = octag.MustNewKey("error")

var K8sDeployDuration = stats.Float64(
	"k8s_deploy_duration",
	"K8s Deploy duration",
	stats.UnitMilliseconds)

var K8sDeployObjects = stats.Int64("objects", "The number of objects deployed", "1")

var K8sDeployDurationDistribution = view.Distribution(
	10, 100, 500, 1000, 2000, 5000,
	10000, 15000, 20000, 30000, 45000, 60000)

var K8sDeployDurationView = &view.View{
	Name:        "k8s_deploy_duration_dist",
	Measure:     K8sDeployDuration,
	Aggregation: K8sDeployDurationDistribution,
	Description: "K8s Deploy time",
	TagKeys:     []octag.Key{keyResourceName, keyHasError},
}

var K8sDeployCount = &view.View{
	Name:        "k8s_deploy_count",
	Measure:     K8sDeployDuration,
	Aggregation: view.Count(),
	Description: "K8s deploy count",
	TagKeys:     []octag.Key{keyResourceName, keyHasError},
}

var K8sDeployObjectsCount = &view.View{
	Name:        "k8s_deploy_objects_count",
	Measure:     K8sDeployObjects,
	Aggregation: view.LastValue(),
	Description: "K8s objects per resource",
	TagKeys:     []octag.Key{keyResourceName, keyHasError},
}

func reportK8sDeployMetrics(ctx context.Context, targetID model.TargetID, dur time.Duration,
	result store.K8sBuildResult, hasError bool) {
	latencyMs := float64(dur / time.Millisecond)
	errorTag := "0"
	if hasError {
		errorTag = "1"
	}
	var deployedCount int64
	if result.KubernetesApplyFilter != nil {
		deployedCount = int64(len(result.KubernetesApplyFilter.DeployedRefs))
	}

	recErr := stats.RecordWithTags(ctx,
		[]octag.Mutator{
			octag.Upsert(keyResourceName, targetID.Name.String()),
			octag.Upsert(keyHasError, errorTag),
		},
		K8sDeployDuration.M(latencyMs),
		K8sDeployObjects.M(deployedCount))
	if recErr != nil {
		logger.Get(ctx).Debugf("k8s deploy stats: %v", recErr)
	}
}
