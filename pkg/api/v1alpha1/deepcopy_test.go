package v1alpha1

import "testing"

func TestWarmupConfig_DeepCopy_isolatesSlices(t *testing.T) {
	orig := &WarmupConfig{
		Spec: WarmupConfigSpec{
			Steps: []WarmupStep{
				{Name: "s1", Requests: []WarmupRequest{{Endpoint: "/a"}}},
			},
		},
	}
	cp := orig.DeepCopy()

	cp.Spec.Steps[0].Name = "mutated"
	cp.Spec.Steps[0].Requests[0].Endpoint = "/mutated"

	if orig.Spec.Steps[0].Name != "s1" {
		t.Error("DeepCopy shared Steps slice with original")
	}
	if orig.Spec.Steps[0].Requests[0].Endpoint != "/a" {
		t.Error("DeepCopy shared Requests slice with original")
	}
}

func TestWarmupConfig_DeepCopy_nilSafe(t *testing.T) {
	var orig *WarmupConfig
	if orig.DeepCopy() != nil {
		t.Error("DeepCopy of nil WarmupConfig should return nil")
	}
}

func TestWarmupConfig_DeepCopy_nilSlices(t *testing.T) {
	orig := &WarmupConfig{Spec: WarmupConfigSpec{Steps: nil}}
	cp := orig.DeepCopy()
	if cp.Spec.Steps != nil {
		t.Error("DeepCopy of nil Steps should produce nil Steps, not empty slice")
	}
}

func TestWarmupConfigSpec_DeepCopyInto_mapsIsolated(t *testing.T) {
	orig := WarmupConfigSpec{
		Steps: []WarmupStep{
			{Requests: []WarmupRequest{
				{
					Headers: map[string]string{"Authorization": "Bearer original"},
					Extract: map[string]string{"token": "$.token"},
				},
			}},
		},
	}
	var cp WarmupConfigSpec
	orig.DeepCopyInto(&cp)

	cp.Steps[0].Requests[0].Headers["Authorization"] = "Bearer mutated"
	cp.Steps[0].Requests[0].Extract["token"] = "$.mutated"

	if orig.Steps[0].Requests[0].Headers["Authorization"] != "Bearer original" {
		t.Error("DeepCopyInto shared Headers map with original")
	}
	if orig.Steps[0].Requests[0].Extract["token"] != "$.token" {
		t.Error("DeepCopyInto shared Extract map with original")
	}
}
