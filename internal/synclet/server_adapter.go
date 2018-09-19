package synclet

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/proto"
)

type GRPCServer struct {
	del *Synclet
}

func NewGRPCServer(del *Synclet) *GRPCServer {
	return &GRPCServer{del: del}
}

var _ proto.SyncletServer = &GRPCServer{}

func logLevelToProto(level logger.Level) (proto.LogLevel, error) {
	var protoLevel proto.LogLevel
	switch level {
	case logger.InfoLvl:
		protoLevel = proto.LogLevel_INFO
	case logger.VerboseLvl:
		protoLevel = proto.LogLevel_VERBOSE
	case logger.DebugLvl:
		protoLevel = proto.LogLevel_DEBUG
	default:
		return proto.LogLevel_INFO, fmt.Errorf("unknown log level '%v'", level)
	}

	return protoLevel, nil
}

func makeContext(ctx context.Context, pCtx *proto.Context, f func(m *proto.LogMessage) error) (context.Context, error) {
	writeLog := func(level logger.Level, bytes []byte) error {
		protoLevel, err := logLevelToProto(level)
		if err != nil {
			return err
		}

		logMessage := &proto.LogMessage{Level: protoLevel, Message: bytes}
		return f(logMessage)
	}

	l := logger.NewFuncLogger(pCtx.LogColorsEnabled, writeLog)

	// TODO(matt) making a new outputter here is hacky - since outputter is stateful, someone might make
	// rely on state persisting across service boundaries
	ret := output.WithOutputter(logger.WithLogger(ctx, l), output.NewOutputter(l))
	return ret, nil
}

func (s *GRPCServer) GetContainerIdForPod(req *proto.GetContainerIdForPodRequest, server proto.Synclet_GetContainerIdForPodServer) error {

	send := func(m *proto.LogMessage) error {
		return server.Send(&proto.GetContainerIdForPodReply{Content: &proto.GetContainerIdForPodReply_Message{Message: m}})
	}

	ctx, err := makeContext(server.Context(), req.Context, send)
	if err != nil {
		return err
	}

	name, err := reference.ParseNamed(req.ImageId)
	if err != nil {
		return err
	}

	ref, ok := name.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("Expected a tagged ref: %s", req.ImageId)
	}

	containerID, err := s.del.ContainerIDForPod(ctx, k8s.PodID(req.PodId), ref)
	if err != nil {
		return err
	}

	return server.Send(&proto.GetContainerIdForPodReply{Content: &proto.GetContainerIdForPodReply_ContainerId{ContainerId: string(containerID)}})
}

func (s *GRPCServer) UpdateContainer(req *proto.UpdateContainerRequest, server proto.Synclet_UpdateContainerServer) error {
	var commands []model.Cmd
	for _, cmd := range req.Commands {
		commands = append(commands, model.Cmd{Argv: cmd.Argv})
	}

	send := func(m *proto.LogMessage) error {
		return server.Send(&proto.UpdateContainerReply{LogMessage: m})
	}

	ctx, err := makeContext(server.Context(), req.Context, send)
	if err != nil {
		return err
	}

	return s.del.UpdateContainer(ctx, k8s.ContainerID(req.ContainerId), req.TarArchive, req.FilesToDelete, commands)
}
