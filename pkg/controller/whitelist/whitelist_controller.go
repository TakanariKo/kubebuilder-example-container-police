/*
Copyright 2018 TakanariKo.

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

package whitelist

import (
	"context"
	"log"
	"strings"

	policev1beta1 "foo.bar/pkg/apis/police/v1beta1"
	"foo.bar/pkg/controller/whitelist/accesslist"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new WhiteList Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this police.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWhiteList{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("sloop-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Sloop
	err = c.Watch(&source.Kind{Type: &policev1beta1.WhiteList{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by Sloop - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	whiteList = make(map[string]map[string]*accesslist.WhiteList)
	return nil
}

var whiteList map[string]map[string]*accesslist.WhiteList
var _ reconcile.Reconciler = &ReconcileWhiteList{}

// ReconcileWhiteList reconciles a WhiteList object
type ReconcileWhiteList struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a WhiteList object and makes changes based on the state read
// and what is in the WhiteList.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=police.k8s.io,resources=whitelists,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileWhiteList) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Deployment instance
	d := &appsv1.Deployment{}
	err := r.Get(context.TODO(), request.NamespacedName, d)
	if err == nil {
		// check Deployment image
		log.Printf("Checking Deployment %s/%s", d.Namespace, d.Name)
		if !isAcceptableDeployment(*d) {
			deletingDeployment := &appsv1.Deployment{}
			deletingDeployment.ObjectMeta = d.ObjectMeta
			log.Printf("Found! Illegal container image!")
			log.Printf("Deleting Deployment %s/%s", deletingDeployment.Namespace, deletingDeployment.Name)
			if err = r.Delete(context.TODO(), deletingDeployment); err != nil {
				log.Fatal(err)
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Fetch the Sloop instance
	instance := &policev1beta1.WhiteList{}
	err = r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// cleanup deleted Sloop
			log.Printf("Deployment or Sloop %s deleted ", request.NamespacedName)
			// TODO update all whiteList with "r.List func"
			delete(whiteList[request.Namespace], request.Name)
			return reconcile.Result{}, nil
		}
	}

	log.Printf("Watching CRD %s", request.NamespacedName)
	l := accesslist.NewWhiteList()
	for _, v := range instance.Spec.Images {
		for _, tag := range v.Tags {
			l.Add(v.Name, tag)
		}
	}
	if len(whiteList[request.Namespace]) == 0 {
		whiteList[request.Namespace] = make(map[string]*accesslist.WhiteList)
	}
	whiteList[request.Namespace][request.Name] = l

	deployments := &appsv1.DeploymentList{}
	listOptions := &client.ListOptions{Namespace: request.Namespace}
	err = r.List(context.TODO(), listOptions, deployments)
	if err == nil {
		for _, d := range deployments.Items {
			log.Printf("Checking Deployment %s/%s", d.Namespace, d.Name)
			if !isAcceptableDeployment(d) {
				deletingDeployment := &appsv1.Deployment{}
				deletingDeployment.ObjectMeta = d.ObjectMeta
				log.Printf("Found! Illegal container image!")
				log.Printf("Deleting Deployment %s/%s", deletingDeployment.Namespace, deletingDeployment.Name)
				if err = r.Delete(context.TODO(), deletingDeployment); err != nil {
					return reconcile.Result{}, err

				}
			}
		}
	} else {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func isAcceptableDeployment(deploy appsv1.Deployment) bool {
	// Ignore if namespace was not registered
	if _, ok := whiteList[deploy.Namespace]; !ok {
		return true
	}

	for _, c := range deploy.Spec.Template.Spec.Containers {
		imageName, tag := parseImageName(c.Image)
		log.Printf("Image %s:%s", imageName, tag)
		for _, l := range whiteList[deploy.Namespace] {
			if match := l.Search(imageName, tag); match != true {
				return false
			}
		}
	}
	return true
}

func parseImageName(imageName string) (string, string) {
	index := strings.LastIndex(imageName, ":")
	image := imageName
	tag := "latest"
	if index != -1 {
		image = imageName[:index]
		tag = imageName[index+1:]
	}
	return image, tag
}
