package main

import (
	"context"
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	var log = controllers.Log.WithName("builder-examples")

	manager, err := controllers.NewManager(controllers.GetConfigOrDie(), controllers.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = controllers.
		NewControllerManagedBy(manager). // Create the Controller
		For(&appsv1.ReplicaSet{}).       // ReplicaSet is the Application API
		Owns(&corev1.Pod{}).             // ReplicaSet owns Pods created by it
		Complete(&ReplicaSetReconciler{Client: manager.GetClient()})
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(controllers.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

// ReplicaSetReconciler is a simple Controller example implementation.
type ReplicaSetReconciler struct {
	client.Client
}

// Implement the business logic:
// This function will be called when there is a change to a ReplicaSet or a Pod with an OwnerReference
// to a ReplicaSet.
//
// * Read the ReplicaSet
// * Read the Pods
// * Set a Label on the ReplicaSet with the Pod count
func (a *ReplicaSetReconciler) Reconcile(req controllers.Request) (controllers.Result, error) {
	// Read the ReplicaSet
	rs := &appsv1.ReplicaSet{}
	err := a.Get(context.TODO(), req.NamespacedName, rs)
	if err != nil {
		return controllers.Result{}, err
	}

	// List the Pods matching the PodTemplate Labels
	pods := &corev1.PodList{}
	err = a.List(context.TODO(), pods, client.InNamespace(req.Namespace), client.MatchingLabels(rs.Spec.Template.Labels))
	if err != nil {
		return controllers.Result{}, err
	}

	// Update the ReplicaSet
	rs.Labels["pod-count"] = fmt.Sprintf("%v", len(pods.Items))
	err = a.Update(context.TODO(), rs)
	if err != nil {
		return controllers.Result{}, err
	}

	return controllers.Result{}, nil
}
