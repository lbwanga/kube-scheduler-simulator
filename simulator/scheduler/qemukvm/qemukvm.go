package qemukvm

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"math"
)

const (
	// Name 是插件的名称
	Name = "QemuKvmAnnotation"
	// QemuKvmAnnotation 是用于标识 QEMU/KVM 节点的注解
	QemuKvmAnnotation = "qemukvm"
	// QemuKvmAnnotationValue 是注解的值
	QemuKvmAnnotationValue = "true"
)

// QemuKvmScheduler 是一个自定义调度器插件
type QemuKvmScheduler struct {
	handle framework.Handle

	// QemuKvmScore 是 QEMU/KVM 节点的额外分数
	qemuKvmScore int64

	// NonQemuKvmScore 是非 QEMU/KVM 节点的减分
	nonQemuKvmScore int64

	// ResourceScoreWeight 是资源评分的权重
	weight float64
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type QemuKvmArgs struct {
	metav1.TypeMeta `json:",inline"`

	QemuKvmScore    int64   `json:"qemuKvmScore"`
	NonQemuKvmScore int64   `json:"nonQemuKvmScore"`
	Weight          float64 `json:"weight"`
}

// Name 返回插件名称
func (s *QemuKvmScheduler) Name() string {
	return Name
}

// Filter 实现过滤函数
func (s *QemuKvmScheduler) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	klog.V(3).InfoS("Running Filter for pod", "pod", klog.KObj(pod), "node", nodeInfo.Node().Name)
	return nil
}

// Score 实现评分函数
func (s *QemuKvmScheduler) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	klog.V(3).InfoS("Running Score for pod", "pod", klog.KObj(pod), "node", nodeName)

	nodeInfo, err := s.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		klog.ErrorS(err, "Failed to get node info", "node", nodeName)
		return 0, framework.AsStatus(fmt.Errorf("getting node %q from Snapshot: %w", nodeName, err))
	}

	node := nodeInfo.Node()
	if node == nil {
		klog.ErrorS(nil, "Node is nil", "node", nodeName)
		return 0, framework.NewStatus(framework.Error, "node is nil")
	}

	// 计算资源分数
	resourceScore := calculateResourceScore(nodeInfo, pod)
	klog.V(3).InfoS("Resource score calculated", "node", nodeName, "score", resourceScore)

	// 计算注解分数
	annotationScore := s.calculateAnnotationScore(node)
	klog.V(3).InfoS("Annotation score calculated", "node", nodeName, "score", annotationScore)

	// 合并分数
	mergeScore := int64(float64(resourceScore)*s.weight) + annotationScore

	finalScore := normalizeScore(int64(mergeScore), framework.MaxNodeScore, framework.MinNodeScore)
	klog.V(3).InfoS("Final score calculated", "node", nodeName, "score", finalScore)

	return finalScore, nil
}

// calculateResourceScore 计算资源分数
func calculateResourceScore(nodeInfo *framework.NodeInfo, pod *v1.Pod) int64 {
	// 获取节点的可分配资源
	allocatable := nodeInfo.Allocatable
	// 获取节点的已用资源
	used := nodeInfo.Requested

	// 计算 CPU 分数
	cpuScore := calculateCPUScore(allocatable.MilliCPU, used.MilliCPU, pod)
	// 计算内存分数
	memoryScore := calculateMemoryScore(allocatable.Memory, used.Memory, pod)

	// 返回 CPU 和内存分数的平均值
	return (cpuScore + memoryScore) / 2
}

// calculateCPUScore 计算 CPU 分数
func calculateCPUScore(allocatableCPU, usedCPU int64, pod *v1.Pod) int64 {
	// 计算 Pod 的 CPU 请求
	podCPU := int64(0)
	for _, container := range pod.Spec.Containers {
		podCPU += container.Resources.Requests.Cpu().MilliValue()
	}

	// 计算 CPU 使用率
	cpuUsage := float64(usedCPU) / float64(allocatableCPU)
	// 计算剩余 CPU 比例
	remainingCPU := 1.0 - cpuUsage

	// 使用对数函数计算分数，使得分数分布更均匀
	score := int64(math.Log2(remainingCPU*100+1) * 10)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// calculateMemoryScore 计算内存分数
func calculateMemoryScore(allocatableMemory, usedMemory int64, pod *v1.Pod) int64 {
	// 计算 Pod 的内存请求
	podMemory := int64(0)
	for _, container := range pod.Spec.Containers {
		podMemory += container.Resources.Requests.Memory().Value()
	}

	// 计算内存使用率
	memoryUsage := float64(usedMemory) / float64(allocatableMemory)
	// 计算剩余内存比例
	remainingMemory := 1.0 - memoryUsage

	// 使用对数函数计算分数，使得分数分布更均匀
	score := int64(math.Log2(remainingMemory*100+1) * 10)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// calculateAnnotationScore 计算注解分数
func (q *QemuKvmScheduler) calculateAnnotationScore(node *v1.Node) int64 {
	if value, exists := node.Annotations[QemuKvmAnnotation]; exists {
		if value == QemuKvmAnnotationValue {
			return q.qemuKvmScore
		}
		return q.nonQemuKvmScore
	}
	return q.nonQemuKvmScore
}

// ScoreExtensions 返回 ScoreExtensions 接口
func (q *QemuKvmScheduler) ScoreExtensions() framework.ScoreExtensions {
	return q
}

// NormalizeScore 实现分数标准化
func (s *QemuKvmScheduler) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	klog.V(3).InfoS("Running NormalizeScore for pod", "pod", klog.KObj(pod))
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}

	return nil
}

// normalizaScore nornalize the score in range [min, max]
func normalizeScore(value, max, min int64) int64 {
	if value < min {
		value = min
	}

	if value > max {
		value = max
	}

	return value
}

func (a *QemuKvmArgs) DeepCopyObject() runtime.Object {
	if a == nil {
		return nil
	}
	out := new(QemuKvmArgs)
	*out = *a
	return out
}

// New 创建插件实例
func New(_ context.Context, obj runtime.Object, h framework.Handle) (framework.Plugin, error) {
	klog.V(3).InfoS("Creating new QemuKvmScheduler plugin")

	args, ok := obj.(*QemuKvmArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments, expected QemuKvmArgs")
	}
	return &QemuKvmScheduler{
		handle:          h,
		qemuKvmScore:    args.QemuKvmScore,
		nonQemuKvmScore: args.NonQemuKvmScore,
		weight:          args.Weight,
	}, nil
}
