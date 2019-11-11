package autotrigger

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

func NewControllerConstructor(name string, gvr schema.GroupVersionResource) injection.ControllerConstructor {
	return func(
		ctx context.Context,
		cmw configmap.Watcher,
	) *controller.Impl {
		logger := logging.FromContext(ctx)

		triggerInformer := triggerinformer.Get(ctx)

		addressinformer := &duck.TypedInformerFactory{
			Client:       dynamicclient.Get(ctx),
			Type:         &duckv1.AddressableType{},
			ResyncPeriod: 10 * time.Hour,
			StopChannel:  ctx.Done(),
		}

		addressInformer, addressLister, err := addressinformer.Get(gvr)
		if err != nil {
			panic(err)
		}

		c := &Reconciler{
			eventingClientSet: eventingclient.Get(ctx),
			triggerLister:     triggerInformer.Lister(),
			addressableLister: addressLister,
			gvr:               gvr,
		}
		impl := controller.NewImpl(c, logger, name)

		logger.Info("Setting up event handlers for %s", name)

		addressInformer.AddEventHandler(controller.HandleAll(impl.Enqueue))

		triggerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.Filter(eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})

		return impl
	}
}
