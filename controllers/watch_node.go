// @Time    : 2022/10/20 2:16 PM
// @Author  : HuYuan
// @File    : node.go
// @Email   : huyuan@virtaitech.com

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NodeResourceWatcher struct {
	log    logr.Logger
	client client.Client
	scheme *runtime.Scheme
}

func NewNodeResourceWatcher(client client.Client, scheme *runtime.Scheme) NodeResourceWatcher {
	return NodeResourceWatcher{
		log:    log.FromContext(context.TODO()),
		client: client,
		scheme: scheme,
	}
}

func (w NodeResourceWatcher) Create(event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	w.log.V(1).Info("Watcher to node resource Create action")
	queue.Add(reconcile.Request{})
}

func (w NodeResourceWatcher) Update(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	w.log.V(1).Info("Watcher to node resource Update action")
	queue.Add(reconcile.Request{})
}

func (w NodeResourceWatcher) Delete(event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	w.log.V(1).Info("Watcher to node resource Delete action")
	ctx := context.TODO()
	var podList v1.PodList

	selector := labels.NewSelector()
	matchOnceDaemonSetController, _ := labels.NewRequirement(DefaultOnceDaemonSetControllerUIDKey, selection.Exists, nil)

	label := selector.Add(*matchOnceDaemonSetController)
	options := &client.ListOptions{
		LabelSelector: label,
	}

	if err := w.client.List(ctx, &podList, options); err != nil {
		w.log.Error(err, "Failed to get the pod list")
		return
	}

	nodeName := event.Object.GetName()

	for _, item := range podList.Items {
		plog := w.log.WithValues("namespace", item.Namespace, "pod", item.Name, "action", "Delete Node", "node", nodeName)
		if item.Spec.NodeName == nodeName {
			if err := w.client.Delete(ctx, &item); err != nil {
				plog.Error(err, "Failed to delete pod")
			}
			plog.V(1).Info("Successfully deleted pod")
		}
	}
}

func (w NodeResourceWatcher) Generic(event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	w.log.V(1).Info("Watcher to node resource Generic action")
	queue.Add(reconcile.Request{})
}
