package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupVersion = schema.GroupVersion{Group: "plexus.io", Version: "v1beta1"}

	SchemeBuilder = runtime.NewSchemeBuilder()

	AddToScheme = SchemeBuilder.AddToScheme
)
