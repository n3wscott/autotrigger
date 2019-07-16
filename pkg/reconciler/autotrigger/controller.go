package autotrigger

import (
	"context"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis/duck"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	eventingclient "github.com/knative/eventing/pkg/client/injection/client"
	"github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"
	"knative.dev/pkg/injection/clients/dynamicclient"
)

// NewController returns a new HPA reconcile controller.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	triggerInformer := trigger.Get(ctx)
	//serviceInformer := service.Get(ctx)

	addressinformer := duck.TypedInformerFactory{
		Client:       dynamicclient.Get(ctx),
		Type:         &duckv1beta1.AddressableType{},
		ResyncPeriod: 10 * time.Hour,
		StopChannel:  ctx.Done(),
	}

	//gvr := schema.GroupVersionResource{
	//	Group:    "n3wscott.com",
	//	Version:  "v1alpha1",
	//	Resource: "tasks",
	//}
	gvr := schema.GroupVersionResource{
		Group:    "serving.knative.dev",
		Version:  "v1alpha1",
		Resource: "services",
	}
	addressInformer, addressLister, err := addressinformer.Get(gvr)
	if err != nil {
		panic(err)
	}

	c := &Reconciler{
		eventingClientSet: eventingclient.Get(ctx),
		triggerLister:     triggerInformer.Lister(),
		addressableLister: addressLister,
	}
	impl := controller.NewImpl(c, logger, "Autotrigger-"+uuid.New().String())

	logger.Info("Setting up event handlers")

	addressInformer.AddEventHandler(controller.HandleAll(impl.Enqueue))

	triggerInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
