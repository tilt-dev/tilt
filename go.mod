module github.com/tilt-dev/tilt

go 1.16

require (
	cloud.google.com/go v0.56.0 // indirect
	github.com/adrg/xdg v0.3.3
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/akutz/memconn v0.1.0
	github.com/alessio/shellescape v1.2.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/bugsnag/bugsnag-go v1.5.3 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cloudflare/cfssl v1.4.1 // indirect
	github.com/compose-spec/compose-go v0.0.0-20210910070419-813d4ccb40f8
	github.com/containerd/continuity v0.0.0-20201208142359-180525291bb7
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/cli v20.10.5+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.0-ce-rc1.0.20200730172259-9f28837c1d93+incompatible
	github.com/docker/go v1.5.1-1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/fatih/color v1.9.0
	github.com/gdamore/tcell v1.1.3
	github.com/ghodss/yaml v1.0.0
	github.com/go-errors/errors v1.1.1 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.5.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/jinzhu/gorm v1.9.12 // indirect
	github.com/jonboulle/clockwork v0.2.2
	github.com/json-iterator/go v1.1.11
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/looplab/tarjan v0.0.0-20161115091335-9cc6d6cebfb5
	github.com/mattn/go-colorable v0.1.7
	github.com/mattn/go-isatty v0.0.12
	github.com/mattn/go-tty v0.0.3
	github.com/miekg/pkcs11 v0.0.0-20180817151620-df0db7a16a9e // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/buildkit v0.7.1-0.20200925001807-2b6cccb9b3e9
	github.com/modern-go/reflect2 v1.0.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/pelletier/go-toml v1.5.0 // indirect
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/browser v0.0.0-00010101000000-000000000000
	github.com/pkg/errors v0.9.1
	github.com/rivo/tview v0.0.0-20180926100353-bc39bf8d245d
	github.com/schollz/closestmatch v2.1.0+incompatible
	github.com/spf13/cobra v1.1.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/theupdateframework/notary v0.6.1 // indirect
	github.com/tilt-dev/dockerignore v0.0.0-20200910202654-0d8c17a73277
	github.com/tilt-dev/fsevents v0.0.0-20200515134857-2efe37af20de
	github.com/tilt-dev/fsnotify v1.4.8-0.20210701141043-dd524499d3fe
	github.com/tilt-dev/go-get v0.2.1
	github.com/tilt-dev/localregistry-go v0.0.0-20200615231835-07e386f4ebd7
	github.com/tilt-dev/probe v0.3.0
	github.com/tilt-dev/tilt-apiserver v0.5.0
	github.com/tilt-dev/wmclient v0.0.0-20201109174454-1839d0355fbc
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	github.com/wojas/genericr v0.2.0
	go.opencensus.io v0.22.4
	go.opentelemetry.io/otel v0.20.0
	go.opentelemetry.io/otel/sdk v0.20.0
	go.opentelemetry.io/otel/trace v0.20.0
	go.starlark.net v0.0.0-20200615180055-61b64bc45990
	golang.org/x/mod v0.4.2
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/dancannon/gorethink.v3 v3.0.5 // indirect
	gopkg.in/fatih/pool.v2 v2.0.0 // indirect
	gopkg.in/gorethink/gorethink.v3 v3.0.5 // indirect
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.6.2
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/apiserver v0.22.2
	k8s.io/cli-runtime v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/code-generator v0.22.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	k8s.io/kubectl v0.22.2
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.1
	sigs.k8s.io/kustomize/api v0.8.11
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible

	// This is just evanphx/json-patch v4.6.0 with a fix for
	// https://github.com/evanphx/json-patch/issues/98
	// so that we can pull it in correctly
	github.com/evanphx/json-patch => github.com/tilt-dev/json-patch/v4 v4.8.1 // indirect

	// Workaround for https://github.com/kubernetes-sigs/kustomize/issues/3262
	// Kustomize depends on go-openapi/spec, which made a backwards-incompatible
	// change: https://github.com/go-openapi/spec/commit/55f43acfece4ec21dd910b355e80e15d35960aa9
	// kubectl pulls in an old version of kustomize, which pulls in an old version of go-openapi, with which the new kustomize is incompatible
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.3

	// can remove if/when https://github.com/pkg/browser/pull/30 is merged
	github.com/pkg/browser => github.com/tilt-dev/browser v0.0.1

	go.opencensus.io => github.com/tilt-dev/opencensus-go v0.22.5-0.20200904175236-275b1754f353
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413

	k8s.io/apimachinery => github.com/tilt-dev/apimachinery v0.22.2-tilt-20210928
)
