module github.com/windmilleng/tilt

require (
	cloud.google.com/go v0.27.0
	contrib.go.opencensus.io/exporter/ocagent v0.2.0
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
	github.com/Azure/go-autorest v11.4.0+incompatible
	github.com/Microsoft/go-winio v0.4.11
	github.com/Microsoft/hcsshim v0.6.14
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5
	github.com/Shopify/sarama v1.18.0
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412
	github.com/apache/thrift v0.0.0-20171203172758-327ebb6c2b6d
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/blang/semver v3.5.1+incompatible
	github.com/census-instrumentation/opencensus-proto v0.1.0
	github.com/containerd/console v0.0.0-20180822173158-c12b1e7919c1
	github.com/containerd/containerd v1.2.0-beta.1
	github.com/containerd/continuity v0.0.0-20180814194400-c7c5070e6f6e
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260
	github.com/containerd/typeurl v0.0.0-20180627222232-a93fcdb778cd
	github.com/davecgh/go-spew v1.1.1
	github.com/denisbrodbeck/machineid v1.0.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/cli v0.0.0-20180821235727-08cf36daa65e
	github.com/docker/distribution v0.0.0-20180827212453-059f301d548d
	github.com/docker/docker v0.0.0-20180828171745-a005332346dc
	github.com/docker/docker-credential-helpers v0.6.1
	github.com/docker/go v1.5.1-1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82
	github.com/docker/go-units v0.3.3
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29
	github.com/dustin/go-humanize v1.0.0
	github.com/eapache/go-resiliency v1.1.0
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21
	github.com/eapache/queue v1.1.0
	github.com/evanphx/json-patch v4.1.0+incompatible
	github.com/fatih/color v1.7.0
	github.com/gdamore/encoding v0.0.0-20151215212835-b23993cbb635
	github.com/gdamore/tcell v1.1.0
	github.com/go-logfmt/logfmt v0.3.0
	github.com/gobwas/glob v0.2.3
	github.com/gogo/googleapis v1.1.0
	github.com/gogo/protobuf v1.1.1
	github.com/golang/protobuf v1.2.0
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c
	github.com/google/go-cloud v0.0.0-20180911231741-1e2867e5b7c0
	github.com/google/go-cmp v0.2.0
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/google/uuid v1.0.0
	github.com/google/wire v0.2.1
	github.com/googleapis/gnostic v0.2.0
	github.com/gophercloud/gophercloud v0.0.0-20190130105114-cc9c99918988
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.6.2
	github.com/gorilla/websocket v1.4.0
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/go-version v1.0.0
	github.com/imdario/mergo v0.3.6
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go v1.1.5
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/lucasb-eyer/go-colorful v0.0.0-20180526135729-345fbb3dbcdb
	github.com/mattn/go-colorable v0.0.9
	github.com/mattn/go-isatty v0.0.3
	github.com/mattn/go-runewidth v0.0.3
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/miekg/pkcs11 v0.0.0-20180817151620-df0db7a16a9e
	github.com/moby/buildkit v0.0.0-20180828135223-94cc96a0ed84
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742
	github.com/monochromegane/go-gitignore v0.0.0-20160105113617-38717d0a108c
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.1.1
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/opentracing-contrib/go-observer v0.0.0-20170622124052-a52f23424492
	github.com/opentracing/opentracing-go v1.0.2
	github.com/openzipkin/zipkin-go-opentracing v0.3.2
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pierrec/lz4 v0.0.0-20180906185208-bb6bfd13c6a2
	github.com/pkg/browser v0.0.0-20170505125900-c90ca0c84f15
	github.com/pkg/errors v0.8.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/prometheus/common v0.0.0-20181126121408-4724e9255275
	github.com/prometheus/procfs v0.0.0-20181204211112-1dc9a6cbc91a
	github.com/rcrowley/go-metrics v0.0.0-20180503174638-e2704e165165
	github.com/rivo/tview v0.0.0-20180926100353-bc39bf8d245d
	github.com/sirupsen/logrus v1.0.6
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.2
	github.com/stretchr/testify v1.2.2
	github.com/syndtr/gocapability v0.0.0-20180223013746-33e07d32887e
	github.com/theupdateframework/notary v0.6.1
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa
	github.com/windmilleng/fsevents v0.0.0-20190206153914-2ad75e5ddeed
	github.com/windmilleng/fsnotify v1.4.7
	github.com/windmilleng/wmclient v0.0.0-20190205232421-1ae9d14ff820
	go.opencensus.io v0.18.0
	go.starlark.net v0.0.0-20181218181455-c0b6b768d91b
	golang.org/x/crypto v0.0.0-20180820150726-614d502a4dac
	golang.org/x/net v0.0.0-20181201002055-351d144fa1fc
	golang.org/x/oauth2 v0.0.0-20181203162652-d668ce993890
	golang.org/x/sync v0.0.0-20181108010431-42b317875d0f
	golang.org/x/sys v0.0.0-20180909124046-d0be0721c37e
	golang.org/x/text v0.3.0
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/api v0.1.0
	google.golang.org/appengine v1.3.0
	google.golang.org/genproto v0.0.0-20181202183823-bd91e49a0898
	google.golang.org/grpc v1.17.0
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v2 v2.2.1
	k8s.io/api v0.0.0-20190111032252-67edc246be36
	k8s.io/apimachinery v0.0.0-20181127025237-2b1284ed4c93
	k8s.io/client-go v0.0.0-20190111032708-6bf63545bd02
	k8s.io/klog v0.1.0
	k8s.io/kube-openapi v0.0.0-20180731170545-e3762e86a74c
	sigs.k8s.io/yaml v1.1.0
)
