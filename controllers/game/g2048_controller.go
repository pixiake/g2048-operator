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
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gamev1alpha1 "github.com/pixiake/g2048-operator/apis/game/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// G2048Reconciler reconciles a G2048 object
type G2048Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=game.example.com,resources=g2048s,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.example.com,resources=g2048s/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=game.example.com,resources=g2048s/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *G2048Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("g2048", req.NamespacedName)

	// Fetch the G2048 instance
	g2048 := &gamev1alpha1.G2048{}
	err := r.Get(ctx, req.NamespacedName, g2048)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("G2048 resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get G2048")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: g2048.Name, Namespace: g2048.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForG2048(g2048)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := g2048.Spec.Size
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the G2048 status with the pod names
	// List the pods for this g2048's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(g2048.Namespace),
		client.MatchingLabels(labelsForG2048(g2048.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "G2048.Namespace", g2048.Namespace, "G2048.Name", g2048.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Check if the service already exists, if not create a new one
	foundSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: g2048.Name, Namespace: g2048.Namespace}, foundSvc)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		svc := r.serviceForG2048(g2048)
		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, g2048.Status.Nodes) {
		g2048.Status.Nodes = podNames
		err := r.Status().Update(ctx, g2048)
		if err != nil {
			log.Error(err, "Failed to update G2048 status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *G2048Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gamev1alpha1.G2048{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// deploymentForG2048 returns a g2048 Deployment object
func (r *G2048Reconciler) deploymentForG2048(m *gamev1alpha1.G2048) *appsv1.Deployment {
	ls := labelsForG2048(m.Name)
	replicas := m.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "alexwhen/docker-2048",
						Name:  "g2048",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "g2048-web",
						}},
					}},
				},
			},
		},
	}
	// Set G2048 instance as the owner and controller
	_ = ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForG2048 returns the labels for selecting the resources
// belonging to the given g2048 CR name.
func labelsForG2048(name string) map[string]string {
	return map[string]string{"app": "g2048", "g2048_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// serviceForG2048 returns a g2048 Service object
func (r *G2048Reconciler) serviceForG2048(m *gamev1alpha1.G2048) *corev1.Service {
	ls := labelsForG2048(m.Name)
	svcType := m.Spec.ServiceType

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:     "g2048",
				Protocol: "TCP",
				Port:     80,
				TargetPort: intstr.IntOrString{
					StrVal: "80",
				},
			}},
			Selector: ls,
			Type:     corev1.ServiceType(svcType),
		},
	}
	// Set G2048 instance as the owner and controller
	_ = ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}
