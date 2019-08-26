package testyaml

const OneDeleted = `job.batch "echo-hi" deleted
apiVersion: batch/v1
kind: Job
metadata:
  creationTimestamp: "2019-08-18T13:07:52Z"
  labels:
    tilt-deployid: "1566133672508279794"
    tilt-manifest: echo-hi
    tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
  name: echo-hi
  namespace: default
  resourceVersion: "6356116"
  selfLink: /apis/batch/v1/namespaces/default/jobs/echo-hi
  uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
spec:
  backoffLimit: 4
  completions: 1
  parallelism: 1
  selector:
    matchLabels:
      controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
  template:
    metadata:
      creationTimestamp: ~
      labels:
        controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
        job-name: echo-hi
        tilt-deployid: "1566133672508279794"
        tilt-manifest: echo-hi
        tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
    spec:
      containers:
        -
          command:
            - echo
            - hi
          image: alpine
          imagePullPolicy: IfNotPresent
          name: echohi
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Never
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status: {}
`

const TwoDeleted = `job.batch "echo-hi" deleted
job.batch "echo-hi2" deleted
apiVersion: batch/v1
kind: Job
metadata:
  creationTimestamp: "2019-08-18T13:07:52Z"
  labels:
    tilt-deployid: "1566133672508279794"
    tilt-manifest: echo-hi
    tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
  name: echo-hi
  namespace: default
  resourceVersion: "6356116"
  selfLink: /apis/batch/v1/namespaces/default/jobs/echo-hi
  uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
spec:
  backoffLimit: 4
  completions: 1
  parallelism: 1
  selector:
    matchLabels:
      controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
  template:
    metadata:
      creationTimestamp: ~
      labels:
        controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
        job-name: echo-hi
        tilt-deployid: "1566133672508279794"
        tilt-manifest: echo-hi
        tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
    spec:
      containers:
        -
          command:
            - echo
            - hi
          image: alpine
          imagePullPolicy: IfNotPresent
          name: echohi
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Never
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status: {}
---
apiVersion: batch/v1
kind: Job
metadata:
  creationTimestamp: "2019-08-18T13:07:52Z"
  labels:
    tilt-deployid: "1566133672508279794"
    tilt-manifest: echo-hi2
    tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
  name: echo-hi2
  namespace: default
  resourceVersion: "6356116"
  selfLink: /apis/batch/v1/namespaces/default/jobs/echo-hi2
  uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
spec:
  backoffLimit: 4
  completions: 1
  parallelism: 1
  selector:
    matchLabels:
      controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
  template:
    metadata:
      creationTimestamp: ~
      labels:
        controller-uid: 81d15054-a30f-4b89-8f0b-e0afe642367e
        job-name: echo-hi
        tilt-deployid: "1566133672508279794"
        tilt-manifest: echo-hi
        tilt-runid: e1afedb3-521d-4e1e-b85e-df0787454d23
    spec:
      containers:
        -
          command:
            - echo
            - hi
          image: alpine
          imagePullPolicy: IfNotPresent
          name: echohi
          resources: {}
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Never
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
status: {}
`
