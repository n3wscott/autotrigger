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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingclientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	servingv1beta1 "github.com/knative/serving/pkg/apis/serving/v1beta1"
	servinglisters "github.com/knative/serving/pkg/client/listers/serving/v1beta1"
	"github.com/n3wscott/autotrigger/pkg/reconciler/autotrigger/resources"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// Reconciler implements controller.Reconciler for Service resources.
type Reconciler struct {
	// Serving
	serviceLister servinglisters.ServiceLister

	// Eventing
	eventingClientSet eventingclientset.Interface
	triggerLister     eventinglisters.TriggerLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the Service resource with this namespace/name
	original, err := c.serviceLister.Services(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("service %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	} else if !resources.AutoTriggerEnabled(original) {
		return nil
	}

	// Don't modify the informers copy
	service := original.DeepCopy()

	// Reconcile this copy of the service. We do not control service, so do not update status.
	return c.reconcile(ctx, service)
}

func (c *Reconciler) reconcile(ctx context.Context, service *servingv1beta1.Service) error {
	logger := logging.FromContext(ctx)

	if service.GetDeletionTimestamp() != nil {
		// All triggers that were created from service are owned by that service, so they will be cleaned up.
		return nil
	}

	triggers, err := c.triggerLister.Triggers(service.Namespace).List(labels.SelectorFromSet(resources.MakeLabels(service)))

	triggers = filterTriggers(service, triggers)

	if errors.IsNotFound(err) || len(triggers) == 0 { // TODO: might not get an IsNotFound error for list.
		triggers, err = c.createTriggers(ctx, service)
		if err != nil {
			logger.Errorf("Failed to create Triggers for Service %q: %v", service.Name, err)
			return err
		}
	} else if err != nil {
		logger.Errorw(fmt.Sprintf("Failed to Get Triggers for Service %q", service.Name), zap.Error(err))
		return err
	} else if triggers, err = c.reconcileTriggers(ctx, service, triggers); err != nil {
		logger.Errorw(fmt.Sprintf("Failed to reconcile Triggers for Service %q", service.Name), zap.Error(err))
		return err
	}

	return nil
}

func filterTriggers(service *servingv1beta1.Service, triggers []*eventingv1alpha1.Trigger) []*eventingv1alpha1.Trigger {
	filteredTriggers := []*eventingv1alpha1.Trigger(nil)
	for _, trigger := range triggers {
		if metav1.IsControlledBy(trigger, service) {
			filteredTriggers = append(filteredTriggers, trigger)
		}
	}
	return filteredTriggers
}

func (c *Reconciler) createTriggers(ctx context.Context, service *servingv1beta1.Service) ([]*eventingv1alpha1.Trigger, error) {
	logger := logging.FromContext(ctx)

	triggers, err := resources.MakeTriggers(service)
	if err != nil {
		return nil, err
	}
	var retErr error
	createdTriggers := []*eventingv1alpha1.Trigger(nil)
	for _, trigger := range triggers {
		createdTrigger, err := c.eventingClientSet.EventingV1alpha1().Triggers(service.Namespace).Create(trigger)
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

func (c *Reconciler) reconcileTriggers(ctx context.Context, service *servingv1beta1.Service, existingTriggers []*eventingv1alpha1.Trigger) ([]*eventingv1alpha1.Trigger, error) {
	logger := logging.FromContext(ctx)

	_ = logger

	desiredTriggers, err := resources.MakeTriggers(service)
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
			trigger, err = c.eventingClientSet.EventingV1alpha1().Triggers(service.Namespace).Create(desiredTrigger)
			if err != nil {
				return nil, err
			}
		}

		triggers = append(triggers, trigger)
	}

	// Delete all the remaining triggers.
	for _, trigger := range existingTriggers {
		err := c.eventingClientSet.EventingV1alpha1().Triggers(service.Namespace).Delete(trigger.Name, &metav1.DeleteOptions{})
		if err != nil {
			logger.Errorf("Failed to delete Trigger %q: %v", trigger.Name, err)
		}
	}

	// TODO: we need to look at the remaining existingTriggers and delete the ones that are leftover.

	return triggers, nil
}
