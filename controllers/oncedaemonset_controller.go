/*
Copyright 2022.

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
	"errors"
	"sync"
	"time"

	appsv1 "k8s-once-daemonset/api/v1"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OnceDaemonSetReconciler reconciles a OnceDaemonSet object
type OnceDaemonSetReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	log         logr.Logger
	nodeWatcher *NodeResourceWatcher
	lock        *sync.Map
}

func NewReconciler(mgr manager.Manager) *OnceDaemonSetReconciler {
	client := mgr.GetClient()
	scheme := mgr.GetScheme()
	nodeWatcher := NewNodeResourceWatcher(client, scheme)
	return &OnceDaemonSetReconciler{
		Client:      client,
		Scheme:      scheme,
		nodeWatcher: &nodeWatcher,
		lock:        new(sync.Map),
	}
}

func (r *OnceDaemonSetReconciler) getLock(uid string) {
	for {
		var t string
		if _, ok := r.lock.LoadOrStore(uid, t); ok {
			r.log.V(1).Info("waiting for lock", "uid", uid)
			time.Sleep(time.Second * 3)
			continue
		}
		r.log.V(1).Info("get a lock", "uid", uid)
		break
	}
}

// releaseLock 释放锁操作, 如果释放锁速度过快可能出现 pod 还没有创建出来就触发了第二次调协导致一个node上创建了属于一个 OnceDaemonSet 控制器的多个 pod
func (r *OnceDaemonSetReconciler) releaseLock(uid string) {
	time.Sleep(time.Second * 5)
	r.lock.Delete(uid)
}

//+kubebuilder:rbac:groups=apps.io.huyuan,resources=oncedaemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.io.huyuan,resources=oncedaemonsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.io.huyuan,resources=oncedaemonsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pod,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pod/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pod/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=node,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=node/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=node/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OnceDaemonSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *OnceDaemonSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	if req.Namespace == "" {
		return r.reconcileNode(ctx, req)
	}
	return r.reconcileObject(ctx, req)
}

func (r *OnceDaemonSetReconciler) handle(ctx context.Context, onceDaemonSet appsv1.OnceDaemonSet, log logr.Logger) (ctrl.Result, error) {
	var nodes v1.NodeList

	if err := r.Client.List(ctx, &nodes); err != nil {
		log.Error(err, "unable to get node list")
		return ctrl.Result{}, err
	}

	log.V(1).Info("Successfully obtained the node list", "count", len(nodes.Items))

	for _, node := range nodes.Items {
		if shouldRun, _ := nodeShouldRunOnceDaemonPod(&node, &onceDaemonSet); !shouldRun {
			log.V(1).Info("node should not run OnceDaemonSet Pod", "node", node.Name)
			continue
		}

		podController := NewPodController(r.Client, onceDaemonSet, node, r.Scheme, log)

		if err := podController.CreateOrUpdate(); err != nil {
			if err == ErrUnsupportedRestartPolicy {
				log.Error(err, "unsupported restartPolicy")
				return ctrl.Result{}, nil
			}
			log.Error(err, "unknown error")
		}
	}
	return ctrl.Result{}, nil
}

func (r *OnceDaemonSetReconciler) reconcileNode(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("event", "node")
	var onceDaemonSetList appsv1.OnceDaemonSetList

	if err := r.Client.List(ctx, &onceDaemonSetList); err != nil {
		log.Error(err, "unable to get OnceDaemonSet list")
		return ctrl.Result{}, nil
	}

	var wg sync.WaitGroup
	for _, onceDaemonSet := range onceDaemonSetList.Items {
		wg.Add(1)
		go func(ods appsv1.OnceDaemonSet) {
			defer wg.Done()
			uid := string(ods.GetUID())
			r.getLock(uid)
			defer r.releaseLock(uid)
			r.handle(ctx, ods, log)
		}(onceDaemonSet)
	}

	wg.Wait()
	return ctrl.Result{}, nil
}

func (r *OnceDaemonSetReconciler) reconcileObject(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("event", "object")
	onceDaemonSet := appsv1.OnceDaemonSet{}
	err := r.Client.Get(ctx, req.NamespacedName, &onceDaemonSet)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "reconciling OnceDaemonSet resource")
		return ctrl.Result{Requeue: true}, nil
	}

	if onceDaemonSet.DeletionTimestamp != nil {
		log.Error(errors.New("reconcile object is being deleted"), "deletionTimestamp is not nil")
		return ctrl.Result{}, nil
	}

	uid := string(onceDaemonSet.GetUID())
	r.getLock(uid)
	defer r.releaseLock(uid)
	return r.handle(ctx, onceDaemonSet, log)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OnceDaemonSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.OnceDaemonSet{}).
		Watches(&source.Kind{Type: &v1.Node{}}, r.nodeWatcher).
		Complete(r)
}
