package testyaml

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

const DoggosDeploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: doggos
  labels:
    app: doggos
    breed: corgi
    whosAGoodBoy: imAGoodBoy
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
    whosAGoodBoy: imAGoodBoy
`

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
        tier: web
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
  creationTimestamp: null
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
      creationTimestamp: null
      labels:
        app: snack
        tier: web
    spec:
      containers:
      - command:
        - /go/bin/snack
        image: gcr.io/windmill-public-containers/servantes/snack
        name: snack
        resources: {}
status: {}
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
