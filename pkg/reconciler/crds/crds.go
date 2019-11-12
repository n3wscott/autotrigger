/*
Copyright 2019 The Knative Authors

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

package crds

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/n3wscott/autotrigger/pkg/reconciler/autotrigger"
)

type runningController struct {
	gvr        schema.GroupVersionResource
	controller *controller.Impl
	cancel     context.CancelFunc
}

var addressable = `duck.knative.dev/addressable`

// Reconciler implements controller.Reconciler for Addressable resources.
type Reconciler struct {

	// Injected

	crdLister apiextensionsv1beta1.CustomResourceDefinitionLister
	ogctx     context.Context
	ogcmw     configmap.Watcher

	// Local state

	controllers map[schema.GroupVersionResource]runningController
	kToR        map[schema.GroupVersionKind]schema.GroupVersionResource
	lock        sync.Mutex
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Create maps if needed.
	if c.controllers == nil {
		c.controllers = make(map[schema.GroupVersionResource]runningController)
	}
	if c.kToR == nil {
		c.kToR = make(map[schema.GroupVersionKind]schema.GroupVersionResource)
	}

	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %q", key)
		return nil
	}

	crd, err := c.crdLister.Get(name)
	if err != nil {
		logger.Errorf("unable to get CustomResourceDefinition[%q]: %s", key, err)
		return nil
	}

	logger.Info("addressable label == ", crd.Labels[addressable])

	if l, ok := crd.Labels[addressable]; ok && l == "true" {
		return c.ensureAddressableController(ctx, crd)
	}

	return nil
}

func (c *Reconciler) ensureAddressableController(ctx context.Context, crd *v1beta1.CustomResourceDefinition) error {
	logger := logging.FromContext(ctx)
	var gvr *schema.GroupVersionResource

	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}

		// TODO: deal with cluster scoped resources.
		gvr = &schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  v.Name,
			Resource: crd.Spec.Names.Plural,
		}

		c.kToR[schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: v.Name,
			Kind:    crd.Spec.Names.Kind,
		}] = *gvr
	}
	if gvr == nil {
		return fmt.Errorf("unable to find gvr for %s", crd.ClusterName)
	}

	rc, found := c.controllers[*gvr]
	if found {
		if crd.DeletionTimestamp != nil {
			logger.Infof("stopping autotrigger reconciler for gvr %q", rc.gvr.String())

			c.lock.Lock()
			rc.cancel()
			delete(c.controllers, *gvr)
			c.lock.Unlock()
		}
		return nil
	}

	// Auto Trigger Constructor
	atc := autotrigger.NewControllerConstructor(crd.ClusterName, *gvr, c)
	// Auto Trigger Context
	atctx, cancel := context.WithCancel(c.ogctx)
	// Auto Trigger
	at := atc(atctx, c.ogcmw)

	rc = runningController{
		gvr:        *gvr,
		controller: at,
		cancel:     cancel,
	}

	c.lock.Lock()
	c.controllers[rc.gvr] = rc
	c.lock.Unlock()

	logger.Infof("starting autotrigger reconciler for gvr %q", rc.gvr.String())
	go func(c *controller.Impl) {
		if err := c.Run(2, atctx.Done()); err != nil {
			logger.Errorf("unable to start autotrigger reconciler for gvr %q", rc.gvr.String())
		}
	}(rc.controller)

	logger.Infof("-----AutoTriggering-------")
	for k, _ := range c.controllers {
		logger.Infof(" - %q", k)
	}
	logger.Infof("==========================")

	return nil
}

func (c *Reconciler) IsGVKAddressable(ctx context.Context, gvk schema.GroupVersionKind) bool {
	_, found := c.kToR[gvk]
	return found
}
