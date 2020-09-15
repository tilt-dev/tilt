module github.com/tilt-dev/tilt

go 1.14

require (
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	github.com/Shopify/logrus-bugsnag v0.0.0-20171204204709-577dee27f20d // indirect
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/alessio/shellescape v1.2.2
	github.com/bitly/go-hostpool v0.1.0 // indirect
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/bugsnag/bugsnag-go v1.5.3 // indirect
	github.com/bugsnag/panicwrap v1.2.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/chai2010/gettext-go v0.0.0-20170215093142-bf70f2a70fb1 // indirect
	github.com/cloudflare/cfssl v1.4.1 // indirect
	github.com/containerd/cgroups v0.0.0-20191220161829-06e718085901 // indirect
	github.com/containerd/continuity v0.0.0-20191214063359-1097c8bae83b
	github.com/containerd/ttrpc v0.0.0-20191028202541-4f1b8fe65a5c // indirect
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/cli v0.0.0-20191220145525-ba63a92655c0
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20191210192822-1347481b9eb5
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go v1.5.1-1 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/fatih/color v1.7.0
	github.com/gdamore/tcell v1.1.3
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/gofrs/uuid v3.2.0+incompatible // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.4.0
	github.com/google/shlex v0.0.0-20181106134648-c34317bd91bf // indirect
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.3.0
	github.com/gorilla/mux v1.7.2
	github.com/gorilla/websocket v1.4.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/jinzhu/gorm v1.9.12 // indirect
	github.com/jonboulle/clockwork v0.1.0
	github.com/json-iterator/go v1.1.8
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/looplab/tarjan v0.0.0-20161115091335-9cc6d6cebfb5
	github.com/mattn/go-colorable v0.1.4
	github.com/mattn/go-isatty v0.0.10
	github.com/mattn/go-sqlite3 v2.0.2+incompatible // indirect
	github.com/mattn/go-tty v0.0.3
	github.com/miekg/pkcs11 v0.0.0-20180817151620-df0db7a16a9e // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/buildkit v0.6.3
	github.com/modern-go/reflect2 v1.0.1
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/runtime-spec v1.0.1 // indirect
	github.com/opentracing/opentracing-go v1.1.1-0.20190913142402-a7454ce5950e
	github.com/pkg/browser v0.0.0-20170505125900-c90ca0c84f15
	github.com/pkg/errors v0.9.1
	github.com/rivo/tview v0.0.0-20180926100353-bc39bf8d245d
	github.com/schollz/closestmatch v2.1.0+incompatible
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/theupdateframework/notary v0.6.1 // indirect
	github.com/tilt-dev/dockerignore v0.0.0-20200910202654-0d8c17a73277
	github.com/tilt-dev/fsevents v0.0.0-20200515134857-2efe37af20de
	github.com/tilt-dev/fsnotify v1.4.8-0.20200727200623-991e307aab7f
	github.com/tilt-dev/go-get v0.0.0-20200911222649-1acd29546527
	github.com/tilt-dev/localregistry-go v0.0.0-20200615231835-07e386f4ebd7
	github.com/tilt-dev/wmclient v0.0.0-20200901155816-d8d972f01eb9
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	go.opencensus.io v0.22.4
	go.opentelemetry.io/otel v0.2.0
	go.starlark.net v0.0.0-20200615180055-61b64bc45990
	golang.org/x/crypto v0.0.0-20200302210943-78000ba7a073 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	google.golang.org/genproto v0.0.0-20200527145253-8367513e4ece
	google.golang.org/grpc v1.29.1
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/dancannon/gorethink.v3 v3.0.5 // indirect
	gopkg.in/fatih/pool.v2 v2.0.0 // indirect
	gopkg.in/gorethink/gorethink.v3 v3.0.5 // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/cli-runtime v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/component-base v0.18.3
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.18.3
	sigs.k8s.io/kustomize v2.0.3+incompatible
	sigs.k8s.io/yaml v1.2.0
	vbom.ml/util v0.0.0-20180919145318-efcd4e0f9787 // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.3+incompatible

	// This is just evanphx/json-patch v4.6.0 with a fix for
	// https://github.com/evanphx/json-patch/issues/98
	// so that we can pull it in correctly
	github.com/evanphx/json-patch => github.com/tilt-dev/json-patch/v4 v4.8.1 // indirect
	// TODO(dmiller) remove this replace once https://github.com/moby/buildkit/pull/1297 is merged
	github.com/moby/buildkit => github.com/zachbadgett/buildkit v0.6.2-0.20191220071605-814e2794095f
	go.opencensus.io => github.com/tilt-dev/opencensus-go v0.22.5-0.20200904175236-275b1754f353
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413

	k8s.io/client-go => github.com/tilt-dev/client-go v0.0.0-20200326150806-41017343d309
)
