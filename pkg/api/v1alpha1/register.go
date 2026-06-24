package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is the group/version for kube-booster CRDs.
var SchemeGroupVersion = schema.GroupVersion{Group: "kube-booster.io", Version: "v1alpha1"}

// SchemeBuilder registers the v1alpha1 types.
var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion, &WarmupConfig{}, &WarmupConfigList{})
	return nil
}

// Resource returns a GroupResource for the given resource name.
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
