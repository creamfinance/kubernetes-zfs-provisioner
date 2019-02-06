
// Modifications copyright (C) 2019 Cream Finance IT Austria GmbH
package provisioner

import "strconv"

import "github.com/prometheus/client_golang/prometheus"
import zfs "github.com/simt2/go-zfs"
// import log "github.com/Sirupsen/logrus"
// import "k8s.io/api/core/v1"

// ZFSProvisioner implements the Provisioner interface to create and export ZFS volumes
type ZFSProvisioner struct {
	persistentVolumeCapacity *prometheus.Desc
	persistentVolumeUsed     *prometheus.Desc
}

/*

	parent: parent,

		shareOptions:   shareOptions,
		serverHostname: serverHostname,
		reclaimPolicy:  kubernetesReclaimPolicy,

		persistentVolumeCapacity: prometheus.NewDesc(
			"zfs_provisioner_persistent_volume_capacity",
			"Capacity of a zfs persistent volume.",
			[]string{"persistent_volume"},
			prometheus.Labels{
				"parent":   "Dataset", // parent.Name
				"hostname": serverHostname,
			},
		),
		persistentVolumeUsed: prometheus.NewDesc(
			"zfs_provisioner_persistent_volume_used",
			"Usage of a zfs persistent volume.",
			[]string{"persistent_volume"},
			prometheus.Labels{
				"parent":   "Dataset", // parent.Name
				"hostname": serverHostname,
			},
		),
*/

// Describe implements prometheus.Collector
func (p ZFSProvisioner) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.persistentVolumeCapacity
	ch <- p.persistentVolumeUsed
}

// Collect implements prometheus.Collector
func (p ZFSProvisioner) Collect(ch chan<- prometheus.Metric) {
	return;

	/*children, err := p.parent.Children(1)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("Collecting metrics failed")
	}

	for _, child := range children {
		// Skip shapshots
		if child.Type != "filesystem" {
			continue
		}

		capacity, used, err := p.datasetMetrics(child)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Collecting metrics failed")
		} else {
			ch <- *capacity
			ch <- *used
		}
	}
	*/
}

// NewZFSProvisioner returns a new ZFSProvisioner
func NewZFSProvisioner() ZFSProvisioner {
	return ZFSProvisioner{
	}
}

// datasetMetrics returns prometheus metrics for a given ZFS dataset
func (p ZFSProvisioner) datasetMetrics(dataset *zfs.Dataset) (*prometheus.Metric, *prometheus.Metric, error) {
	capacityString, err := dataset.GetProperty("refquota")
	if err != nil {
		return nil, nil, err
	}
	capacityInt, _ := strconv.Atoi(capacityString)

	usedString, err := dataset.GetProperty("usedbydataset")
	if err != nil {
		return nil, nil, err
	}
	usedInt, _ := strconv.Atoi(usedString)

	capacity := prometheus.MustNewConstMetric(p.persistentVolumeCapacity, prometheus.GaugeValue, float64(capacityInt), dataset.Name)
	used := prometheus.MustNewConstMetric(p.persistentVolumeUsed, prometheus.GaugeValue, float64(usedInt), dataset.Name)

	return &capacity, &used, nil
}
