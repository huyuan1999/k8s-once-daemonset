// @Time    : 2022/10/20 11:24 AM
// @Author  : HuYuan
// @File    : pods.go
// @Email   : huyuan@virtaitech.com

package controllers

import (
	"context"
	"errors"
	"fmt"
	v12 "k8s-once-daemonset/api/v1"
	"reflect"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DefaultOnceDaemonSetControllerUIDKey  = "once-daemonset-controller-uid"
	DefaultOnceDaemonSetControllerNameKey = "once-daemonset-controller-name"
	LastAppliedConfigurationKey           = "kubectl.kubernetes.io/last-applied-configuration"
)

var ErrUnsupportedRestartPolicy = errors.New("restartPolicy: Unsupported value: Always: supported values: OnFailure, Never")

type PodController struct {
	client        client.Client
	onceDaemonSet v12.OnceDaemonSet
	node          v1.Node
	scheme        *runtime.Scheme
	log           logr.Logger
	ctx           context.Context
}

func NewPodController(client client.Client, onceDaemonSet v12.OnceDaemonSet, node v1.Node, scheme *runtime.Scheme, log logr.Logger) *PodController {
	return &PodController{
		client:        client,
		node:          node,
		onceDaemonSet: onceDaemonSet,
		scheme:        scheme,
		log:           log,
		ctx:           context.TODO(),
	}
}

func (p *PodController) getListOptions() *client.ListOptions {
	selector := labels.NewSelector()
	matchControllerUID, _ := labels.NewRequirement(DefaultOnceDaemonSetControllerUIDKey, selection.DoubleEquals, []string{string(p.onceDaemonSet.UID)})
	matchControllerName, _ := labels.NewRequirement(DefaultOnceDaemonSetControllerNameKey, selection.DoubleEquals, []string{p.onceDaemonSet.Name})
	label := selector.Add(*matchControllerUID, *matchControllerName)
	return &client.ListOptions{
		LabelSelector: label,
		Namespace:     p.onceDaemonSet.Namespace,
	}
}

func (p *PodController) setPodSpec(pod *v1.PodTemplateSpec, nodeName string) *v1.Pod {
	pod.Spec.Affinity = ReplaceDaemonSetPodNodeNameNodeAffinity(pod.Spec.Affinity, nodeName)

	pod.Name = fmt.Sprintf("%s-%s", p.onceDaemonSet.Name, randStr(5))
	pod.Namespace = p.onceDaemonSet.Namespace
	var podLabels map[string]string
	if pod.Labels == nil {
		podLabels = make(map[string]string)
	} else {
		podLabels = pod.Labels
	}

	podLabels[DefaultOnceDaemonSetControllerUIDKey] = string(p.onceDaemonSet.UID)
	podLabels[DefaultOnceDaemonSetControllerNameKey] = p.onceDaemonSet.Name
	pod.Labels = podLabels

	spec := &v1.Pod{
		ObjectMeta: pod.ObjectMeta,
		Spec:       pod.Spec,
	}
	return spec
}

func (p *PodController) create(spec *v1.Pod) {
	if err := p.client.Create(p.ctx, spec); err != nil {
		p.log.Error(err, "Failed to create pod", "onceDaemonSet", p.onceDaemonSet.Name)
		return
	}
	p.log.V(1).Info("Created pod successfully", "onceDaemonSet", p.onceDaemonSet.Name)
}

func (p *PodController) CreateOrUpdate() error {
	if p.onceDaemonSet.Spec.RestartPolicy == v1.RestartPolicyAlways {
		return ErrUnsupportedRestartPolicy
	}

	pts := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      p.onceDaemonSet.Labels,
			Annotations: p.onceDaemonSet.Annotations,
		},
		Spec: p.onceDaemonSet.Spec,
	}

	template := CreatePodTemplate(pts)

	var oldPodList v1.PodList

	options := p.getListOptions()

	p.log = p.log.WithValues("namespace", p.onceDaemonSet.Namespace, "OnceDaemonSet", p.onceDaemonSet.Name)

	pod := template.DeepCopy()
	curPod := p.setPodSpec(pod, p.node.Name)

	if err := controllerutil.SetControllerReference(&p.onceDaemonSet, curPod, p.scheme); err != nil {
		p.log.Error(err, "SetControllerReference", "node", p.node, "onceDaemonSet", p.onceDaemonSet.Name)
		return err
	}

	if err := p.client.List(p.ctx, &oldPodList, options); err != nil || len(oldPodList.Items) == 0 {
		p.create(curPod)
		return nil
	}

	oldPod := func() *v1.Pod {
		for _, item := range oldPodList.Items {
			if item.Spec.NodeName == p.node.Name {
				return &item
			}
		}
		return nil
	}()

	if oldPod == nil {
		p.create(curPod)
		return nil
	}

	curControllerRef := metav1.GetControllerOf(curPod)
	oldControllerRef := metav1.GetControllerOf(oldPod)

	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	labelChanged := !reflect.DeepEqual(curPod.Labels, oldPod.Labels)

	if controllerRefChanged || labelChanged {
		p.client.Delete(p.ctx, oldPod)
		p.create(curPod)
		return nil
	}

	curAnnotations := curPod.GetAnnotations()
	oldAnnotations := oldPod.GetAnnotations()

	curConfiguration := curAnnotations[LastAppliedConfigurationKey]
	oldConfiguration := oldAnnotations[LastAppliedConfigurationKey]

	if curConfiguration != oldConfiguration {
		p.client.Delete(p.ctx, oldPod)
		p.create(curPod)
		return nil
	}

	// p.onceDaemonSet.Status.DesiredNumberScheduled++

	// return p.client.Status().Update(p.ctx, &p.onceDaemonSet)
	return nil
}
