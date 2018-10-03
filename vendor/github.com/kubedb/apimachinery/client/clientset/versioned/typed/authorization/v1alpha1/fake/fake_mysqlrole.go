/*
Copyright 2018 The KubeDB Authors.

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
	v1alpha1 "github.com/kubedb/apimachinery/apis/authorization/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMySQLRoles implements MySQLRoleInterface
type FakeMySQLRoles struct {
	Fake *FakeAuthorizationV1alpha1
	ns   string
}

var mysqlrolesResource = schema.GroupVersionResource{Group: "authorization.kubedb.com", Version: "v1alpha1", Resource: "mysqlroles"}

var mysqlrolesKind = schema.GroupVersionKind{Group: "authorization.kubedb.com", Version: "v1alpha1", Kind: "MySQLRole"}

// Get takes name of the mySQLRole, and returns the corresponding mySQLRole object, and an error if there is any.
func (c *FakeMySQLRoles) Get(name string, options v1.GetOptions) (result *v1alpha1.MySQLRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(mysqlrolesResource, c.ns, name), &v1alpha1.MySQLRole{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MySQLRole), err
}

// List takes label and field selectors, and returns the list of MySQLRoles that match those selectors.
func (c *FakeMySQLRoles) List(opts v1.ListOptions) (result *v1alpha1.MySQLRoleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(mysqlrolesResource, mysqlrolesKind, c.ns, opts), &v1alpha1.MySQLRoleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MySQLRoleList{ListMeta: obj.(*v1alpha1.MySQLRoleList).ListMeta}
	for _, item := range obj.(*v1alpha1.MySQLRoleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested mySQLRoles.
func (c *FakeMySQLRoles) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(mysqlrolesResource, c.ns, opts))

}

// Create takes the representation of a mySQLRole and creates it.  Returns the server's representation of the mySQLRole, and an error, if there is any.
func (c *FakeMySQLRoles) Create(mySQLRole *v1alpha1.MySQLRole) (result *v1alpha1.MySQLRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(mysqlrolesResource, c.ns, mySQLRole), &v1alpha1.MySQLRole{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MySQLRole), err
}

// Update takes the representation of a mySQLRole and updates it. Returns the server's representation of the mySQLRole, and an error, if there is any.
func (c *FakeMySQLRoles) Update(mySQLRole *v1alpha1.MySQLRole) (result *v1alpha1.MySQLRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(mysqlrolesResource, c.ns, mySQLRole), &v1alpha1.MySQLRole{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MySQLRole), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMySQLRoles) UpdateStatus(mySQLRole *v1alpha1.MySQLRole) (*v1alpha1.MySQLRole, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(mysqlrolesResource, "status", c.ns, mySQLRole), &v1alpha1.MySQLRole{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MySQLRole), err
}

// Delete takes name of the mySQLRole and deletes it. Returns an error if one occurs.
func (c *FakeMySQLRoles) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(mysqlrolesResource, c.ns, name), &v1alpha1.MySQLRole{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMySQLRoles) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(mysqlrolesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MySQLRoleList{})
	return err
}

// Patch applies the patch and returns the patched mySQLRole.
func (c *FakeMySQLRoles) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MySQLRole, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mysqlrolesResource, c.ns, name, data, subresources...), &v1alpha1.MySQLRole{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MySQLRole), err
}
