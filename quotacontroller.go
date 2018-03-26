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
	"fmt"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	throttlev1alpha1 "github.com/xychu/throttle/pkg/apis/throttlecontroller/v1alpha1"
	clientset "github.com/xychu/throttle/pkg/client/clientset/versioned"
	throttlescheme "github.com/xychu/throttle/pkg/client/clientset/versioned/scheme"
	informers "github.com/xychu/throttle/pkg/client/informers/externalversions"
	listers "github.com/xychu/throttle/pkg/client/listers/throttlecontroller/v1alpha1"
)

const controllerAgentName = "throttle-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a GPUQuota is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a GPUQuota fails
	// to sync due to a Pod of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Pod already existing
	MessageResourceExists = "Resource %q already exists and is not managed by GPUQuota"
	// MessageResourceSynced is the message used for an Event fired when a GPUQuota
	// is synced successfully
	MessageResourceSynced = "GPUQuota synced successfully"
)

// Controller is the controller implementation for GPUQuota resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// throttleclientset is a clientset for our own API group
	throttleclientset clientset.Interface

	podsLister      corelisters.PodLister
	podsSynced      cache.InformerSynced
	gpuQuotasLister listers.GPUQuotaLister
	gpuQuotasSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new throttle controller
func NewController(
	kubeclientset kubernetes.Interface,
	throttleclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	throttleInformerFactory informers.SharedInformerFactory) *Controller {

	// obtain references to shared index informers for the Pod and GPUQuota types.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	gpuQuotaInformer := throttleInformerFactory.Throttlecontroller().V1alpha1().GPUQuotas()

	// Create event broadcaster
	// Add throttle-controller types to the default Kubernetes Scheme so Events can be
	// logged for throttle-controller types.
	throttlescheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		throttleclientset: throttleclientset,
		podsLister:        podInformer.Lister(),
		podsSynced:        podInformer.Informer().HasSynced,
		gpuQuotasLister:   gpuQuotaInformer.Lister(),
		gpuQuotasSynced:   gpuQuotaInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "GPUQuotas"),
		recorder:          recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when GPUQuota resources change
	gpuQuotaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueGPUQuota,
		UpdateFunc: func(old, new interface{}) {
			// TODO(xychu): cmp if quota values have changed
			controller.enqueueGPUQuota(new)
		},
	})
	// Set up an event handler for when Pod resources change. This
	// handler will lookup the GPU resource requests of the given Pod.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Pod)
			oldDepl := old.(*corev1.Pod)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Pods.
				// Two different versions of the same Pod will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting GPUQuota controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.gpuQuotasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process GPUQuota resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// GPUQuota resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the GPUQuota resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the GPUQuota resource with this namespace/name
	gpuQuota, err := c.gpuQuotasLister.GPUQuotas(namespace).Get(name)
	gpuQuotaCopy := gpuQuota.DeepCopy()
	if err != nil {
		// The GPUQuota resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("gpuQuota '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Re-calc GPUQuota Status under current Namespace
	// TODO(xychu): add scope support, maybe evaluator pattern.
	pods, err := c.podsLister.Pods(gpuQuota.Namespace).List(labels.Everything())
	if err != nil {
		runtime.HandleError(fmt.Errorf("error getting pods under Namespace: %s", gpuQuota.Namespace))
		return nil
	}

	used := corev1.ResourceList{}
	newRequestUsage := resource.Quantity{}
	newLimitUsage := resource.Quantity{}
	for i := range pods {
		request_value := resource.Quantity{}
		limit_value := resource.Quantity{}
		for j := range pods[i].Spec.Containers {
			if request, found := pods[i].Spec.Containers[j].Resource.Requests[throttlev1alpha1.ResourceRequestsGPU]; found {
				request_value.Add(request)
			}
			if limit, found := pods[i].Spec.Containers[j].Resource.Limits[throttlev1alpha1.ResourceRequestsGPU]; found {
				limit_value.Add(limit)
			}
		}
		// InitContainers are run **sequentially** before other containers start, so the highest
		// init container resource is compared against the sum of app containers to determine
		// the effective usage for both requests and limits.
		for j := range pods[i].Spec.InitContainers {
			if request, found := pods[i].Spec.InitContainers[j].Resource.Requests[throttlev1alpha1.ResourceRequestsGPU]; found {
				if request_value.Cmp(request) < 0 {
					request_value = request.Copy()
				}
			}
			if limit, found := pods[i].Spec.InitContainers[j].Resource.Limits[throttlev1alpha1.ResourceRequestsGPU]; found {
				if limit_value.Cmp(limit) < 0 {
					limit_value = limit.Copy()
				}
			}
		}
		newRequestUsage.Add(request_value)
		newLimitUsage.Add(limit_value)
	}
	used[throttlev1alpha1.ResourceRequestsGPU] = newRequestUsage
	used[throttlev1alpha1.ResourceLimitsGPU] = newLimitUsage

	newStatus := corev1.ResourceQuotaStatus{}
	newStatus.Hard = gpuQuota.Hard
	newStatus.Used = used

	gpuQuotaCopy.Status = newStatus
	// Finally, we update the status block of the GPUQuota resource to reflect the
	// current state of the world
	err = c.updateGPUQuotaStatus(gpuQuotaCopy)
	if err != nil {
		runtime.HandleError(fmt.Errorf("error updating gpuQuota: %v", gpuQuota))
		return err
	}

	c.recorder.Event(gpuQuota, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateGPUQuotaStatus(gpuQuota *throttlev1alpha1.GPUQuota) error {
	_, err := c.throttleclientset.ThrottlecontrollerV1alpha1().GPUQuotas(gpuQuota.Namespace).Update(gpuQuota)
	return err
}

// enqueueGPUQuota takes a GPUQuota resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than GPUQuota.
func (c *Controller) enqueueGPUQuota(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the GPU resource that consumed by it. It does this by looking up the
// container resources.
// It then enqueues that GPUQuota resource to be processed. If the object does not
// have requested any GPU resource, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	pod, okay := obj.(*corev1.Pod)
	if !okay {
		runtime.HandleError(fmt.Errorf("error converting object, invalid obj: %v", obj))
		return
	}

	usedGPU := false
	for i := range pod.Spec.Containers {
		requests := pod.Spec.Containers[i].Resources.Requests
		limits := pod.Spec.Containers[i].Resources.Limits

		if _, found := requests[throttlev1alpha1.ResourceGPU]; found {
			usedGPU = true
			break
		}
		if _, found := limits[throttlev1alpha1.ResourceGPU]; found {
			usedGPU = true
			break
		}
	}
	if !usedGPU {
		for i := range pod.Spec.InitContainers {
			requests := pod.Spec.InitContainers[i].Resources.Requests
			limits := pod.Spec.InitContainers[i].Resources.Limits

			if _, found := requests[throttlev1alpha1.ResourceGPU]; found {
				usedGPU = true
				break
			}
			if _, found := limits[throttlev1alpha1.ResourceGPU]; found {
				usedGPU = true
				break
			}
		}
	}

	if usedGPU {
		gpuQuotas, err := c.gpuQuotasLister.GPUQuotas(object.GetNamespace()).List(labels.Everything())
		if err != nil {
			msg := fmt.Sprintf("error getting gpuQuotas under Namespace '%s'", object.GetNamespace())
			glog.V(4).Infof(msg)
			runtime.HandleError(fmt.Errorf(msg))
			return
		}

		for i := range gpuQuotas {
			c.enqueueGPUQuota(gpuQuotas[i])
		}
		return
	}
}
