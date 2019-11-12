package crds

import (
	"context"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	_ "github.com/n3wscott/autotrigger/pkg/reconciler/autotrigger"
	crdinfomer "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	crdInformer := crdinfomer.Get(ctx)

	c := &Reconciler{
		crdLister: crdInformer.Lister(),
		ogctx:     ctx,
		ogcmw:     cmw,
	}
	impl := controller.NewImpl(c, logger, "AddressableCRDs")

	logger.Info("Setting up event handlers")
	crdInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
