package reconciler

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AddressableInfo interface {
	IsGVKAddressable(ctx context.Context, gvk schema.GroupVersionKind) bool
}
