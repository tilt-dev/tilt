package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/filesync"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fsutiltypes "github.com/tonistiigi/fsutil/types"
)

var useCache bool

// A small utility for running Buildkit on the dockerfile
// in the current directory printing out all the buildkit api
// response protobufs.
func main() {
	flag.BoolVar(&useCache, "cache", false, "Enable docker caching")
	flag.Parse()

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	d, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	d.NegotiateAPIVersion(ctx)

	session, err := session.NewSession(ctx, "tilt", identity.NewID())
	if err != nil {
		return err
	}

	fileMap := func(path string, s *fsutiltypes.Stat) fsutil.MapResult {
		s.Uid = 0
		s.Gid = 0
		return fsutil.MapResultKeep
	}

	dir, _ := os.Getwd()
	session.Allow(filesync.NewFSSyncProvider([]filesync.SyncedDir{
		{
			Name: "context",
			Dir:  dir,
			Map:  fileMap,
		},
		{
			Name: "dockerfile",
			Dir:  dir,
		},
	}))

	go func() {
		defer func() {
			_ = session.Close()
		}()

		// Start the server
		dialSession := func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			return d.DialHijack(ctx, "/session", proto, meta)
		}
		_ = session.Run(ctx, dialSession)
	}()

	opts := types.ImageBuildOptions{}
	opts.Version = types.BuilderBuildKit
	opts.Dockerfile = "Dockerfile"
	opts.RemoteContext = "client-session"
	opts.SessionID = session.ID()
	if !useCache {
		opts.NoCache = true
	}
	defer session.Close()

	response, err := d.ImageBuild(ctx, nil, opts)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	return readDockerOutput(ctx, response.Body)
}

func readDockerOutput(ctx context.Context, reader io.Reader) error {
	decoder := json.NewDecoder(reader)

	for decoder.More() {
		message := jsonmessage.JSONMessage{}
		err := decoder.Decode(&message)
		if err != nil {
			return errors.Wrap(err, "decoding docker output")
		}

		if messageIsFromBuildkit(message) {
			err := writeBuildkitStatus(message.Aux)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeBuildkitStatus(aux *json.RawMessage) error {
	var resp controlapi.StatusResponse
	var dt []byte
	if err := json.Unmarshal(*aux, &dt); err != nil {
		return err
	}
	if err := (&resp).Unmarshal(dt); err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

func messageIsFromBuildkit(msg jsonmessage.JSONMessage) bool {
	return msg.ID == "moby.buildkit.trace"
}
