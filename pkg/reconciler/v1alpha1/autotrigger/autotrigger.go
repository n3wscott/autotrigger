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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/n3wscott/autotrigger/pkg/reconciler/v1alpha1/autotrigger/resources"
)

func Add(manager controllers.Manager) error {
	return controllers.
		NewControllerManagedBy(manager).
		For(&servingv1alpha1.Service{}).
		Owns(&eventingv1alpha1.Trigger{}).
		Complete(&Reconciler{Client: manager.GetClient()})
}

type Reconciler struct {
	client.Client
}

func (r *Reconciler) Reconcile(req controllers.Request) (controllers.Result, error) {
	ctx := context.TODO()

	// Read the Service
	s := &servingv1alpha1.Service{}
	err := r.Get(ctx, req.NamespacedName, s)
	if err != nil {
		return controllers.Result{}, err
	} else if !AutoTriggerEnabled(s) {
		return controllers.Result{}, nil
	}

	triggerList := &eventingv1alpha1.TriggerList{}
	err = r.List(ctx, client.InNamespace(req.Namespace).MatchingLabels(MakeLabels(s)), triggerList)
	if err != nil {
		return controllers.Result{}, err
	}

	triggers := filterTriggers(s, triggerList.Items)

	return r.reconcileTriggers(ctx, s, triggers)
}

func filterTriggers(service *servingv1alpha1.Service, triggers []eventingv1alpha1.Trigger) []*eventingv1alpha1.Trigger {
	filteredTriggers := []*eventingv1alpha1.Trigger(nil)
	for _, trigger := range triggers {
		if metav1.IsControlledBy(&trigger, service) {
			filteredTriggers = append(filteredTriggers, &trigger)
		}
	}
	return filteredTriggers
}

func (c *Reconciler) reconcileTriggers(ctx context.Context, service *servingv1alpha1.Service, existingTriggers []*eventingv1alpha1.Trigger) (controllers.Result, error) {
	logger := logging.FromContext(ctx)

	desiredTriggers, err := MakeTriggers(service)
	if err != nil {
		return controllers.Result{}, err
	} else if len(desiredTriggers) == 0 {
		// No auto-triggers for this service.
		return controllers.Result{}, nil
	}

	for _, desiredTrigger := range desiredTriggers {

		var trigger *eventingv1alpha1.Trigger
		existingTriggers, trigger = extractTriggerLike(existingTriggers, desiredTrigger)

		if trigger == nil {
			if err := c.Create(ctx, desiredTrigger); err != nil {
				return controllers.Result{}, err
			}
		}
	}

	// Delete all the remaining triggers.
	for _, trigger := range existingTriggers {
		if err := c.Delete(ctx, trigger); err != nil {
			logger.Errorf("Failed to delete Trigger %q: %v", trigger.Name, err)
		}
	}

	return controllers.Result{}, nil
}

func triggerSemanticEquals(desiredTrigger, trigger *eventingv1alpha1.Trigger) bool {
	// ignore differences in DeprecatedGeneration.
	desiredTrigger.Spec.DeprecatedGeneration = trigger.Spec.DeprecatedGeneration

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
