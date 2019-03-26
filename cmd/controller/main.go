package main

///*
//Copyright 2019 The Knative Authors
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//*/
//
//package main
//
//import (
//	"context"
//	"flag"
//	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions"
//	"github.com/knative/pkg/configmap"
//	"github.com/knative/pkg/controller"
//	"github.com/knative/pkg/signals"
//	"github.com/knative/pkg/system"
//	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"
//	"github.com/n3wscott/autotrigger/pkg/logging"
//	"github.com/n3wscott/autotrigger/pkg/metrics"
//	"github.com/n3wscott/autotrigger/pkg/reconciler"
//	"github.com/n3wscott/autotrigger/pkg/reconciler/v1alpha1/autotrigger"
//	"go.uber.org/zap"
//	"k8s.io/client-go/rest"
//	"k8s.io/client-go/tools/clientcmd"
//	"log"
//	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
//	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
//)
//
//const (
//	threadsPerController = 2
//	component            = "autotriggercontroller"
//)
//
//var (
//	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
//	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
//)
//
//func main() {
//	flag.Parse()
//	loggingConfigMap, err := configmap.Load("/etc/config-logging")
//	if err != nil {
//		log.Fatalf("Error loading logging configuration: %v", err)
//	}
//	loggingConfig, err := logging.NewConfigFromMap(loggingConfigMap)
//	if err != nil {
//		log.Fatalf("Error parsing logging configuration: %v", err)
//	}
//	logger, atomicLevel := logging.NewLoggerFromConfig(loggingConfig, component)
//	defer logger.Sync()
//
//	// set up signals so we handle the first shutdown signal gracefully
//	stopCh := signals.SetupSignalHandler()
//
//	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
//	if err != nil {
//		logger.Fatalw("Error building kubeconfig", zap.Error(err))
//	}
//
//	// We run 6 controllers, so bump the defaults.
//	cfg.QPS = 6 * rest.DefaultQPS
//	cfg.Burst = 6 * rest.DefaultBurst
//
//	opts := reconciler.NewOptions(context.Background(), cfg, stopCh)
//
//	servingInformerFactory := servinginformers.NewSharedInformerFactory(opts.ServingClientSet, opts.ResyncPeriod)
//
//	eventingInformerFactory := eventinginformers.NewSharedInformerFactory(opts.EventingClientSet, opts.ResyncPeriod)
//
//	serviceInformer := servingInformerFactory.Serving().V1alpha1().Services()
//
//	triggerInformer := eventingInformerFactory.Eventing().V1alpha1().Triggers()
//
//	// Build all of our controllers, with the clients constructed above.
//	// Add new controllers to this array.
//	controllers := []*controller.Impl{
//		autotrigger.NewController(
//			opts,
//			serviceInformer,
//			triggerInformer,
//		),
//	}
//
//	configMapWatcher := configmap.NewInformedWatcher(opts.KubeClientSet, system.Namespace())
//	// Watch the logging config map and dynamically update logging levels.
//	configMapWatcher.Watch(logging.ConfigName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))
//	// Watch the observability config map and dynamically update metrics exporter.
//	configMapWatcher.Watch(metrics.ObservabilityConfigName, metrics.UpdateExporterFromConfigMap(component, logger))
//
//	if err := controller.StartInformers(stopCh, serviceInformer.Informer(), triggerInformer.Informer()); err != nil {
//		logger.Fatalw("failed to start informers", zap.Error(err))
//	}
//
//	if err := configMapWatcher.Start(stopCh); err != nil {
//		logger.Fatalw("failed to start configuration manager", zap.Error(err))
//	}
//
//	// Start all of the controllers.
//	logger.Info("Starting....")
//	controller.StartAll(stopCh, controllers...)
//
//	<-stopCh
//}
