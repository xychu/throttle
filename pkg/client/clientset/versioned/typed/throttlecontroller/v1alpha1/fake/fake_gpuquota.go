/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/xychu/throttle/pkg/apis/throttlecontroller/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeGPUQuotas implements GPUQuotaInterface
type FakeGPUQuotas struct {
	Fake *FakeThrottlecontrollerV1alpha1
	ns   string
}

var gpuquotasResource = schema.GroupVersionResource{Group: "throttlecontroller.example.com", Version: "v1alpha1", Resource: "gpuquotas"}

var gpuquotasKind = schema.GroupVersionKind{Group: "throttlecontroller.example.com", Version: "v1alpha1", Kind: "GPUQuota"}

// Get takes name of the gPUQuota, and returns the corresponding gPUQuota object, and an error if there is any.
func (c *FakeGPUQuotas) Get(name string, options v1.GetOptions) (result *v1alpha1.GPUQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(gpuquotasResource, c.ns, name), &v1alpha1.GPUQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GPUQuota), err
}

// List takes label and field selectors, and returns the list of GPUQuotas that match those selectors.
func (c *FakeGPUQuotas) List(opts v1.ListOptions) (result *v1alpha1.GPUQuotaList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(gpuquotasResource, gpuquotasKind, c.ns, opts), &v1alpha1.GPUQuotaList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.GPUQuotaList{}
	for _, item := range obj.(*v1alpha1.GPUQuotaList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested gPUQuotas.
func (c *FakeGPUQuotas) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(gpuquotasResource, c.ns, opts))

}

// Create takes the representation of a gPUQuota and creates it.  Returns the server's representation of the gPUQuota, and an error, if there is any.
func (c *FakeGPUQuotas) Create(gPUQuota *v1alpha1.GPUQuota) (result *v1alpha1.GPUQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(gpuquotasResource, c.ns, gPUQuota), &v1alpha1.GPUQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GPUQuota), err
}

// Update takes the representation of a gPUQuota and updates it. Returns the server's representation of the gPUQuota, and an error, if there is any.
func (c *FakeGPUQuotas) Update(gPUQuota *v1alpha1.GPUQuota) (result *v1alpha1.GPUQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(gpuquotasResource, c.ns, gPUQuota), &v1alpha1.GPUQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GPUQuota), err
}

// Delete takes name of the gPUQuota and deletes it. Returns an error if one occurs.
func (c *FakeGPUQuotas) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(gpuquotasResource, c.ns, name), &v1alpha1.GPUQuota{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeGPUQuotas) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(gpuquotasResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.GPUQuotaList{})
	return err
}

// Patch applies the patch and returns the patched gPUQuota.
func (c *FakeGPUQuotas) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.GPUQuota, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(gpuquotasResource, c.ns, name, data, subresources...), &v1alpha1.GPUQuota{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.GPUQuota), err
}
