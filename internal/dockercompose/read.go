package dockercompose

import (
	"context"
	"strings"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

func ReadConfigAndServiceNames(ctx context.Context, dcc DockerComposeClient,
	configPaths []string) (conf Config, svcNames []string, err error) {
	// calls to `docker-compose config` take a bit, and we need two,
	// so do them in parallel to make things faster
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {

		configOut, err := dcc.Config(ctx, configPaths)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal([]byte(configOut), &conf)
		if err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		var err error
		svcNames, err = serviceNames(ctx, dcc, configPaths)
		if err != nil {
			return err
		}
		return nil
	})

	err = g.Wait()
	return conf, svcNames, err
}

func serviceNames(ctx context.Context, dcc DockerComposeClient, configPaths []string) ([]string, error) {
	servicesText, err := dcc.Services(ctx, configPaths)
	if err != nil {
		return nil, err
	}

	serviceNames := strings.Split(servicesText, "\n")

	var result []string

	for _, name := range serviceNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		result = append(result, name)
	}

	return result, nil
}
