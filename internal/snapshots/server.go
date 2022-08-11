package snapshots

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/tilt-dev/tilt/pkg/assets"
	"github.com/tilt-dev/tilt/pkg/model"
	pkgsnapshot "github.com/tilt-dev/tilt/pkg/snapshot"
)

func Serve(ctx context.Context, l net.Listener, rawSnapshot []byte) error {
	buf := bytes.NewReader(rawSnapshot)
	var snapshot map[string]interface{}

	err := json.NewDecoder(buf).Decode(&snapshot)
	if err != nil {
		return err
	}

	version, err := pkgsnapshot.GetVersionFromSnapshot(snapshot)
	if err != nil {
		return err
	}

	ss, err := newSnapshotServer(rawSnapshot, version)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = ss.server.Shutdown(context.Background())
	}()

	err = ss.serve(l)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type snapshotServer struct {
	assetServer assets.Server
	snapshot    []byte
	server      http.Server
}

func newSnapshotServer(snapshot []byte, version string) (*snapshotServer, error) {
	result := &snapshotServer{}

	s, err := assets.NewProdServer(assets.ProdAssetBucket, model.WebVersion(version))
	if err != nil {
		return result, err
	}
	result.assetServer = s
	result.snapshot = snapshot

	return result, nil
}

func (ss *snapshotServer) serve(l net.Listener) error {
	m := http.NewServeMux()

	m.HandleFunc("/api/snapshot/local", ss.snapshotJSONHandler(ss.snapshot))
	m.HandleFunc("/", ss.assetServer.ServeHTTP)

	ss.server = http.Server{Handler: m}
	return ss.server.Serve(l)
}

func (ss *snapshotServer) snapshotJSONHandler(snapshot []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		_, err := w.Write(snapshot)
		if err != nil {
			http.Error(w, fmt.Sprintf("error writing snapshot to http response: %v", err.Error()), http.StatusInternalServerError)
		}
	}
}
