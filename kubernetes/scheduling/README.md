# Scheduling

A few years ago I was adding GPU enabled services to our Kubernetes clusters
and needed to modify our auto-scaling configuration to support the mixed compute
requirements.

I've included some of the key configuration and reasoning below.

Over time this has or will become obsolete due to new Kubernetes features and
supporting applications, however I hope it helps with your planning.

## Table of Contents

1. [Priority Classes](#priority-classes)
2. [Pod Disruption Budgets](#pod-disruption-budgets)
3. [Cluster Autoscaler](#cluster-autoscaler)
4. [Regular Application](#regular-application)
5. [GPU Enabled Application](#gpu-enabled-application)
6. [Node](#node)

## Priority Classes

### Table

| priorityClassName       | Custom | Priority   | Why                                                                          |
| ----------------------- | ------ | ---------- | ---------------------------------------------------------------------------- |
| system-node-critical    | N      | 2000001000 | Node can't function without                                                  |
| system-cluster-critical | N      | 2000000000 | Cluster can't function without                                               |
| monitoring              | Y      | 1000       | Monitoring cluster, without can't alert/make decisions                       |
| infra                   | Y      | 800        | Monitoring portal access where breakglass exists or key cluster wide service |
| high                    | Y      | 600        | Specific requirements, enables preferential scheduling                       |
| medium                  | Y      | 400        | General workload, no specific requirements                                   |
| low                     | Y      | 200        | Can sustain outage, not external customer impacting                          |
| default                 | Y      | 100        | Catch-all for missing class name, lint and fail CI if not specified          |

### Example mappings

```sh
$ grep -r priorityClassName: | grep --only-matching '/[\a-z-]_.yaml._' | sort -u

/efs-csi-driver.yaml: priorityClassName: system-node-critical
/kube-proxy.yaml: priorityClassName: system-node-critical
/nvidia-device-plugin.yaml: priorityClassName: system-node-critical

/calico-typha.yaml: priorityClassName: system-cluster-critical
/calico-node.yaml: priorityClassName: system-node-critical
/cluster-autoscaler.yaml: priorityClassName: system-cluster-critical
/core-dns.yaml: priorityClassName: system-cluster-critical

/fluent-bit.yaml: priorityClassName: monitoring
/prometheus.yaml: priorityClassName: monitoring # grafana/thanos/etc

/cert-manager-controller.yaml: priorityClassName: infra
/external-dns.yaml: priorityClassName: infra
/nginx-ingress-myapp.yaml: priorityClassName: infra
/nginx-ingress-tools.yaml: priorityClassName: infra
/prometheus-exporter-xyz.yaml: priorityClassName: infra
/sealed-secrets-controller.yaml: priorityClassName: infra

/example-gpu-app.yaml: priorityClassName: high
/example-app.yaml: priorityClassName: medium
/example-nice-to-have-app.yaml: priorityClassName: low
```

## Pod Disruption Budgets

Crucial to enable cluster to scale node count up and down:

- generally no time pressure other than cost to scale down
- prevent blocking eternally on app that doesn't shutdown gracefully
- prevent service outage on draining all nodes housing service
- interplay with availability zone targets if multi-zone cluster

```yaml
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: calico-typha
  namespace: kube-system
  labels:
    k8s-app: calico-typha
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: calico-typha
```

## Cluster Autoscaler

- Prioritise non-GPU instance types when GPU not required
- Match corresponding tagged AWS Autoscaling Groups so non-k8s ASG's unaffected

### ConfigMap

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-autoscaler-priority-expander
  namespace: kube-system
  labels:
    app: cluster-autoscaler
data:
  # regex on AWS ASG name convention using instance type in name
  # prioritise cheaper (non-gpu) instance classes
  # catch all for non-matching group
  # note ASG must have CAS tags to be a candidate
  priorities: |-
    30:
      - .*g6xlarge.*
    50:
      - .*m6.*
      - .*c6.*
```

### Deployment

- Note ASG tagging convention, match with Terraform/Pulumi other.

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    app: cluster-autoscaler
  annotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    metadata:
      labels:
        app: cluster-autoscaler
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
    spec:
      serviceAccountName: cluster-autoscaler
      containers:
        - image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.20.0
          name: cluster-autoscaler
          command:
            - ./cluster-autoscaler
            - --v=4
            - --stderrthreshold=info
            - --cloud-provider=aws
            - --skip-nodes-with-local-storage=false
            - --expander=priority
            - --node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,k8s.io/cluster-autoscaler/my-cluster-name ## NOTE ASG TAGGING CONVENTION
            - --balance-similar-node-groups
            - --skip-nodes-with-system-pods=false
```

## Regular Application

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-app
  namespace: my-namespace
  labels:
    app.kubernetes.io/name: example-app
spec:
  revisionHistoryLimit: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
  selector:
    matchLabels:
      app.kubernetes.io/name: example-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: example-app
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app.kubernetes.io/name
                    operator: In
                    values:
                      - example-app
              topologyKey: kubernetes.io/hostname
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app.kubernetes.io/name
                      operator: In
                      values:
                        - example-app
                topologyKey: topology.kubernetes.io/zone
      containers:
        - name: example-app
          image: my-company/example-gpu-app # note tag added by CICD tool
          imagePullPolicy: IfNotPresent
          resources:
            limits:
              cpu: 450m
              memory: 100Mi
            requests:
              cpu: 150m
              memory: 100Mi
          livenessProbe:
            httpGet:
              path: /ping
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          readinessProbe:
            httpGet:
              path: /ping
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081
              name: metrics
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
      imagePullSecrets:
        - name: my-image-pull-secret
      priorityClassName: medium
      securityContext:
        runAsNonRoot: true
        runAsGroup: 18081
        runAsUser: 18081
        fsGroup: 18081
```

### HorizontalPodAutoscaler

```yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  annotations: {}
  labels:
    app.kubernetes.io/name: example-app
  name: example-app
  namespace: my-namespace
spec:
  maxReplicas: 10
  metrics:
    - pods:
        metric:
          name: example_app_utilisation:max2m
        target:
          averageValue: "0.80"
          type: AverageValue
      type: Pods
  minReplicas: 2
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: example-app
```

### PodDisruptionBudget

```yaml
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: example-app
  namespace: my-namespace
  labels:
    app.kubernetes.io/name: example-app
spec:
  maxUnavailable: 33%
  selector:
    matchLabels:
      app.kubernetes.io/name: example-app
```

## GPU Enabled Application

### Deployment

| Configuration                          | Why                                                                                                               |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `nodeSelector: []`                     | ensure targets GPU enabled nodes                                                                                  |
| `priorityClassName: high`              | bump lower classes off GPU nodes whilst still allowing non-gpu apps to share node to use any spare compute        |
| `resources.requests.nvidia.com/gpu: 1` | instances have only 1 GPU, ensure committed allocation, time slicing GPU's not possible (at this time)            |
| `tolerations: []`                      | Sometimes we want to keep GPU enabled nodes for GPU apps only, add toleration for corresponding GPU node `taint`. |

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-gpu-app
  namespace: my-namespace
  labels:
    app.kubernetes.io/name: example-gpu-app
spec:
  replicas: 0
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: example-gpu-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: example-gpu-app
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 10
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app.kubernetes.io/name
                      operator: In
                      values:
                        - example-gpu-app
                topologyKey: kubernetes.io/hostname
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app.kubernetes.io/name
                      operator: In
                      values:
                        - example-gpu-app
                topologyKey: topology.kubernetes.io/zone
      containers:
        - name: example-gpu-app
          image: my-company/example-gpu-app
          imagePullPolicy: IfNotPresent
          lifecycle:
            preStop:
              exec:
                command:
                  - "sleep"
                  - "10" # wait for inflight request to finish
          resources:
            limits:
              nvidia.com/gpu: 1 # time-slicing GPU not possible at this time.
              cpu: 2
              memory: 10.0Gi
            requests:
              nvidia.com/gpu: 1 # time-slicing GPU not possible at this time.
              cpu: 1
              memory: 10.0Gi
          ports:
            - containerPort: 8000
              name: metrics
            - containerPort: 8080
              name: http
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
      nodeSelector:
        node.kubernetes.io/instance-type: g6.xlarge
        nvidia.com/gpu: "true" # required for CAS to scale to 0
      priorityClassName: high
      tolerations:
        - key: example.com/workload
          operator: Equal
          value: gpu # if preventing non-gpu apps from scheduling on gpu-enabled nodes.
          effect: NoSchedule
```

## Node

AWS ASG Name: `eks_01_w_01_g6dn_xlarge_a`

- Match Cluster Autoscaling regex config

Kubelet extra args: `--register-with-taints=example.com/workload=gpu:NoSchedule`

- Prevent non-gpu enabled applications running on node (if required)
- Use corresponding `tolerations: [...]` on `Deployment` or `StatefulSet`, etc to schedule on node (e.g: GPU enabled app).

Kubelet extra args: `--node-labels=nvidia.com/gpu=true,k8s.amazonaws.com/accelerator=nvidia-tesla`

- Pair with `nodeSelector: { }` in `Deployment` to target GPU enabled workload
- For CAS to scale GPU node pool to 0, all apps needed above `nodeSelector` targeting GPU node pool, otherwise CAS wouldn't scale in to 0.
- Advertise number of GPU's. At the time the nodes only had a single GPU.
