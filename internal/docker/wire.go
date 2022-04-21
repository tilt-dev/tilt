package docker

import "github.com/google/wire"

func ProvideLocalAsDefault(cli LocalClient) Client {
	return Client(cli)
}
func ProvideClusterAsDefault(cli ClusterClient) Client {
	return Client(cli)
}

var ClientCreatorWireSet = wire.NewSet(
	wire.Value(RealClientCreator{}),
	wire.Bind(new(ClientCreator), new(RealClientCreator)))

// Bind a docker client that can either talk to the in-cluster
// Docker daemon or to the local Docker daemon.
var SwitchWireSet = wire.NewSet(
	ProvideClusterCli,
	ProvideLocalCli,
	ProvideSwitchCli,
	ProvideLocalEnv,
	ProvideClusterEnv,
	wire.Bind(new(Client), new(CompositeClient)),
	ClientCreatorWireSet)

// Bind a docker client that talks to the in-cluster Docker daemon.
var ClusterWireSet = wire.NewSet(
	ProvideClusterCli,
	ProvideLocalCli,
	ProvideLocalEnv,
	ProvideClusterEnv,
	ProvideClusterAsDefault,
	ClientCreatorWireSet)

// Bind a docker client that can only talk to the local Docker daemon.
var LocalWireSet = wire.NewSet(
	ProvideLocalCli,
	ProvideLocalEnv,
	ProvideEmptyClusterEnv,
	ProvideLocalAsDefault,
	ClientCreatorWireSet)

func ProvideEmptyClusterEnv() ClusterEnv {
	return ClusterEnv{}
}
