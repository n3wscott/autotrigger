/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/injection"
	"os"
	"path/filepath"

	// The set of controllers this controller process runs.
	"github.com/n3wscott/autotrigger/pkg/reconciler/autotrigger"

	// This defines the shared main for injected controllers.
	"knative.dev/pkg/injection/sharedmain"
)

type GroupVersionResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

func (g GroupVersionResource) String() string {
	return fmt.Sprintf("%s.%s/%s", g.Resource, g.Group, g.Version)
}

func main() {

	var files []string

	root := "/etc/config-autotrigger"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		fmt.Println("FILE ---: ", file)
	}

	dat, err := ioutil.ReadFile("/etc/config-autotrigger/gvrs")
	if err != nil {
		panic(err)
	}
	fmt.Println("FILE --- gvrs:\n", string(dat))

	gvrs := []GroupVersionResource(nil)
	if err := json.Unmarshal(dat, &gvrs); err != nil {
		panic(err)
	}

	// AutoTriggerControllers
	atcs := []injection.ControllerConstructor(nil)

	for _, g := range gvrs {
		name := "Autotrigger-" + g.String()
		gvr := schema.GroupVersionResource{
			Group:    g.Group,
			Version:  g.Version,
			Resource: g.Resource,
		}
		_ = gvr

		fmt.Println("GVR --->", name)

		atcs = append(atcs, autotrigger.NewControllerConstructor(name, gvr))
	}

	sharedmain.Main("controller",
		atcs...,
	)
}
