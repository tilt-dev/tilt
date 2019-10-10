package cli

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cobra"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/minikube"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/logger"
)

type dockerPruneCmd struct {
	untilStr string
}

func (c *dockerPruneCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-prune",
		Short: "run docker prune as tilt does",
	}

	cmd.Flags().StringVar(&c.untilStr, "until", "12h", "max age of image to keep (as go duration string, e.g. 1h30m, 12h")

	return cmd
}

func (c *dockerPruneCmd) run(ctx context.Context, args []string) error {
	// a := analytics.Get(ctx)
	// a.Incr("cmd.dockerPrune", map[string]string{
	// 	"watch": fmt.Sprintf("%v", c.watch),
	// 	"mode":  string(updateModeFlag),
	// })
	// a.IncrIfUnopted("analytics.up.optdefault")
	// defer a.Flush(time.Second)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Up")
	defer span.Finish()

	// tags := tracer.TagStrToMap(c.traceTags)

	// for k, v := range tags {
	// 	span.SetTag(k, v)
	// }

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	dCli, err := provideDockerClient(ctx)
	if err != nil {
		return err
	}

	until, err := time.ParseDuration(c.untilStr)
	if err != nil {
		return err
	}

	err = dCli.Prune(ctx, until)
	if err != nil {
		return err
	}
	return nil
}

func provideDockerClient(ctx context.Context) (docker.Client, error) {
	clientConfig := k8s.ProvideClientConfig()
	config, err := k8s.ProvideKubeConfig(clientConfig)
	env := k8s.ProvideEnv(ctx, config)
	restConfigOrError := k8s.ProvideRESTConfig(clientConfig)
	clientsetOrError := k8s.ProvideClientset(restConfigOrError)
	portForwardClient := k8s.ProvidePortForwardClient(restConfigOrError, clientsetOrError)
	namespace := k8s.ProvideConfigNamespace(clientConfig)
	kubeContext, err := k8s.ProvideKubeContext(config)
	if err != nil {
		return nil, err
	}
	int2 := provideKubectlLogLevel()
	kubectlRunner := k8s.ProvideKubectlRunner(kubeContext, int2)
	client := k8s.ProvideK8sClient(ctx, env, restConfigOrError, clientsetOrError, portForwardClient, namespace, kubectlRunner, clientConfig)
	runtime := k8s.ProvideContainerRuntime(ctx, client)
	minikubeClient := minikube.ProvideMinikubeClient()
	clusterEnv, err := docker.ProvideClusterEnv(ctx, env, runtime, minikubeClient)
	if err != nil {
		return nil, err
	}
	localEnv, err := docker.ProvideLocalEnv(ctx, clusterEnv)
	if err != nil {
		return nil, err
	}
	localClient := docker.ProvideLocalCli(ctx, localEnv)
	clusterClient, err := docker.ProvideClusterCli(ctx, localEnv, clusterEnv, localClient)
	if err != nil {
		return nil, err
	}
	return docker.ProvideSwitchCli(clusterClient, localClient), nil
}
