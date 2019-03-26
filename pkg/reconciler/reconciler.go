/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciler

import (
	"context"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging"
	"k8s.io/client-go/rest"

	eventingclientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	"go.uber.org/zap"
)

const (
	LoggingConfigName = "config-logging"
)

// Options defines the common reconciler options.
// We define this to reduce the boilerplate argument list when
// creating our controllers.
type Options struct {
	// Include base options
	controller.Options

	// These are custom:
	ServingClientSet  servingclientset.Interface
	EventingClientSet eventingclientset.Interface

	StatsReporter StatsReporter
}

func NewOptions(ctx context.Context, cfg *rest.Config, stopCh <-chan struct{}) Options {
	logger := logging.FromContext(ctx)

	servingClient, err := servingclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalw("Error building serving clientset", zap.Error(err))
	}

	eventingClient, err := eventingclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalw("Error building eventing clientset", zap.Error(err))
	}

	opts := Options{
		Options:           controller.NewOptions(ctx, cfg, stopCh),
		ServingClientSet:  servingClient,
		EventingClientSet: eventingClient,
	}
	return opts
}

// Base implements the core controller logic, given a Reconciler.
type Base struct {
	*controller.Base

	// ServingClientSet allows us to configure Serving objects
	ServingClientSet servingclientset.Interface

	// EventingClientSet allows us to configure Eventing objects
	EventingClientSet eventingclientset.Interface

	// StatsReporter reports reconciler's metrics.
	StatsReporter StatsReporter
}

// NewBase instantiates a new instance of Base implementing
// the common & boilerplate code between our reconcilers.
func NewBase(opt Options, controllerAgentName string) *Base {
	base := controller.NewBase(opt.Options, controllerAgentName)

	statsReporter := opt.StatsReporter
	if statsReporter == nil {
		base.Logger.Debug("Creating stats reporter")
		var err error
		statsReporter, err = NewStatsReporter(controllerAgentName)
		if err != nil {
			base.Logger.Fatal(err)
		}
	}

	recBase := &Base{
		Base:              base,
		ServingClientSet:  opt.ServingClientSet,
		EventingClientSet: opt.EventingClientSet,
		StatsReporter:     statsReporter,
	}

	return recBase
}
