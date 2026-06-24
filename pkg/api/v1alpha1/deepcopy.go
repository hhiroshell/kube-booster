package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyObject implements runtime.Object.
func (in *WarmupConfig) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(WarmupConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another WarmupConfig.
func (in *WarmupConfig) DeepCopyInto(out *WarmupConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy returns a deep copy of WarmupConfig.
func (in *WarmupConfig) DeepCopy() *WarmupConfig {
	if in == nil {
		return nil
	}
	out := new(WarmupConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *WarmupConfigList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(WarmupConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties into another WarmupConfigList.
func (in *WarmupConfigList) DeepCopyInto(out *WarmupConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WarmupConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyInto copies all properties into another WarmupConfigSpec.
func (in *WarmupConfigSpec) DeepCopyInto(out *WarmupConfigSpec) {
	*out = *in
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]WarmupStep, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyInto copies all properties into another WarmupStep.
func (in *WarmupStep) DeepCopyInto(out *WarmupStep) {
	*out = *in
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = make([]WarmupRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopyInto copies all properties into another WarmupRequest.
func (in *WarmupRequest) DeepCopyInto(out *WarmupRequest) {
	*out = *in
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Extract != nil {
		in, out := &in.Extract, &out.Extract
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}
