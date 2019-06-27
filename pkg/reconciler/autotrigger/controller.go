package autotrigger

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingclient "github.com/knative/eventing/pkg/client/injection/client"
	"github.com/knative/eventing/pkg/client/injection/informers/eventing/v1alpha1/trigger"

	"github.com/knative/serving/pkg/client/injection/informers/serving/v1beta1/service"
)

const (
	controllerAgentName = "autotrigger-controller"
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
		recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}
	impl := controller.NewImpl(c, logger, "Autotrigger")

	logger.Info("Setting up event handlers")

	serviceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	triggerInformer.Informer().AddEventHandler(controller.HandleAll(
		// Call the tracker's OnChanged method, but we've seen the objects
		// coming through this path missing TypeMeta, so ensure it is properly
		// populated.
		controller.EnsureTypeMeta(
			c.tracker.OnChanged,
			eventingv1alpha1.SchemeGroupVersion.WithKind("Trigger"),
		),
	))
	return impl
}
