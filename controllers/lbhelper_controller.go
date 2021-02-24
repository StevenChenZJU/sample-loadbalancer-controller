/*


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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	//kbatch "k8s.io/api/batch/v1"
	//kapp "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LBHelperReconciler reconciles a LBHelper object
type LBHelperReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const desiredNamespace = "example-system"
const assignedLoadBalancerIP = "1.1.1.2"

// +kubebuilder:rbac:groups=webapp.my.domain,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.my.domain,resources=services/spec,verbs=get;update;patch

func (r *LBHelperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("guestbook v2.1", req.NamespacedName)
	log.Info("Reconcile called")
	var service corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Service deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to get Service")
		return ctrl.Result{}, err
	}
	isNamespaceDesired := func(service corev1.Service) bool {
		log.Info("Service.Namespace: " + service.Namespace)
		return service.Namespace == desiredNamespace
	}
	if !isNamespaceDesired(service) {
		log.Info("Service.Namespace not desired")
		return ctrl.Result{}, nil
	}

	isLoadBalancerService := func(service corev1.Service) bool {
		log.Info("Service Type: " + string(service.Spec.Type))
		if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
			log.Info("service.Spec.Type is LoadBalancer")
			return true
		}
		log.Info("service.Spec.Type is not LoadBalancer")
		return false
	}
	hasLoadBalancerIP := func(service corev1.Service) bool {
		log.Info("Service.LoadBalancerIP: [" + service.Spec.LoadBalancerIP + "]")
		if service.Spec.LoadBalancerIP == "" {
			log.Info("LoadBalancerIP is empty")
			return false
		}
		log.Info("LoadBalancerIP is not empty")
		return true
	}

	updateLoadBalancerIP := func(service corev1.Service) error {
		serviceCopy := service.DeepCopy()
		log.Info("Service IP updated from " + service.Spec.LoadBalancerIP)
		serviceCopy.Spec.LoadBalancerIP = assignedLoadBalancerIP
		if err := r.Update(ctx, serviceCopy); err != nil {
			log.Info("Error occurred in updateLoadBalancerIP")
			return err
		}
		return nil
	}

	if isLoadBalancerService(service) && !hasLoadBalancerIP(service) {
		log.Info("Find desired LoadBalancer Service to update IP")
		if err := updateLoadBalancerIP(service); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *LBHelperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldService, ok := e.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}
			newService, ok := e.ObjectNew.(*corev1.Service)
			if !ok {
				return false
			}
			if newService.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return false
			}
			if reflect.DeepEqual(newService.Spec, oldService.Spec) {
				return false
			}
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			service, ok := e.Object.(*corev1.Service)
			if !ok {
				return false
			}
			if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
				return false
			}
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}). // reconcile the Service instead of the CRD
		WithEventFilter(p).     // filter reconciling even
		Complete(r)
}
