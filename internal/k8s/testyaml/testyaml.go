package testyaml

import (
	"strings"
)

const BlorgBackendYAML = `
apiVersion: v1
kind: Service
metadata:
  name: devel-nick-lb-blorg-be
  labels:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
spec:
  type: LoadBalancer
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: devel-nick-blorg-be
spec:
  selector:
    matchLabels:
      app: blorg
      owner: nick
      environment: devel
      tier: backend
  template:
    metadata:
      name: devel-nick-blorg-be
      labels:
        app: blorg
        owner: nick
        environment: devel
        tier: backend
    spec:
      containers:
      - name: backend
        imagePullPolicy: Always
        image: gcr.io/blorg-dev/blorg-backend:devel-nick
        command: [
          "/app/server",
          "--dbAddr", "hissing-cockroach-cockroachdb:26257"
        ]
        ports:
        - containerPort: 8080
`

const BlorgBackendAmbiguousYAML = `
apiVersion: v1
kind: Service
metadata:
  name: blorg 
  labels:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
spec:
  type: LoadBalancer
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: blorg 
spec:
  selector:
    matchLabels:
      app: blorg
      owner: nick
      environment: devel
      tier: backend
  template:
    metadata:
      name: blorg 
      labels:
        app: blorg
        owner: nick
        environment: devel
        tier: backend
    spec:
      containers:
      - name: backend
        imagePullPolicy: Always
        image: gcr.io/blorg-dev/blorg
        command: [
          "/app/server",
          "--dbAddr", "hissing-cockroach-cockroachdb:26257"
        ]
        ports:
        - containerPort: 8080
`

const BlorgJobYAML = `apiVersion: batch/v1
kind: Job
metadata:
  name: blorg-job
spec:
  template:
    spec:
      containers:
      - name: blorg-job
        image: gcr.io/blorg-dev/blorg-backend:devel-nick
        command: ["/app/server",  "-job=clean"]
      restartPolicy: Never
  backoffLimit: 4
`

const SanchoImage = "gcr.io/some-project-162817/sancho"

const SanchoYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoTwoContainersOneImageYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho-2c1i
  namespace: sancho-ns
  labels:
    app: sancho-2c1i
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho-2c1i
  template:
    metadata:
      labels:
        app: sancho-2c1i
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
      - name: sancho2
        image: gcr.io/some-project-162817/sancho
`

const SanchoYAMLWithCommand = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        command: ["foo.sh"]
        args: ["something", "something_else"]
`

const SanchoBeta1YAML = `
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoStatefulSetBeta1YAML = `
apiVersion: apps/v1beta1
kind: StatefulSet
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
`

const SanchoBeta2YAML = `
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoExtBeta1YAML = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoTwinYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho-twin
  namespace: sancho-ns
  labels:
    app: sancho-twin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho-twin
  template:
    metadata:
      labels:
        app: sancho-twin
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoSidecarYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
      - name: sancho-sidecar
        image: gcr.io/some-project-162817/sancho-sidecar
`
const SanchoSidecarImage = "gcr.io/some-project-162817/sancho-sidecar"

const SanchoRedisSidecarYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
      - name: redis-sidecar
        image: redis:latest
`

const SanchoImageInEnvYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  namespace: sancho-ns
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: foo
            value: gcr.io/some-project-162817/sancho2
          - name: bar
            value: gcr.io/some-project-162817/sancho
`

const TracerYAML = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: tracer-prod
spec:
  replicas: 1
  revisionHistoryLimit: 2
  template:
    metadata:
      labels:
        app: tracer
        track: prod
    spec:
      nodeSelector:
        cloud.google.com/gke-nodepool: default-pool

      containers:
      - name: tracer
        image: openzipkin/zipkin
        ports:
        - name: http
          containerPort: 9411
        livenessProbe:
          httpGet:
            path: /
            port: 9411
          initialDelaySeconds: 60
          periodSeconds: 60
        readinessProbe:
          httpGet:
            path: /
            port: 9411
          initialDelaySeconds: 30
          periodSeconds: 1
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 10
---
apiVersion: v1
kind: Service
metadata:
  name: tracer-prod
  labels:
    app: tracer
    track: prod
spec:
  selector:
    app: tracer
    track: prod
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: http
---
apiVersion: v1
kind: Service
metadata:
  name: tracer-lb-prod
  labels:
    app: tracer
    track: prod
spec:
  selector:
    app: tracer
    track: prod
  type: LoadBalancer
  ports:
    - protocol: TCP
      port: 80
      targetPort: http
`

const JobYAML = `
apiVersion: batch/v1
kind: Job
metadata:
  name: pi
spec:
  template:
    spec:
      containers:
      - name: pi
        image: perl
        command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      restartPolicy: Never
  backoffLimit: 4
`

const PodYAML = `apiVersion: v1
kind: Pod
metadata:
 name: sleep
 labels:
   app: sleep
spec:
  restartPolicy: OnFailure
  containers:
  - name: sleep
    image: gcr.io/windmill-public-containers/servantes/sleep
`

const MultipleContainersYAML = `
apiVersion: batch/v1
kind: Job
metadata:
  name: pi
spec:
  template:
    spec:
      containers:
      - name: pi1
        image: gcr.io/blorg-dev/perl
        command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      - name: pi2
        image: gcr.io/blorg-dev/perl
        command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      restartPolicy: Never
  backoffLimit: 4
`

const MultipleContainersDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: client
          image: dockerhub.io/client:0.1.0-dev
          imagePullPolicy: Always
          ports:
          - name: http-client
            containerPort: 9000
            protocol: TCP
        - name: backend
          image: dockerhub.io/backend:0.1.0-dev
          imagePullPolicy: Always
          ports:
          - name: http-backend
            containerPort: 8000
            protocol: TCP
          volumeMounts:
          - name: config
            mountPath: /etc/backend
            readOnly: true
      volumes:
        - name: config
          configMap:
            name: fe-backend
            items:
            - key: config
              path: config.yaml`

const SyncletYAML = `apiVersion: apps/v1beta2
kind: DaemonSet
metadata:
  name: owner-synclet
  namespace: kube-system
  labels:
    app: synclet
    owner: owner
    environment: dev
spec:
  selector:
    matchLabels:
      app: synclet
      owner: owner
      environment: dev
  template:
    metadata:
      labels:
        app: synclet
        owner: owner
        environment: dev
    spec:
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      containers:
      - name: synclet
        image: gcr.io/windmill-public-containers/synclet
        imagePullPolicy: Always
        volumeMounts:
        - name: dockersocker
          mountPath: /var/run/docker.sock
        securityContext:
          privileged: true
      - image: jaegertracing/jaeger-agent
        name: jaeger-agent
        ports:
        - containerPort: 5775
          protocol: UDP
        - containerPort: 6831
          protocol: UDP
        - containerPort: 6832
          protocol: UDP
        - containerPort: 5778
          protocol: TCP
        args: ["--collector.host-port=jaeger-collector.default:14267"]
      volumes:
        - name: dockersocker
          hostPath:
            path: /var/run/docker.sock
`

// We deliberately create a pod without any labels, to
// ensure code works without them.
const LonelyPodYAML = `
apiVersion: v1
kind: Pod
metadata:
  name: lonely-pod
spec:
  containers:
  - name: lonely-pod
    image: gcr.io/windmill-public-containers/lonely-pod
    imagePullPolicy: Always
    command: ["/go/bin/lonely-pod"]
    ports:
    - containerPort: 8001
`

// Useful if you ever want to play around with
// deploying postgres
const PostgresYAML = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  labels:
    app: postgres
data:
  POSTGRES_DB: postgresdb
  POSTGRES_USER: postgresadmin
  POSTGRES_PASSWORD: admin123
---
kind: PersistentVolume
apiVersion: v1
metadata:
  name: postgres-pv-volume
  labels:
    type: local
    app: postgres
spec:
  storageClassName: manual
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  hostPath:
    path: "/mnt/data"
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: postgres-pv-claim
  labels:
    app: postgres
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: 3
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    selector:
    spec:
      updateStrategy:
        type: RollingUpdate
      containers:
        - name: postgres
          image: postgres:10.4
          imagePullPolicy: "IfNotPresent"
          ports:
            - containerPort: 5432
          envFrom:
            - configMapRef:
                name: postgres-config
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: postgredb
      volumes:
        - name: postgredb
          persistentVolumeClaim:
            claimName: postgres-pv-claim
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  type: NodePort
  ports:
   - port: 5432
  selector:
   app: postgres
`

// Requires significant sorting to get to an order that's "safe" for applying (see kustomize/ordering.go)
const OutOfOrderYaml = `
apiVersion: batch/v1
kind: Job
metadata:
  name: blorg-job
spec:
  template:
    spec:
      containers:
      - name: blorg-job
        image: gcr.io/blorg-dev/blorg-backend:devel-nick
        command: ["/app/server",  "-job=clean"]
      restartPolicy: Never
  backoffLimit: 4
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: postgres-pv-claim
  labels:
    app: postgres
spec:
  storageClassName: manual
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  type: NodePort
  ports:
   - port: 5432
  selector:
   app: postgres
---
apiVersion: v1
kind: Pod
metadata:
 name: sleep
 labels:
   app: sleep
spec:
  restartPolicy: OnFailure
  containers:
  - name: sleep
    image: gcr.io/windmill-public-containers/servantes/sleep
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  labels:
    app: postgres
data:
  POSTGRES_DB: postgresdb
  POSTGRES_USER: postgresadmin
  POSTGRES_PASSWORD: admin123
---
kind: PersistentVolume
apiVersion: v1
metadata:
  name: postgres-pv-volume
  labels:
    type: local
    app: postgres
spec:
  storageClassName: manual
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  hostPath:
    path: "/mnt/data"
---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres
  replicas: 3
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    selector:
    spec:
      updateStrategy:
        type: RollingUpdate
      containers:
        - name: postgres
          image: postgres:10.4
          imagePullPolicy: "IfNotPresent"
          ports:
            - containerPort: 5432
          envFrom:
            - configMapRef:
                name: postgres-config
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: postgredb
      volumes:
        - name: postgredb
          persistentVolumeClaim:
            claimName: postgres-pv-claim

`

const DoggosDeploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: doggos
  labels:
    app: doggos
    breed: corgi
    whosAGoodBoy: imAGoodBoy
  namespace: the-dog-zone
spec:
  selector:
    matchLabels:
      app: doggos
      breed: corgi
      whosAGoodBoy: imAGoodBoy
  template:
    metadata:
      labels:
        app: doggos
        breed: corgi
        whosAGoodBoy: imAGoodBoy
        tier: web
    spec:
      containers:
      - name: doggos
        image: gcr.io/windmill-public-containers/servantes/doggos
        command: ["/go/bin/doggos"]
`

const DoggosServiceYaml = `
apiVersion: v1
kind: Service
metadata:
  name: doggos
  labels:
    app: doggos
    whosAGoodBoy: imAGoodBoy
spec:
  ports:
    - port: 80
      targetPort: 8083
      protocol: TCP
  selector:
    app: doggos
`
const (
	DoggosName      = "doggos"
	DoggosNamespace = "the-dog-zone"
)

const SnackYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: snack
  labels:
    app: snack
spec:
  selector:
    matchLabels:
      app: snack
  template:
    metadata:
      labels:
        app: snack
    spec:
      containers:
      - name: snack
        image: gcr.io/windmill-public-containers/servantes/snack
        command: ["/go/bin/snack"]
`
const (
	SnackName  = "snack"
	SnackImage = "gcr.io/windmill-public-containers/servantes/snack"
)

const SnackYAMLPostConfig = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: snack
  name: snack
spec:
  selector:
    matchLabels:
      app: snack
  strategy: {}
  template:
    metadata:
      labels:
        app: snack
    spec:
      containers:
      - command:
        - /go/bin/snack
        image: gcr.io/windmill-public-containers/servantes/snack
        name: snack
        resources: {}
`

const SecretName = "mysecret"
const SecretYaml = `
apiVersion: v1
kind: Secret
metadata:
  name: mysecret
type: Opaque
data:
  username: YWRtaW4=
  password: MWYyZDFlMmU2N2Rm
`

// Generated with
// helm fetch stable/redis --version 5.1.3 --untar --untardir tmp && helm template tmp/redis --name test
const HelmGeneratedRedisYAML = `
---
# Source: redis/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: test-redis
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
type: Opaque
data:
  redis-password: "VnF0bkFrUks0cg=="
---
# Source: redis/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: redis
    chart: redis-5.1.3
    heritage: Tiller
    release: test
  name: test-redis
data:
  redis.conf: |-
    # User-supplied configuration:
    # maxmemory-policy volatile-lru
  master.conf: |-
    dir /data
    rename-command FLUSHDB ""
    rename-command FLUSHALL ""
  replica.conf: |-
    dir /data
    rename-command FLUSHDB ""
    rename-command FLUSHALL ""

---
# Source: redis/templates/health-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: redis
    chart: redis-5.1.3
    heritage: Tiller
    release: test
  name: test-redis-health
data:
  ping_local.sh: |-
    response=$(
      redis-cli \
        -a $REDIS_PASSWORD \
        -h localhost \
        -p $REDIS_PORT \
        ping
    )
    if [ "$response" != "PONG" ]; then
      echo "$response"
      exit 1
    fi
  ping_master.sh: |-
    response=$(
      redis-cli \
        -a $REDIS_MASTER_PASSWORD \
        -h $REDIS_MASTER_HOST \
        -p $REDIS_MASTER_PORT_NUMBER \
        ping
    )
    if [ "$response" != "PONG" ]; then
      echo "$response"
      exit 1
    fi
  ping_local_and_master.sh: |-
    script_dir="$(dirname "$0")"
    exit_status=0
    "$script_dir/ping_local.sh" || exit_status=$?
    "$script_dir/ping_master.sh" || exit_status=$?
    exit $exit_status

---
# Source: redis/templates/redis-master-svc.yaml
apiVersion: v1
kind: Service
metadata:
  name: test-redis-master
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
spec:
  type: ClusterIP
  ports:
  - name: redis
    port: 6379
    targetPort: redis
  selector:
    app: redis
    release: "test"
    role: master

---
# Source: redis/templates/redis-slave-svc.yaml

apiVersion: v1
kind: Service
metadata:
  name: test-redis-slave
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
spec:
  type: ClusterIP
  ports:
  - name: redis
    port: 6379
    targetPort: redis
  selector:
    app: redis
    release: "test"
    role: slave

---
# Source: redis/templates/redis-slave-deployment.yaml

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: test-redis-slave
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
spec:
  replicas: 1
  selector:
    matchLabels:
        release: "test"
        role: slave
        app: redis
  template:
    metadata:
      labels:
        release: "test"
        chart: redis-5.1.3
        role: slave
        app: redis
      annotations:
        checksum/health: 0fb018ad71cf7f2bf0bc3482d40b88ccbe3df15cb2a0d51a1f75d02398661bfe
        checksum/configmap: 3ba8fa67229e9f3c03390d9fb9d470d323c0f0f3e07d581e8f46f261945d241b
        checksum/secret: a1edae0cd29184bb1b5065b2388ec3d8c9ccd21eaac533ffceae4fe5ff7ac159
    spec:
      securityContext:
        fsGroup: 1001
        runAsUser: 1001
      serviceAccountName: "default"
      containers:
      - name: test-redis
        image: docker.io/bitnami/redis:4.0.12
        imagePullPolicy: "Always"
        command:
          - /run.sh

        args:
        - "--port"
        - "$(REDIS_PORT)"
        - "--slaveof"
        - "$(REDIS_MASTER_HOST)"
        - "$(REDIS_MASTER_PORT_NUMBER)"
        - "--requirepass"
        - "$(REDIS_PASSWORD)"
        - "--masterauth"
        - "$(REDIS_MASTER_PASSWORD)"
        - "--include"
        - "/opt/bitnami/redis/etc/redis.conf"
        - "--include"
        - "/opt/bitnami/redis/etc/replica.conf"
        env:
        - name: REDIS_REPLICATION_MODE
          value: slave
        - name: REDIS_MASTER_HOST
          value: test-redis-master
        - name: REDIS_PORT
          value: "6379"
        - name: REDIS_MASTER_PORT_NUMBER
          value: "6379"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: test-redis
              key: redis-password
        - name: REDIS_MASTER_PASSWORD
          valueFrom:
            secretKeyRef:
              name: test-redis
              key: redis-password
        ports:
        - name: redis
          containerPort: 6379
        livenessProbe:
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
          exec:
            command:
            - sh
            - -c
            - /health/ping_local_and_master.sh
        readinessProbe:
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 5
          exec:
            command:
            - sh
            - -c
            - /health/ping_local_and_master.sh
        resources:
          null

        volumeMounts:
        - name: health
          mountPath: /health
        - name: redis-data
          mountPath: /data
        - name: config
          mountPath: /opt/bitnami/redis/etc
      volumes:
      - name: health
        configMap:
          name: test-redis-health
          defaultMode: 0755
      - name: config
        configMap:
          name: test-redis
      - name: redis-data
        emptyDir: {}

---
# Source: redis/templates/redis-master-statefulset.yaml
apiVersion: apps/v1beta2
kind: StatefulSet
metadata:
  name: test-redis-master
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
spec:
  selector:
    matchLabels:
      release: "test"
      role: master
      app: redis
  serviceName: test-redis-master
  template:
    metadata:
      labels:
        release: "test"
        chart: redis-5.1.3
        role: master
        app: redis
      annotations:
        checksum/health: 0fb018ad71cf7f2bf0bc3482d40b88ccbe3df15cb2a0d51a1f75d02398661bfe
        checksum/configmap: 3ba8fa67229e9f3c03390d9fb9d470d323c0f0f3e07d581e8f46f261945d241b
        checksum/secret: 4ce19ad3da007ff5f0c283389f765d43b33ed5fa4fcfb8e212308bedc33d62b2
    spec:
      securityContext:
        fsGroup: 1001
        runAsUser: 1001
      serviceAccountName: "default"
      containers:
      - name: test-redis
        image: "docker.io/bitnami/redis:4.0.12"
        imagePullPolicy: "Always"
        command:
          - /run.sh

        args:
        - "--port"
        - "$(REDIS_PORT)"
        - "--requirepass"
        - "$(REDIS_PASSWORD)"
        - "--include"
        - "/opt/bitnami/redis/etc/redis.conf"
        - "--include"
        - "/opt/bitnami/redis/etc/master.conf"
        env:
        - name: REDIS_REPLICATION_MODE
          value: master
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: test-redis
              key: redis-password
        - name: REDIS_PORT
          value: "6379"
        ports:
        - name: redis
          containerPort: 6379
        livenessProbe:
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
          exec:
            command:
            - sh
            - -c
            - /health/ping_local.sh
        readinessProbe:
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 5
          exec:
            command:
            - sh
            - -c
            - /health/ping_local.sh
        resources:
          null

        volumeMounts:
        - name: health
          mountPath: /health
        - name: redis-data
          mountPath: /data
          subPath:
        - name: config
          mountPath: /opt/bitnami/redis/etc
      initContainers:
      - name: volume-permissions
        image: "docker.io/bitnami/minideb:latest"
        imagePullPolicy: "IfNotPresent"
        command: ["/bin/chown", "-R", "1001:1001", "/data"]
        securityContext:
          runAsUser: 0
        volumeMounts:
        - name: redis-data
          mountPath: /data
          subPath:
      volumes:
      - name: health
        configMap:
          name: test-redis-health
          defaultMode: 0755
      - name: config
        configMap:
          name: test-redis
  volumeClaimTemplates:
    - metadata:
        name: redis-data
        labels:
          app: "redis"
          component: "master"
          release: "test"
          heritage: "Tiller"
      spec:
        accessModes:
          - "ReadWriteOnce"
        resources:
          requests:
            storage: "8Gi"
  updateStrategy:
    type: RollingUpdate

---
# Source: redis/templates/metrics-deployment.yaml


---
# Source: redis/templates/metrics-prometheus.yaml

---
# Source: redis/templates/metrics-svc.yaml


---
# Source: redis/templates/networkpolicy.yaml


---
# Source: redis/templates/redis-role.yaml

---
# Source: redis/templates/redis-rolebinding.yaml

---
# Source: redis/templates/redis-serviceaccount.yaml
`

// Example CRD YAML from:
// https://github.com/martin-helmich/kubernetes-crd-example/tree/master/kubernetes
const CRDYAML = `
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: projects.example.martin-helmich.de
spec:
  group: example.martin-helmich.de
  names:
    kind: Project
    plural: projects
    singular: project
  scope: Namespaced
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            image: docker.io/bitnami/minideb:latest
            replicas:
              minimum: 1
              type: integer
          required:
          - replicas
      required:
      - spec
  version: v1alpha1

---
apiVersion: example.martin-helmich.de/v1alpha1
kind: Project
metadata:
  name: example-project
  namespace: default
spec:
  replicas: 1
`

const CRDImage = "docker.io/bitnami/minideb:latest"

const MyNamespaceYAML = `apiVersion: v1
kind: Namespace
metadata:
  name: mynamespace
`
const MyNamespaceName = "mynamespace"

const RedisStatefulSetYAML = `
# Modified from: redis/templates/redis-master-statefulset.yaml
apiVersion: apps/v1beta2
kind: StatefulSet
metadata:
  name: test-redis-master
  labels:
    app: redis
    chart: redis-5.1.3
    release: "test"
    heritage: "Tiller"
spec:
  selector:
    matchLabels:
      release: "test"
      role: master
      app: redis
  serviceName: test-redis-master
  template:
    metadata:
      labels:
        release: "test"
        chart: redis-5.1.3
        role: master
        app: redis
    spec:
      securityContext:
        fsGroup: 1001
        runAsUser: 1001
      serviceAccountName: "default"
      containers:
      - name: test-redis
        image: "docker.io/bitnami/redis:4.0.12"
        imagePullPolicy: "Always"
        command:
          - /run.sh

        args:
        - "--port"
        - "$(REDIS_PORT)"
        - "--requirepass"
        - "$(REDIS_PASSWORD)"
        - "--include"
        - "/opt/bitnami/redis/etc/redis.conf"
        - "--include"
        - "/opt/bitnami/redis/etc/master.conf"
        env:
        - name: REDIS_REPLICATION_MODE
          value: master
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: test-redis
              key: redis-password
        - name: REDIS_PORT
          value: "6379"
        ports:
        - name: redis
          containerPort: 6379
        volumeMounts:
        - name: health
          mountPath: /health
        - name: redis-data
          mountPath: /data
          subPath:
        - name: config
          mountPath: /opt/bitnami/redis/etc
      initContainers:
      - name: volume-permissions
        image: "docker.io/bitnami/minideb:latest"
        imagePullPolicy: "IfNotPresent"
        command: ["/bin/chown", "-R", "1001:1001", "/data"]
        securityContext:
          runAsUser: 0
        volumeMounts:
        - name: redis-data
          mountPath: /data
          subPath:
      volumes:
      - name: health
        configMap:
          name: test-redis-health
          defaultMode: 0755
      - name: config
        configMap:
          name: test-redis
  volumeClaimTemplates:
    - metadata:
        name: redis-data
        labels:
          app: "redis"
          component: "master"
          release: "test"
          heritage: "Tiller"
      spec:
        accessModes:
          - "ReadWriteOnce"
        resources:
          requests:
            storage: "8Gi"
  updateStrategy:
    type: RollingUpdate
`

func Deployment(name string, imageName string) string {
	result := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: NAME
  labels:
    app: NAME
spec:
  replicas: 1
  selector:
    matchLabels:
      app: NAME
  template:
    metadata:
      labels:
        app: NAME
    spec:
      containers:
      - name: NAME
        image: IMAGE
`
	result = strings.Replace(result, "NAME", name, -1)
	result = strings.Replace(result, "IMAGE", imageName, -1)
	return result
}

const PodDisruptionBudgetYAML = `
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  labels:
    app: zookeeper
  name: infra-kafka-zookeeper
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: zookeeper
      component: server
      release: infra-kafka
`

const DoggosListYAML = `
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Service
  metadata:
    name: doggos
    labels:
      app: doggos
      whosAGoodBoy: imAGoodBoy
  spec:
    ports:
      - port: 80
        targetPort: 8083
        protocol: TCP
    selector:
      app: doggos
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: doggos
    labels:
      app: doggos
      breed: corgi
      whosAGoodBoy: imAGoodBoy
    namespace: the-dog-zone
  spec:
    selector:
      matchLabels:
        app: doggos
        breed: corgi
        whosAGoodBoy: imAGoodBoy
    template:
      metadata:
        labels:
          app: doggos
          breed: corgi
          whosAGoodBoy: imAGoodBoy
          tier: web
      spec:
        containers:
        - name: doggos
          image: gcr.io/windmill-public-containers/servantes/doggos
          command: ["/go/bin/doggos"]
`

const KnativeRelease = `

---
# net-istio.yaml
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  # These are the permissions needed by the Istio Ingress implementation.
  name: knative-serving-istio
  labels:
    serving.knative.dev/release: "v0.15.0"
    serving.knative.dev/controller: "true"
    networking.knative.dev/ingress-provider: istio
rules:
- apiGroups: ["networking.istio.io"]
  resources: ["virtualservices", "gateways"]
  verbs: ["get", "list", "create", "update", "delete", "patch", "watch"]

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This is the shared Gateway for all Knative routes to use.
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: knative-ingress-gateway
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "*"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# A cluster local gateway to allow pods outside of the mesh to access
# Services and Routes not exposing through an ingress.  If the users
# do have a service mesh setup, this isn't required.
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: cluster-local-gateway
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
spec:
  selector:
    istio: cluster-local-gateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "*"

---
# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: webhook.istio.networking.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
webhooks:
- admissionReviewVersions:
  - v1beta1
  clientConfig:
    service:
      name: istio-webhook
      namespace: knative-serving
  failurePolicy: Fail
  sideEffects: None
  objectSelector:
    matchExpressions:
    - {key: "serving.knative.dev/configuration", operator: Exists}
  name: webhook.istio.networking.internal.knative.dev

---
# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingWebhookConfiguration
metadata:
  name: config.webhook.istio.networking.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
webhooks:
- admissionReviewVersions:
  - v1beta1
  clientConfig:
    service:
      name: istio-webhook
      namespace: knative-serving
  failurePolicy: Fail
  sideEffects: None
  name: config.webhook.istio.networking.internal.knative.dev
  namespaceSelector:
    matchExpressions:
    - key: serving.knative.dev/release
      operator: Exists

---
# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: v1
kind: Secret
metadata:
  name: istio-webhook-certs
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio

---
# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: v1
kind: ConfigMap
metadata:
  name: config-istio
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
data:
  _example: |
    gateway.knative-serving.knative-ingress-gateway: "istio-ingressgateway.istio-system.svc.cluster.local"
    local-gateway.knative-serving.cluster-local-gateway: "cluster-local-gateway.istio-system.svc.cluster.local"

    # To use only Istio service mesh and no cluster-local-gateway, replace
    # all local-gateway.* entries by the following entry.
    local-gateway.mesh: "mesh"
  # TODO(nghia): Extract the .svc.cluster.local suffix into its own config.

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apps/v1
kind: Deployment
metadata:
  name: networking-istio
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
spec:
  selector:
    matchLabels:
      app: networking-istio
  template:
    metadata:
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
        # This must be outside of the mesh to probe the gateways.
        # NOTE: this is allowed here and not elsewhere because
        # this is the Istio controller, and so it may be Istio-aware.
        sidecar.istio.io/inject: "false"
      labels:
        app: networking-istio
        serving.knative.dev/release: "v0.15.0"
    spec:
      serviceAccountName: controller
      containers:
      - name: networking-istio
        # This is the Go import path for the binary that is containerized
        # and substituted here.
        image: gcr.io/knative-releases/knative.dev/net-istio/cmd/controller@sha256:3f8db840f5b3778a842dbbf7e8bc9f2babd10b144b816c8497ac46430db8254e
        resources:
          requests:
            cpu: 30m
            memory: 40Mi
          limits:
            cpu: 300m
            memory: 400Mi
        env:
        - name: SYSTEM_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: CONFIG_LOGGING_NAME
          value: config-logging
        - name: CONFIG_OBSERVABILITY_NAME
          value: config-observability
        - # TODO(https://github.com/knative/pkg/pull/953): Remove stackdriver specific config
          name: METRICS_DOMAIN
          value: knative.dev/net-istio
        securityContext:
          allowPrivilegeEscalation: false
        ports:
        - name: metrics
          containerPort: 9090
        - name: profiling
          containerPort: 8008

# Unlike other controllers, this doesn't need a Service defined for metrics and
# profiling because it opts out of the mesh (see annotation above).

---
# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apps/v1
kind: Deployment
metadata:
  name: istio-webhook
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
spec:
  selector:
    matchLabels:
      app: istio-webhook
      role: istio-webhook
  template:
    metadata:
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
      labels:
        app: istio-webhook
        role: istio-webhook
        serving.knative.dev/release: "v0.15.0"
    spec:
      serviceAccountName: controller
      containers:
      - name: webhook
        # This is the Go import path for the binary that is containerized
        # and substituted here.
        image: gcr.io/knative-releases/knative.dev/net-istio/cmd/webhook@sha256:b691c81d117d666d479b4f57416f3edd53f282a7346c64af4ce9c4585df5bec7
        resources:
          requests:
            cpu: 20m
            memory: 20Mi
          limits:
            cpu: 200m
            memory: 200Mi
        env:
        - name: SYSTEM_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: CONFIG_LOGGING_NAME
          value: config-logging
        - name: CONFIG_OBSERVABILITY_NAME
          value: config-observability
        - # TODO(https://github.com/knative/pkg/pull/953): Remove stackdriver specific config
          name: METRICS_DOMAIN
          value: knative.dev/net-istio
        - name: WEBHOOK_NAME
          value: istio-webhook
        securityContext:
          allowPrivilegeEscalation: false
        ports:
        - name: metrics
          containerPort: 9090
        - name: profiling
          containerPort: 8008
        - name: https-webhook
          containerPort: 8443

---
# Copyright 2020 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: v1
kind: Service
metadata:
  name: istio-webhook
  namespace: knative-serving
  labels:
    role: istio-webhook
    serving.knative.dev/release: "v0.15.0"
    networking.knative.dev/ingress-provider: istio
spec:
  ports:
  - # Define metrics and profiling for them to be accessible within service meshes.
    name: http-metrics
    port: 9090
    targetPort: 9090
  - name: http-profiling
    port: 8008
    targetPort: 8008
  - name: https-webhook
    port: 443
    targetPort: 8443
  selector:
    app: istio-webhook

---

`

const KnativeServingCore = `
---
# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: caching.internal.knative.dev/v1alpha1
kind: Image
metadata:
  name: queue-proxy
  namespace: knative-serving
  labels:
    serving.knative.dev/release: "v0.15.0"
spec:
  # This is the Go import path for the binary that is containerized
  # and substituted here.
  image: gcr.io/knative-releases/knative.dev/serving/cmd/queue@sha256:713bd548700bf7fe5452969611d1cc987051bd607d67a4e7623e140f06c209b2

`
const KnativeServingCRDs = `
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: certificates.networking.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: networking.internal.knative.dev
  version: v1alpha1
  names:
    kind: Certificate
    plural: certificates
    singular: certificate
    categories:
    - knative-internal
    - networking
    shortNames:
    - kcert
  scope: Namespaced
  subresources:
    status: {}
  additionalPrinterColumns:
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type==\"Ready\")].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type==\"Ready\")].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: configurations.serving.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
    duck.knative.dev/podspecable: "true"
spec:
  group: serving.knative.dev
  preserveUnknownFields: false
  validation:
    openAPIV3Schema:
      type: object
      # this is a work around so we don't need to flush out the
      # schema for each version at this time
      #
      # see issue: https://github.com/knative/serving/issues/912
      x-kubernetes-preserve-unknown-fields: true
  versions:
  - name: v1alpha1
    served: true
    storage: false
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
  names:
    kind: Configuration
    plural: configurations
    singular: configuration
    categories:
    - all
    - knative
    - serving
    shortNames:
    - config
    - cfg
  scope: Namespaced
  subresources:
    status: {}
  conversion:
    strategy: Webhook
    webhookClientConfig:
      service:
        name: webhook
        namespace: knative-serving
  additionalPrinterColumns:
  - name: LatestCreated
    type: string
    JSONPath: .status.latestCreatedRevisionName
  - name: LatestReady
    type: string
    JSONPath: .status.latestReadyRevisionName
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: ingresses.networking.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: networking.internal.knative.dev
  versions:
  - name: v1alpha1
    served: true
    storage: true
  names:
    kind: Ingress
    plural: ingresses
    singular: ingress
    categories:
    - knative-internal
    - networking
    shortNames:
    - kingress
    - king
  scope: Namespaced
  subresources:
    status: {}
  additionalPrinterColumns:
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: metrics.autoscaling.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: autoscaling.internal.knative.dev
  version: v1alpha1
  names:
    kind: Metric
    plural: metrics
    singular: metric
    categories:
    - knative-internal
    - autoscaling
  scope: Namespaced
  subresources:
    status: {}
  additionalPrinterColumns:
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: podautoscalers.autoscaling.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: autoscaling.internal.knative.dev
  versions:
  - name: v1alpha1
    served: true
    storage: true
  names:
    kind: PodAutoscaler
    plural: podautoscalers
    singular: podautoscaler
    categories:
    - knative-internal
    - autoscaling
    shortNames:
    - kpa
    - pa
  scope: Namespaced
  subresources:
    status: {}
  additionalPrinterColumns:
  - name: DesiredScale
    type: integer
    JSONPath: ".status.desiredScale"
  - name: ActualScale
    type: integer
    JSONPath: ".status.actualScale"
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: revisions.serving.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: serving.knative.dev
  preserveUnknownFields: false
  validation:
    openAPIV3Schema:
      type: object
      # this is a work around so we don't need to flush out the
      # schema for each version at this time
      #
      # see issue: https://github.com/knative/serving/issues/912
      x-kubernetes-preserve-unknown-fields: true
  versions:
  - name: v1alpha1
    served: true
    storage: false
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
  names:
    kind: Revision
    plural: revisions
    singular: revision
    categories:
    - all
    - knative
    - serving
    shortNames:
    - rev
  scope: Namespaced
  subresources:
    status: {}
  conversion:
    strategy: Webhook
    webhookClientConfig:
      service:
        name: webhook
        namespace: knative-serving
  additionalPrinterColumns:
  - name: Config Name
    type: string
    JSONPath: ".metadata.labels['serving\\.knative\\.dev/configuration']"
  - name: K8s Service Name
    type: string
    JSONPath: ".status.serviceName"
  - name: Generation
    type: string # int in string form :(
    JSONPath: ".metadata.labels['serving\\.knative\\.dev/configurationGeneration']"
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: routes.serving.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
    duck.knative.dev/addressable: "true"
spec:
  group: serving.knative.dev
  preserveUnknownFields: false
  validation:
    openAPIV3Schema:
      type: object
      # this is a work around so we don't need to flush out the
      # schema for each version at this time
      #
      # see issue: https://github.com/knative/serving/issues/912
      x-kubernetes-preserve-unknown-fields: true
  versions:
  - name: v1alpha1
    served: true
    storage: false
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
  names:
    kind: Route
    plural: routes
    singular: route
    categories:
    - all
    - knative
    - serving
    shortNames:
    - rt
  scope: Namespaced
  subresources:
    status: {}
  conversion:
    strategy: Webhook
    webhookClientConfig:
      service:
        name: webhook
        namespace: knative-serving
  additionalPrinterColumns:
  - name: URL
    type: string
    JSONPath: .status.url
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: serverlessservices.networking.internal.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
spec:
  group: networking.internal.knative.dev
  versions:
  - name: v1alpha1
    served: true
    storage: true
  names:
    kind: ServerlessService
    plural: serverlessservices
    singular: serverlessservice
    categories:
    - knative-internal
    - networking
    shortNames:
    - sks
  scope: Namespaced
  subresources:
    status: {}
  additionalPrinterColumns:
  - name: Mode
    type: string
    JSONPath: ".spec.mode"
  - name: Activators
    type: integer
    JSONPath: ".spec.numActivators"
  - name: ServiceName
    type: string
    JSONPath: ".status.serviceName"
  - name: PrivateServiceName
    type: string
    JSONPath: ".status.privateServiceName"
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: services.serving.knative.dev
  labels:
    serving.knative.dev/release: "v0.15.0"
    knative.dev/crd-install: "true"
    duck.knative.dev/addressable: "true"
    duck.knative.dev/podspecable: "true"
spec:
  group: serving.knative.dev
  preserveUnknownFields: false
  validation:
    openAPIV3Schema:
      type: object
      # this is a work around so we don't need to flush out the
      # schema for each version at this time
      #
      # see issue: https://github.com/knative/serving/issues/912
      x-kubernetes-preserve-unknown-fields: true
  versions:
  - name: v1alpha1
    served: true
    storage: false
  - name: v1beta1
    served: true
    storage: false
  - name: v1
    served: true
    storage: true
  names:
    kind: Service
    plural: services
    singular: service
    categories:
    - all
    - knative
    - serving
    shortNames:
    - kservice
    - ksvc
  scope: Namespaced
  subresources:
    status: {}
  conversion:
    strategy: Webhook
    webhookClientConfig:
      service:
        name: webhook
        namespace: knative-serving
  additionalPrinterColumns:
  - name: URL
    type: string
    JSONPath: .status.url
  - name: LatestCreated
    type: string
    JSONPath: .status.latestCreatedRevisionName
  - name: LatestReady
    type: string
    JSONPath: .status.latestReadyRevisionName
  - name: Ready
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].status"
  - name: Reason
    type: string
    JSONPath: ".status.conditions[?(@.type=='Ready')].reason"

---
# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: images.caching.internal.knative.dev
  labels:
    knative.dev/crd-install: "true"
spec:
  group: caching.internal.knative.dev
  version: v1alpha1
  names:
    kind: Image
    plural: images
    singular: image
    categories:
    - knative-internal
    - caching
    shortNames:
    - img
  scope: Namespaced
  subresources:
    status: {}

---
`
