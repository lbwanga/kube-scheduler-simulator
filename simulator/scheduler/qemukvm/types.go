package qemukvm

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type QemuKvmArgs struct {
	metav1.TypeMeta `json:",inline"`

	QemuKvmScore    int64   `json:"qemuKvmScore,omitempty"`
	NonQemuKvmScore int64   `json:"nonQemuKvmScore,omitempty"`
	Weight          float64 `json:"weight,omitempty"`
}

func (a *QemuKvmArgs) DeepCopyObject() runtime.Object {
	if a == nil {
		return nil
	}
	out := new(QemuKvmArgs)
	*out = *a
	return out
}
