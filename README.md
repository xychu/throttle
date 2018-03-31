# Throttle

Extended resource quota controller and Admission Webhook for using GPU in kubernetes 1.9+.

![throttle arch](/artifacts/img/throttle.png)

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

Let's create a `test-ns` namespace and define a GPUQuota, then attempt to create a Pod with GPU resource beyond that limit.

Create the `test-ns` Namespace:

```
kubectl create ns test-ns
```

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

