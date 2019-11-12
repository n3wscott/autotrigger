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

package autotrigger

import (
	"context"
	"fmt"
	"github.com/n3wscott/autotrigger/pkg/reconciler"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingclientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/n3wscott/autotrigger/pkg/reconciler/autotrigger/resources"
)

// Reconciler implements controller.Reconciler for Addressable resources.
type Reconciler struct {
	// Addressable
	addressableLister cache.GenericLister

	info reconciler.AddressableInfo

	// Eventing
	eventingClientSet eventingclientset.Interface
	triggerLister     eventinglisters.TriggerLister
	gvr               schema.GroupVersionResource
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	logger.Infof("Reconcile %s", c.gvr.String())

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the Addressable resource with this namespace/name
	runtimeobj, err := c.addressableLister.ByNamespace(namespace).Get(name)

	var ok bool
	var original *duckv1.AddressableType
	if original, ok = runtimeobj.(*duckv1.AddressableType); !ok {
		logger.Errorf("runtime object is not convertible to addressable type, key=%q", key)
		return nil
	}

	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("addressable %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	} else if !resources.AutoTriggerEnabled(original) {
		return nil
	}

	// Don't modify the informers copy
	// Reconcile this copy of the service. We do not control service, so do not update status.
	return c.reconcile(ctx, original.DeepCopy())
}

func (c *Reconciler) reconcile(ctx context.Context, addressable *duckv1.AddressableType) error {
	logger := logging.FromContext(ctx)

	if addressable.GetDeletionTimestamp() != nil {
		// All triggers that were created from service are owned by that service, so they will be cleaned up.
		return nil
	}

	for _, owner := range addressable.OwnerReferences {
		gvk := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
		if c.info.IsGVKAddressable(ctx, gvk) {
			// TODO: for now, we will not create triggers for children of addressable objects.
			return nil
		}
	}

	triggers, err := c.triggerLister.Triggers(addressable.Namespace).List(labels.SelectorFromSet(resources.MakeLabels(addressable)))

	triggers = filterTriggers(addressable, triggers)

	// TODO: the trigger should only be made on the top most labeled addressable resource in the owner chain.

	if errors.IsNotFound(err) || len(triggers) == 0 { // TODO: might not get an IsNotFound error for list.
		triggers, err = c.createTriggers(ctx, addressable)
		if err != nil {
			logger.Errorf("failed to create Triggers for Service %q: %v", addressable.Name, err)
			return err
		}
	} else if err != nil {
		logger.Errorw(fmt.Sprintf("failed to Get Triggers for Service %q", addressable.Name), zap.Error(err))
		return err
	} else if triggers, err = c.reconcileTriggers(ctx, addressable, triggers); err != nil {
		logger.Errorw(fmt.Sprintf("failed to reconcile Triggers for Service %q", addressable.Name), zap.Error(err))
		return err
	}

	return nil
}

func filterTriggers(addressable *duckv1.AddressableType, triggers []*eventingv1alpha1.Trigger) []*eventingv1alpha1.Trigger {
	filteredTriggers := []*eventingv1alpha1.Trigger(nil)
	for _, trigger := range triggers {
		if metav1.IsControlledBy(trigger, addressable) {
			filteredTriggers = append(filteredTriggers, trigger)
		}
	}
	return filteredTriggers
}

func (c *Reconciler) createTriggers(ctx context.Context, addressable *duckv1.AddressableType) ([]*eventingv1alpha1.Trigger, error) {
	logger := logging.FromContext(ctx)

	triggers, err := resources.MakeTriggers(addressable)
	if err != nil {
		return nil, err
	}
	var retErr error
	createdTriggers := []*eventingv1alpha1.Trigger(nil)
	for _, trigger := range triggers {
		createdTrigger, err := c.eventingClientSet.EventingV1alpha1().Triggers(addressable.Namespace).Create(trigger)
		if err != nil {
			logger.Errorf("failed to create trigger: %+v, %s", trigger, err.Error())
			retErr = err
			break
		}
		createdTriggers = append(createdTriggers, createdTrigger)
	}
	return createdTriggers, retErr
}

func triggerSemanticEquals(desiredTrigger, trigger *eventingv1alpha1.Trigger) bool {
	return equality.Semantic.DeepEqual(desiredTrigger.Spec, trigger.Spec) &&
		equality.Semantic.DeepEqual(desiredTrigger.ObjectMeta.Labels, trigger.ObjectMeta.Labels)
}

func extractTriggerLike(triggers []*eventingv1alpha1.Trigger, like *eventingv1alpha1.Trigger) ([]*eventingv1alpha1.Trigger, *eventingv1alpha1.Trigger) {
	for i, trigger := range triggers {
		if triggerSemanticEquals(like, trigger) {
			triggers = append(triggers[:i], triggers[i+1:]...)
			return triggers, trigger
		}
	}
	return triggers, nil
}

func (c *Reconciler) reconcileTriggers(ctx context.Context, addressable *duckv1.AddressableType, existingTriggers []*eventingv1alpha1.Trigger) ([]*eventingv1alpha1.Trigger, error) {
	logger := logging.FromContext(ctx)

	_ = logger

	desiredTriggers, err := resources.MakeTriggers(addressable)
	if err != nil {
		return nil, err
	}
	if len(desiredTriggers) == 0 {
		// No auto-triggers for this service.
		return nil, nil
	}

	triggers := []*eventingv1alpha1.Trigger(nil)

	for _, desiredTrigger := range desiredTriggers {

		var trigger *eventingv1alpha1.Trigger
		existingTriggers, trigger = extractTriggerLike(existingTriggers, desiredTrigger)

		if trigger == nil {
			var err error
			trigger, err = c.eventingClientSet.EventingV1alpha1().Triggers(addressable.Namespace).Create(desiredTrigger)
			if err != nil {
				return nil, err
			}
		}

		triggers = append(triggers, trigger)
	}

	// Delete all the remaining triggers.
	for _, trigger := range existingTriggers {
		err := c.eventingClientSet.EventingV1alpha1().Triggers(addressable.Namespace).Delete(trigger.Name, &metav1.DeleteOptions{})
		if err != nil {
			logger.Errorf("failed to delete Trigger %q: %v", trigger.Name, err)
		}
	}

	// TODO: we need to look at the remaining existingTriggers and delete the ones that are leftover.

	return triggers, nil
}
