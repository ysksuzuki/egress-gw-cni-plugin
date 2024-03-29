package controllers

import (
	"context"
	egressv1beta1 "github.com/ysksuzuki/egress-gw-cni-plugin/api/v1beta1"
	"github.com/ysksuzuki/egress-gw-cni-plugin/pkg/constants"
	"sort"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=egress.ysksuzuki.com,resources=egresses,verbs=get;list;watch

// SetupCRBReconciler setups ClusterResourceBinding reconciler for egress-controller.
func SetupCRBReconciler(mgr manager.Manager) error {
	r := &crbReconciler{
		Client: mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rbacv1.ClusterRoleBinding{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			switch object.GetName() {
			case constants.CRBEgress:
				return true
			}
			return false
		})).
		Complete(r)
}

type crbReconciler struct {
	client.Client
}

func (r *crbReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	switch req.Name {
	case constants.CRBEgress:
	default:
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)

	if err := reconcileCRB(ctx, r.Client, logger, req.Name); err != nil {
		logger.Error(err, "failed to reconcile cluster role binding")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func reconcileCRB(ctx context.Context, cl client.Client, log logr.Logger, name string) error {
	crb := &rbacv1.ClusterRoleBinding{}
	if err := cl.Get(ctx, client.ObjectKey{Name: name}, crb); err != nil {
		return err
	}

	egresses := &egressv1beta1.EgressList{}
	if err := cl.List(ctx, egresses); err != nil {
		return err
	}

	nsMap := make(map[string]struct{})
	for _, eg := range egresses.Items {
		nsMap[eg.Namespace] = struct{}{}
	}

	namespaces := make([]string, 0, len(nsMap))
	for k := range nsMap {
		namespaces = append(namespaces, k)
	}
	sort.Strings(namespaces)

	subjects := make([]rbacv1.Subject, len(namespaces))
	for i, n := range namespaces {
		subjects[i] = rbacv1.Subject{
			APIGroup:  "",
			Kind:      "ServiceAccount",
			Name:      constants.SAEgress,
			Namespace: n,
		}
	}

	if equality.Semantic.DeepDerivative(subjects, crb.Subjects) {
		return nil
	}

	log.Info("updating cluster role binding")
	crb.Subjects = subjects
	return cl.Update(ctx, crb)
}
