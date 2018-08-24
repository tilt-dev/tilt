package k8s

const BlorgBackendYAML = `
# Template should be populated using populate_config_template.py

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

const SanchoYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
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
