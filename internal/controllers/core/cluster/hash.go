package cluster

import (
	"encoding/binary"
	"hash/fnv"
	"strconv"

	"k8s.io/client-go/rest"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"

	"github.com/tilt-dev/tilt/internal/k8s"
)

// hashClientConfig produces a hash from relevant cluster connection fields.
func hashClientConfig(config *rest.Config, ns k8s.Namespace) cluster.ClientConfigHash {
	h := fnv.New64a()

	// in practice, what we _really_ care about is the host/namespace changing
	// since that means we actually changed from one cluster to another (or
	// "logical" cluster in the case of namespace)
	//
	// changing the auth portion of a cluster config (much less other settings)
	// isn't really a goal to support, but we include many of those fields to
	// at least make an attempt; this isn't comprehensive/sufficient (e.g. the
	// path to a cert file might not change but its contents could have!)
	fields := []interface{}{
		ns.String(),
		config.Host,
		config.APIPath,
		config.BearerToken,
		config.BearerTokenFile,
		config.Username,
		config.Password,
		config.TLSClientConfig.CertFile,
		config.TLSClientConfig.CertData,
		config.TLSClientConfig.CAFile,
		config.TLSClientConfig.CAData,
		config.TLSClientConfig.KeyFile,
		config.TLSClientConfig.KeyData,
		config.TLSClientConfig.Insecure,
		config.TLSClientConfig.ServerName,
	}

	for _, f := range fields {
		if fStr, ok := f.(string); ok {
			f = []byte(fStr)
		}

		if err := binary.Write(h, binary.LittleEndian, f); err != nil {
			// actual write to fnv hash cannot fail
			// the binary::Write call can only fail if given an invalid type,
			// but we are passing valid types above, so panic if this invariant
			// ever changes
			panic(err)
		}
	}

	return cluster.ClientConfigHash(strconv.FormatUint(h.Sum64(), 10))
}
