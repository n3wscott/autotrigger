package autotrigger

import (
	"context"
	"knative.dev/pkg/apis/duck"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	eventingclient "github.com/knative/eventing/pkg/client/injection/client"
	"github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	"github.com/knative/serving/pkg/client/injection/informers/serving/v1beta1/service"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

// NewController returns a new HPA reconcile controller.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	triggerInformer := trigger.Get(ctx)
	serviceInformer := service.Get(ctx)

	c := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		triggerLister:     triggerInformer.Lister(),
		serviceLister:     serviceInformer.Lister(),
	}
	impl := controller.NewImpl(c, logger, "Autotrigger")

	logger.Info("Setting up event handlers")

	duck.TypedInformerFactory{}

	serviceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	triggerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
