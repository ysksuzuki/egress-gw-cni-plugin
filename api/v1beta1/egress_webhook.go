/*
Copyright 2023.

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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager setups the webhook for Egress
func (r *Egress) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-egress-ysksuzuki-com-v1beta1-egress,mutating=true,failurePolicy=fail,sideEffects=None,groups=egress.ysksuzuki.com,resources=egresses,verbs=create;update,versions=v1beta1,name=megress.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Egress{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Egress) Default() {
	tmpl := r.Spec.Template
	if tmpl == nil {
		return
	}

	if len(tmpl.Spec.Containers) == 0 {
		tmpl.Spec.Containers = []corev1.Container{
			{
				Name: "egress-gw",
			},
		}
	}
}

// +kubebuilder:webhook:path=/validate-egress-ysksuzuki-com-v1beta1-egress,mutating=false,failurePolicy=fail,sideEffects=None,groups=egress.ysksuzuki.com,resources=egresses,verbs=create;update,versions=v1beta1,name=vegress.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Egress{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Egress) ValidateCreate() (warnings admission.Warnings, err error) {
	errs := r.Spec.validate()
	if len(errs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "Egress"}, r.Name, errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Egress) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {
	errs := r.Spec.validateUpdate(old.(*Egress).Spec)
	if len(errs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "Egress"}, r.Name, errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Egress) ValidateDelete() (warnings admission.Warnings, err error) {
	return nil, nil
}
