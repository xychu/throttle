/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	//	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	// TODO: try this library to see if it generates correct json patch
	// https://github.com/mattbaird/jsonpatch

	throttlev1alpha1 "github.com/xychu/throttle/pkg/apis/throttlecontroller/v1alpha1"
	clientset "github.com/xychu/throttle/pkg/client/clientset/versioned"
)

// Config contains the server (the webhook) cert and key.
type Config struct {
	CertFile   string
	KeyFile    string
	KubeConfig string
	MasterURL  string
}

var throttleClient *clientset.Clientset

func (c *Config) addFlags() {
	flag.StringVar(&c.CertFile, "tls-cert-file", c.CertFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", c.KeyFile, ""+
		"File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&c.KubeConfig, "kubeconfig", "", "Path to a kubeconfig. "+
		"Only required if out-of-cluster.")
	flag.StringVar(&c.MasterURL, "master", "", "The address of the Kubernetes API server."+
		" Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

func calcPodGPUUsage(pod *corev1.Pod) (resource.Quantity, resource.Quantity) {
	request_value := resource.Quantity{}
	limit_value := resource.Quantity{}
	for j := range pod.Spec.Containers {
		glog.V(4).Infof("calc container resource: '%v'", pod.Spec.Containers[j].Resources)
		if request, found := pod.Spec.Containers[j].Resources.Requests[throttlev1alpha1.ResourceGPU]; found {
			request_value.Add(request)
		}
		if limit, found := pod.Spec.Containers[j].Resources.Limits[throttlev1alpha1.ResourceGPU]; found {
			limit_value.Add(limit)
		}
	}
	// InitContainers are run **sequentially** before other containers start, so the highest
	// init container resource is compared against the sum of app containers to determine
	// the effective usage for both requests and limits.
	for j := range pod.Spec.InitContainers {
		if request, found := pod.Spec.InitContainers[j].Resources.Requests[throttlev1alpha1.ResourceGPU]; found {
			if request_value.Cmp(request) < 0 {
				request_value = *(request.Copy())
			}
		}
		if limit, found := pod.Spec.InitContainers[j].Resources.Limits[throttlev1alpha1.ResourceGPU]; found {
			if limit_value.Cmp(limit) < 0 {
				limit_value = *(limit.Copy())
			}
		}
	}
	return request_value, limit_value
}

// only allow pods to pull images from specific registry.
func admitPods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	glog.V(2).Info("admitting pods")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		glog.Error(err)
		return toAdmissionResponse(err)
	}

	isUpdate := false
	if &ar.Request.OldObject != nil {
		isUpdate = true
	}
	raw := ar.Request.Object.Raw
	oldRaw := []byte{}
	pod := corev1.Pod{}
	oldPod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}
	if isUpdate {
		oldRaw = ar.Request.OldObject.Raw
		if _, _, err := deserializer.Decode(oldRaw, nil, &oldPod); err != nil {
			glog.Error(err)
			return toAdmissionResponse(err)
		}
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	requestGPUQuota := resource.Quantity{}
	requestGPUQuotaUsed := resource.Quantity{}
	limitGPUQuota := resource.Quantity{}
	limitGPUQuotaUsed := resource.Quantity{}
	gpuQuotas, err := throttleClient.ThrottlecontrollerV1alpha1().GPUQuotas(pod.Namespace).List(metav1.ListOptions{})
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}
	for i := range gpuQuotas.Items {
		if request, found := gpuQuotas.Items[i].Spec.Hard[throttlev1alpha1.ResourceRequestsGPU]; found {
			requestGPUQuota.Add(request)
		}
		if request, found := gpuQuotas.Items[i].Status.Used[throttlev1alpha1.ResourceRequestsGPU]; found {
			requestGPUQuotaUsed.Add(request)
		}
		if limit, found := gpuQuotas.Items[i].Spec.Hard[throttlev1alpha1.ResourceLimitsGPU]; found {
			limitGPUQuota.Add(limit)
		}
		if limit, found := gpuQuotas.Items[i].Status.Used[throttlev1alpha1.ResourceLimitsGPU]; found {
			limitGPUQuotaUsed.Add(limit)
		}
	}
	if requestGPUQuota.IsZero() && limitGPUQuota.IsZero() {
		glog.Infof("no GPU Quota defined for Namespece: %s", pod.Namespace)
		return &reviewResponse
	}

	var msg string
	requestGPU, limitGPU := calcPodGPUUsage(&pod)
	oldRequestGPU := resource.Quantity{}
	oldLimitGPU := resource.Quantity{}
	if isUpdate {
		oldRequestGPU, oldLimitGPU = calcPodGPUUsage(&oldPod)
	}

	requestGPU.Sub(oldRequestGPU)
	limitGPU.Sub(oldLimitGPU)

	requestGPUQuota.Sub(requestGPUQuotaUsed)
	limitGPUQuota.Sub(limitGPUQuotaUsed)
	if requestGPUQuota.Cmp(requestGPU) < 0 {
		requestGPUQuota.Add(requestGPUQuotaUsed)
		reviewResponse.Allowed = false
		msg = msg + fmt.Sprintf("exceeded quota: %s, requested(%s) + used(%s) > limited(%s)",
			throttlev1alpha1.ResourceRequestsGPU,
			requestGPU.String(),
			requestGPUQuotaUsed.String(),
			requestGPUQuota.String(),
		)
	}
	if limitGPUQuota.Cmp(limitGPU) < 0 {
		limitGPUQuota.Add(limitGPUQuotaUsed)
		reviewResponse.Allowed = false
		msg = msg + fmt.Sprintf("exceeded quota: %s, requested(%s) + used(%s) > limited(%s)",
			throttlev1alpha1.ResourceLimitsGPU,
			limitGPU.String(),
			limitGPUQuotaUsed.String(),
			limitGPUQuota.String(),
		)
	}
	if !reviewResponse.Allowed {
		reviewResponse.Result = &metav1.Status{Message: strings.TrimSpace(msg)}
	}
	return &reviewResponse
}

type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func serve(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("contentType=%s, expect application/json", contentType)
		return
	}

	var reviewResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Error(err)
		reviewResponse = toAdmissionResponse(err)
	} else {
		reviewResponse = admit(ar)
	}

	response := v1beta1.AdmissionReview{}
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = ar.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	ar.Request.Object = runtime.RawExtension{}
	ar.Request.OldObject = runtime.RawExtension{}

	resp, err := json.Marshal(response)
	if err != nil {
		glog.Error(err)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Error(err)
	}
}

func servePods(w http.ResponseWriter, r *http.Request) {
	serve(w, r, admitPods)
}

func main() {
	var config Config
	config.addFlags()
	flag.Parse()

	// init throttle client
	cfg, err := clientcmd.BuildConfigFromFlags(config.MasterURL, config.KubeConfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	throttleClient, err = clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building throttle clientset: %s", err.Error())
	}

	http.HandleFunc("/", servePods)
	clientset := getClient()
	server := &http.Server{
		Addr:      ":443",
		TLSConfig: configTLS(config, clientset),
		//TLSConfig: &tls.Config{
		//	ClientAuth: tls.NoClientCert,
		//},
	}
	server.ListenAndServeTLS("", "")
	//server.ListenAndServeTLS(config.CertFile, config.KeyFile)
}
