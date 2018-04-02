# WARN

Will be obsolet after your kubernetes has: https://github.com/kubernetes/kubernetes/pull/57302

Then the usage will be as simple as: https://github.com/kubernetes/kubernetes/pull/57302#issuecomment-367933408

quota example:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: quota1
spec:
  hard:
    cpu: 300m
    memory: 3900Mi
    requests.nvidia.com/gpu: 4
```

pod example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  labels:
    name: test-pod-applied
spec:
  containers:
  - name: kubernetes-pause
    image: gcr.io/google-containers/pause:2.0
    resources:
      requests:
        cpu: 300m
        memory: 1300Mi
        nvidia.com/gpu: "4"
      limits:
        cpu: 300m
        memory: 1300Mi
        nvidia.com/gpu: "4"
```

Kubernetes rocks!


# Throttle

Extended resource quota controller and Admission Webhook for using GPU in kubernetes 1.9+.

![throttle arch](/artifacts/img/throttle.png)

### GPUQuota CRD

`GPUQuota` is a namespaced `scope` kubernetes CRD [(custom resources definition)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/).
So you can define its own quota for each namespace that will consume the GPU resources available in kubernetes cluster.

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: gpuquotas.throttlecontroller.example.com
spec:
  group: throttlecontroller.example.com
  version: v1alpha1
  names:
    kind: GPUQuota
    plural: gpuquotas
  scope: Namespaced
```

### GPUQuota type

```go
// GPUQuota is a specification for a GPUQuota resource
type GPUQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   corev1.ResourceQuotaSpec   `json:"spec"`
	Status corev1.ResourceQuotaStatus `json:"status"`
}
```

### GPUQuota example

```yaml
apiVersion: throttlecontroller.example.com/v1alpha1
kind: GPUQuota
metadata:
  name: example-gpuquota
  namespace: example-namespace
spec:
  hard:
    limits.gpu: 4
    requests.gpu: 2
```

### GPUQuota Webhook

`GPUQuota` Admission Webhook is a web server that admit all `Pod` creating and updating based on their namespace `GPUQuota`, accept`v1beta1.AdmissionReview` and return `v1beta1.AdmissionResponse`.

All the workload contains GPU resource(`nvidia.com/gpu`) requests will be processed.

```yaml
    resources:
        limits:
            nvidia.com/gpu: 5
```

## Usage

### Prerequisites

#### Get code:

```
git clone https://github.com/xychu/throttle.git
```

```
cd throttle
```

#### Setup & config kubernetes cluster

A Kubernetes 1.9+ cluster is required with support for the [ValidatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#validatingadmissionwebhook-alpha-in-18-beta-in-19) alpha feature enabled.

> Any Kubernetes cluster with support for validating admission webhooks should work. 
> For development, you can use `minikube` or `./hack/local-up-cluster.sh`.

### Deploy the GPUQuota Controller

Create the GPUQuota CRD:

```
kubectl apply -f artifacts/crd.yaml
```

Create the GPUQuota related RoleBinding:

```
kubectl apply -f artifacts/rolebinding.yaml
```

Create the GPUQuota Controller deployment and service:

```
kubectl apply -f artifacts/controller.yaml
```

Create the `test-ns` Namespace:

```
kubectl create ns test-ns
```

Create the GPUQuota for `test-ns`:

```
kubectl apply -f artifacts/example-gpuquota.yaml -n test-ns
```


### Deploy the GPUQuota Admission Webhook

Create the `gpuquota-webhook-secret` secret and store the TLS certs:

```
kubectl create secret tls gpuquota-webhook-secret -n kube-system \
  --key artifacts/pki/webhook-key.pem \
  --cert artifacts/pki/webhook.pem
```

Create the webhook deployment and service:

```
kubectl apply -f artifacts/webhook.yaml
```

Create the `gpuquota-webook` ValidatingWebhookConfiguration:

```
kubectl apply -f artifacts/webhookconfig.yaml
```

> After you create the validating webhook configuration, the system will take a few seconds to honor the new configuration.

### Testing the Admission Webhook

Let's declear a `GPUQuota` for `test-ns` namespace that have been created above, then attempt to create a Pod with GPU resource beyond that limit.

> Create the `test-ns` Namespace if you did not do that before:
> 
> ```
> kubectl create ns test-ns
> ```

Create the GPUQuota for `test-ns`:

```
kubectl apply -f artifacts/example-gpuquota.yaml -n test-ns
```

> After that you can view the quota detail by `kubectl get gpuquota -n test-ns -o yaml`.


Create the test Pod request with too much quota under `test-ns` namespace:

```
kubectl apply -f artifacts/test-gpu.yaml
```

Notice the test pod was not created and the follow error was returned: 

```
Error from server: error when creating "artifacts/test-gpu.yaml": admission webhook "gpuquota.example.com" denied the request: exceeded quota: requests.gpu, requested(5) + used(0) > limited(2)exceeded quota: limits.gpu, requested(5) + used(0) > limited(4)
```

Have Fun!


### Refs

- https://github.com/kubernetes/sample-controller
- https://github.com/caesarxuchao/example-webhook-admission-controller
- https://github.com/kelseyhightower/grafeas-tutorial/tree/master/image-signature-webhook

