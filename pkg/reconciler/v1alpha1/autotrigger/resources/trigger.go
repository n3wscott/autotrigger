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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"github.com/n3wscott/autotrigger/pkg/reconciler/v1alpha1/autotrigger/resources/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	filterAnnotation = "trigger.eventing.knative.dev/filter"
)

type brokerFilters struct {
	Broker string `json:"broker,omitempty"`
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
}

// MakeTrigger creates a Trigger from a Service object.
func MakeTriggers(ctx context.Context, service *servingv1alpha1.Service) ([]*eventingv1alpha1.Trigger, error) {
	logger := logging.FromContext(ctx)

	rawFilter, ok := service.Annotations[filterAnnotation]
	if !ok {
		return []*eventingv1alpha1.Trigger(nil), nil
	}

	filters := make([]brokerFilters, 0)
	if rawFilter == "" || rawFilter == "[{}]" || rawFilter == "[]" {
		filters = append(filters, brokerFilters{})
	} else if err := json.Unmarshal([]byte(rawFilter), &filters); err != nil {
		return nil, fmt.Errorf("failed to extract auto-trigger from service: %s", err.Error())
	}

	logger.Errorf("found filters %+v for %s from %q", filters, service.Name, rawFilter)

	triggers := make([]*eventingv1alpha1.Trigger, 0)

	// Kind:       "Service",
	// APIVersion: "serving.knative.dev/v1alpha1",

	subscriber := &eventingv1alpha1.SubscriberSpec{
		Ref: &corev1.ObjectReference{
			APIVersion: "serving.knative.dev/v1alpha1", // service.APIVersion
			Kind:       "Service",                      // service.Kind
			Name:       service.Name,
		},
	}

	for _, filter := range filters {
		t := &eventingv1alpha1.Trigger{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: names.Trigger(service) + "-",
				Namespace:    service.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*kmeta.NewControllerRef(service),
				},
				Labels: MakeLabels(service),
			},
			Spec: eventingv1alpha1.TriggerSpec{
				Broker: filter.Broker,
				Filter: &eventingv1alpha1.TriggerFilter{
					SourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
						Source: filter.Source,
						Type:   filter.Type,
					},
				},
				Subscriber: subscriber,
			},
		}
		triggers = append(triggers, t)
	}

	logger.Errorf("made %d triggers for %s", len(triggers), service.Name)

	return triggers, nil
}
