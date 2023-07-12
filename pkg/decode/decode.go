package decode

import (
	"fmt"
	"math"
	"metric-collector/pkg/storage"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

func DecodeBatch(summary *stats.Summary) (*storage.MetricsBatch, error) {
	//fmt.Println("Func DecodeBatch Called")
	res := &storage.MetricsBatch{
		Node: storage.NodeMetricsPoint{},
		Pods: make([]storage.PodMetricsPoint, len(summary.Pods)),
	}

	var errs []error
	errs = append(errs, decodeNodeStats(&summary.Node, &res.Node)...)
	if len(errs) != 0 {
		// if we had errors providing node metrics, discard the data point
		// so that we don't incorrectly report metric values as zero.
	}
	//fmt.Println("Node Decoding Completed")

	num := 0
	for _, pod := range summary.Pods {
		podErrs := decodePodStats(&pod, &res.Pods[num])
		errs = append(errs, podErrs...)
		if len(podErrs) != 0 {
			// NB: we explicitly want to discard pods with partial results, since
			// the horizontal pod autoscaler takes special action when a pod is missing
			// metrics (and zero CPU or memory does not count as "missing metrics")

			// we don't care if we reuse slots in the result array,
			// because they get completely overwritten in decodePodStats
			continue
		}
		num++
	}
	res.Pods = res.Pods[:num]
	//fmt.Println("Pod Decoding Completed")
	//fmt.Println("=> CPUUsageNanoCores, MemoryUsageBytes, MemoryAvailableBytes, MemoryWorkingSetBytes, NetworkRxBytes, NetworkTxBytes, FsAvailableBytes, FsCapacityBytes, FsUsedBytes")

	return res, utilerrors.NewAggregate(errs)
}

func decodeNodeStats(nodeStats *stats.NodeStats, target *storage.NodeMetricsPoint) []error {
	//fmt.Println("Func decodeNodeStats Called")
	timestamp, err := getScrapeTimeNode(nodeStats.CPU, nodeStats.Memory, nodeStats.Network, nodeStats.Fs)
	if err != nil {
		// if we can't get a timestamp, assume bad data in general
		return []error{fmt.Errorf("unable to get valid timestamp for metric point for node %q, discarding data: %v", nodeStats.NodeName, err)}
	}

	*target = storage.NodeMetricsPoint{
		Name: nodeStats.NodeName,
		MetricsPoint: storage.MetricsPoint{
			Timestamp: timestamp,
		},
	}
	var errs []error
	if err := decodeCPU(&target.MetricsPoint, nodeStats.CPU); err != nil {
		errs = append(errs, fmt.Errorf("unable to get CPU for node %q, discarding data: %v", nodeStats.NodeName, err))
	}
	if err := decodeMemory(&target.MetricsPoint, nodeStats.Memory); err != nil {
		errs = append(errs, fmt.Errorf("unable to get memory for node %q, discarding data: %v", nodeStats.NodeName, err))
	}
	/*if err := decodeNetwork(&target.MetricsPoint, nodeStats.Network); err != nil {
		errs = append(errs, fmt.Errorf("unable to get Network for node %q, discarding data: %v", nodeStats.NodeName, err))
	}*/
	if err := decodeFs(&target.MetricsPoint, nodeStats.Fs); err != nil {
		errs = append(errs, fmt.Errorf("unable to get FS for node %q, discarding data: %v", nodeStats.NodeName, err))
	}

	return errs
}

func decodePodStats(podStats *stats.PodStats, target *storage.PodMetricsPoint) []error {
	//fmt.Println("Func decodePodStats Called")

	timestamp, err := getScrapeTimePod(podStats.CPU, podStats.Memory)
	if err != nil {
		// if we can't get a timestamp, assume bad data in general
		return []error{fmt.Errorf("unable to get valid timestamp for metric point for pod %q, discarding data: %v", podStats.PodRef.Name, err)}
	}

	// completely overwrite data in the target
	*target = storage.PodMetricsPoint{
		Name:      podStats.PodRef.Name,
		Namespace: podStats.PodRef.Namespace,
		MetricsPoint: storage.MetricsPoint{
			Timestamp: timestamp,
		},
		//Containers: make([]storage.ContainerMetricsPoint, len(podStats.Containers)),
	}

	var errs []error
	if err := decodeCPU(&target.MetricsPoint, podStats.CPU); err != nil {
		errs = append(errs, fmt.Errorf("unable to get CPU for pod %q, discarding data: %v", podStats.PodRef.Name, err))
	}
	if err := decodeMemory(&target.MetricsPoint, podStats.Memory); err != nil {
		errs = append(errs, fmt.Errorf("unable to get memory for pod %q, discarding data: %v", podStats.PodRef.Name, err))
	}
	/*if err := decodeNetwork(&target.MetricsPoint, podStats.Network); err != nil {
		errs = append(errs, fmt.Errorf("unable to get network RX for pod %q, discarding data: %v", podStats.PodRef.Name, err))
	}*/
	if err := decodeFs(&target.MetricsPoint, podStats.EphemeralStorage); err != nil {
		errs = append(errs, fmt.Errorf("unable to get Fs for pod %q, discarding data: %v", podStats.PodRef.Name, err))
	}

	return errs
}

func decodeCPU(target *storage.MetricsPoint, cpuStats *stats.CPUStats) error {
	//fmt.Println("Func decodeCPU Called")
	if cpuStats == nil || cpuStats.UsageNanoCores == nil {
		return fmt.Errorf("missing cpu usage metric")
	}

	target.CPUUsageNanoCores = *uint64Quantity(*cpuStats.UsageNanoCores, -9)
	return nil
}

func decodeMemory(target *storage.MetricsPoint, memStats *stats.MemoryStats) error {
	//fmt.Println("Func decodeMemory Called")
	if memStats == nil || memStats.WorkingSetBytes == nil {
		return fmt.Errorf("missing memory usage metric")
	}

	if memStats.AvailableBytes != nil {
		target.MemoryAvailableBytes = *uint64Quantity(*memStats.AvailableBytes, 0)
		target.MemoryAvailableBytes.Format = resource.BinarySI
	}
	target.MemoryUsageBytes = *uint64Quantity(*memStats.UsageBytes, 0)
	target.MemoryUsageBytes.Format = resource.BinarySI

	target.MemoryWorkingSetBytes = *uint64Quantity(*memStats.WorkingSetBytes, 0)
	target.MemoryWorkingSetBytes.Format = resource.BinarySI

	return nil
}
func decodeNetwork(target *storage.MetricsPoint, netStats *stats.NetworkStats) error {
	//fmt.Println("Func decodeNetwork Called")
	if netStats == nil || len(netStats.Interfaces) != 0 && netStats.Interfaces[0].RxBytes == nil {
		return fmt.Errorf("missing network RX usage metric")
	}
	var RX_Usage uint64 = 0
	var TX_Usage uint64 = 0
	for _, Interface := range netStats.Interfaces {
		RX_Usage = RX_Usage + *Interface.RxBytes
		TX_Usage = TX_Usage + *Interface.TxBytes
	}

	target.NetworkRxBytes = *uint64Quantity(RX_Usage, 0)
	target.NetworkRxBytes.Format = resource.BinarySI

	target.NetworkTxBytes = *uint64Quantity(TX_Usage, 0)
	target.NetworkTxBytes.Format = resource.BinarySI

	return nil
}

func decodeFs(target *storage.MetricsPoint, FsStats *stats.FsStats) error {
	//fmt.Println("Func decodeFs Called")
	if FsStats == nil {
		return fmt.Errorf("missing FS usage metric")
	}

	target.FsAvailableBytes = *uint64Quantity(*FsStats.AvailableBytes, 0)
	target.FsAvailableBytes.Format = resource.BinarySI

	target.FsCapacityBytes = *uint64Quantity(*FsStats.CapacityBytes, 0)
	target.FsCapacityBytes.Format = resource.BinarySI

	if FsStats.UsedBytes == nil {
		target.FsUsedBytes = *uint64Quantity(0, 0)
	} else {
		target.FsUsedBytes = *uint64Quantity(*FsStats.UsedBytes, 0)
	}

	target.FsUsedBytes.Format = resource.BinarySI

	return nil
}
func getScrapeTimePod(cpu *stats.CPUStats, memory *stats.MemoryStats) (time.Time, error) {
	//fmt.Println("Func getScrapeTimePod Called")
	// Ensure we get the earlier timestamp so that we can tell if a given data
	// point was tainted by pod initialization.

	var earliest *time.Time
	if cpu != nil && !cpu.Time.IsZero() && (earliest == nil || earliest.After(cpu.Time.Time)) {
		earliest = &cpu.Time.Time
	}

	if memory != nil && !memory.Time.IsZero() && (earliest == nil || earliest.After(memory.Time.Time)) {
		earliest = &memory.Time.Time
	}

	if earliest == nil {
		return time.Time{}, fmt.Errorf("no non-zero timestamp on either CPU or memory")
	}

	return *earliest, nil
}
func getScrapeTimeNode(cpu *stats.CPUStats, memory *stats.MemoryStats, network *stats.NetworkStats, fs *stats.FsStats) (time.Time, error) {
	//fmt.Println("Func getScrapeTimeNode Called")
	// Ensure we get the earlier timestamp so that we can tell if a given data
	// point was tainted by pod initialization.

	var earliest *time.Time
	if cpu != nil && !cpu.Time.IsZero() && (earliest == nil || earliest.After(cpu.Time.Time)) {
		earliest = &cpu.Time.Time
	}

	if memory != nil && !memory.Time.IsZero() && (earliest == nil || earliest.After(memory.Time.Time)) {
		earliest = &memory.Time.Time
	}

	if network != nil && !network.Time.IsZero() && (earliest == nil || earliest.After(network.Time.Time)) {
		earliest = &network.Time.Time
	}

	if fs != nil && !fs.Time.IsZero() && (earliest == nil || earliest.After(fs.Time.Time)) {
		earliest = &fs.Time.Time
	}

	if earliest == nil {
		return time.Time{}, fmt.Errorf("no non-zero timestamp on either CPU or memory")
	}

	return *earliest, nil
}

// uint64Quantity converts a uint64 into a Quantity, which only has constructors
// that work with int64 (except for parse, which requires costly round-trips to string).
// We lose precision until we fit in an int64 if greater than the max int64 value.
func uint64Quantity(val uint64, scale resource.Scale) *resource.Quantity {
	// easy path -- we can safely fit val into an int64
	if val <= math.MaxInt64 {
		return resource.NewScaledQuantity(int64(val), scale)
	}

	//fmt.Println("unexpectedly large resource value %v, loosing precision to fit in scaled resource.Quantity", val)

	// otherwise, lose an decimal order-of-magnitude precision,
	// so we can fit into a scaled quantity
	return resource.NewScaledQuantity(int64(val/10), resource.Scale(1)+scale)
}
