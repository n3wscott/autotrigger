package main

import (
	"github.com/n3wscott/autotrigger/pkg/reconciler/v1alpha1/autotrigger"
	"os"

	controllers "sigs.k8s.io/controller-runtime"
)

func main() {
	var log = controllers.Log.WithName("builder-examples")

	manager, err := controllers.NewManager(controllers.GetConfigOrDie(), controllers.Options{})
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	if err := autotrigger.Add(manager); err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := manager.Start(controllers.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}
