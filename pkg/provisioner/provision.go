// Modifications copyright (C) 2019 Cream Finance IT Austria GmbH
package provisioner

import (
	"fmt"
	"strconv"
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	zfs "github.com/simt2/go-zfs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
)

const (
	annDataset = "creamfinance.com/zfs-dataset"
)

// Provision creates a PersistentVolume, sets quota and shares it via NFS.
func (p ZFSProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {

	log.WithFields(log.Fields{
		"data": options,
	}).Info("Volume Options")


	/*
	options := VolumeOptions{
		PersistentVolumeReclaimPolicy: reclaimPolicy,
		PVName:                        pvName,
		PVC:                           claim,
		MountOptions:                  mountOptions,
		Parameters:                    parameters,
		SelectedNode:                  selectedNode,
		AllowedTopologies:             allowedTopologies,
	}


	Interesting options:
		PersistentVolumeReclaimPolicy
		MountOptions
		Parameters

	*/

	serverHostname, ok := options.Parameters["serverHostname"]

	if !ok {
		return nil, errors.New("Missing parameter serverHostname in storageClass")
	}

	zfsPath, path, err := p.createVolume(options)

	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"volume": path,
	}).Info("Created volume")

	annotations := make(map[string]string)
	annotations[annDataset] = zfsPath

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        options.PVName,
			Labels:      map[string]string{},
			Annotations: annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server:   serverHostname,
					Path:     path,
					ReadOnly: false,
				},
			},
		},
	}

	log.Debug("Returning pv:")
	log.Debug(*pv)

	return pv, nil
}

// createVolume creates a ZFS dataset and returns its mount path
func (p ZFSProvisioner) createVolume(options controller.VolumeOptions) (string, string, error) {
	// retrieve the parent zfs name
	parentDataset, ok := options.Parameters["parentDataset"];

	if !ok {
		return "", "", errors.New("Missing parameter parentDataset in storageClass")
	}

	// validate the parent set name? (no leading or trailing /)
	if !(len(parentDataset) > 2 &&
		parentDataset[0] != '/' &&
		parentDataset[len(parentDataset) - 1] != '/') {
		return "", "", fmt.Errorf("Invalid value for parentDataset %s in storageClass", parentDataset)
	}

	// retrieve the shareOptions
	shareOptions, ok := options.Parameters["shareOptions"]

	if !ok {
		return "", "", errors.New("Missing parameter shareOptions in storageClass")
	}

	// retrieve the overProvision setting
	overProvision := false
	overProvision_str, ok := options.Parameters["overProvision"]

	if ok {
		switch overProvision_str {
		case "true":
			overProvision = true
		case "false":
			overProvision = false
		default:
			return "", "", fmt.Errorf("Invalid value for parameter overProvision (true/false) got %s in storageClass", overProvision_str)
		}
	}

	zfsPath := parentDataset + "/" + options.PVName
	properties := make(map[string]string)

	properties["sharenfs"] = shareOptions

	storageRequest := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	storageRequestBytes := strconv.FormatInt(storageRequest.Value(), 10)
	properties["refquota"] = storageRequestBytes

	if !overProvision {
		properties["refreservation"] = storageRequestBytes
	}

	dataset, err := zfs.CreateFilesystem(zfsPath, properties)

	if err != nil {
		return "", "", fmt.Errorf("Creating ZFS dataset failed with: %v", err.Error())
	}

	return zfsPath, dataset.Mountpoint, nil
}
