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
