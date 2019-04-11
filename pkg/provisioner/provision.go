// Modifications copyright (C) 2019 Cream Finance IT Austria GmbH
package provisioner

import (
	"fmt"
	"strconv"
	"errors"
	"strings"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	zfs "github.com/simt2/go-zfs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/core/v1"
)

const (
	// Volume: Dataset path on the volume
	annDataset = "creamfinance.com/zfs-dataset"

	// Volume: Snapshot name on the volume
	annSnapshot = "creamfinance.com/zfs-snapshot"

	// PVC: Owner for the new volume
	annOwner = "creamfinance.com/zfs-owner"

	// PVC: Source volume for a clone
	annClone = "creamfinance.com/zfs-clone"
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

	annotations := make(map[string]string)

	path, err := p.createVolume(options, annotations)

	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"volume": path,
	}).Info("Created volume")

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
func (p ZFSProvisioner) createVolume(options controller.VolumeOptions, annotations map[string]string) (string, error) {
	// retrieve the parent zfs name
	parentDataset, ok := options.Parameters["parentDataset"];

	if !ok {
		return "", errors.New("Missing parameter parentDataset in storageClass")
	}

	// validate the parent set name? (no leading or trailing /)
	if !(len(parentDataset) > 2 &&
		parentDataset[0] != '/' &&
		parentDataset[len(parentDataset) - 1] != '/') {
		return "", fmt.Errorf("Invalid value for parentDataset %s in storageClass", parentDataset)
	}

	// retrieve the shareOptions
	shareOptions, ok := options.Parameters["shareOptions"]

	if !ok {
		return "", errors.New("Missing parameter shareOptions in storageClass")
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
			return "", fmt.Errorf("Invalid value for parameter overProvision (true/false) got %s in storageClass", overProvision_str)
		}
	}

	zfsPath := parentDataset + "/" + options.PVName
	properties := make(map[string]string)

	properties["sharenfs"] = shareOptions

	storageRequest := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	storageRequestBytes := strconv.FormatInt(storageRequest.Value(), 10)

	/* Handle Cloning of Dataset */
	zfsBaseDataset, cloneOk := options.PVC.ObjectMeta.Annotations[annClone]

	var dataset *zfs.Dataset
	var err error

	if cloneOk {
		// Retrieve our base dataset
		baseDataset, err := zfs.GetDataset(zfsBaseDataset)

		if err != nil {
			// it's probably already deleted here, just log
			return "", fmt.Errorf("Unable to get dataset for snapshot: %v", err)
		}

		// test if the snapshot already exists:
		snapshot, err := zfs.GetDataset(baseDataset.Name + "@" + options.PVName)

		if snapshot == nil {
			// Create a snapshot of base
			snapshot, err = baseDataset.Snapshot(options.PVName, false)

			if err != nil {
				return "", fmt.Errorf("Creating ZFS Snapshot failed with: %v", err.Error())
			}
		}

		annotations[annSnapshot] = snapshot.Name

		// Create the clone
		dataset, err = snapshot.Clone(zfsPath, properties)

		if err != nil {
			return "", fmt.Errorf("Creating ZFS Clone failed with: %v", err.Error())
		}
	} else {
		// only setup reservation if we actually create a new volume
		properties["refquota"] = storageRequestBytes

		if !overProvision {
			properties["refreservation"] = storageRequestBytes
		}

		// create the new volume
		dataset, err = zfs.CreateFilesystem(zfsPath, properties)

		if err != nil {
			return "", fmt.Errorf("Creating ZFS dataset failed with: %v", err.Error())
		}
	}

	// Set ownership
	owner, ok := options.Parameters["owner"];

	// check if the pvc has a special annotation
	pvc_owner, pvc_ok := options.PVC.ObjectMeta.Annotations[annOwner]

	if pvc_ok {
		owner = pvc_owner
		ok = true
	}

	if ok {
		// Split owner
		ids := strings.Split(owner, ":")

		if len(ids) == 2 {
			uid, err := strconv.Atoi(ids[0])

			if err != nil {
				return "", fmt.Errorf("Error setting ownership: %s", err.Error())
			}

			gid, err := strconv.Atoi(ids[1])

			if err != nil {
				return "", fmt.Errorf("Error setting ownership: %s", err.Error())
			}

			err = os.Chown(dataset.Mountpoint, uid, gid)

			if err != nil {
				return "", fmt.Errorf("Error setting ownership: %s", err.Error())
			} else {
				fmt.Printf("Updated ownership of %s to %d %d\n", dataset.Mountpoint, uid, gid)
			}
		} else if len(ids) == 1 {
			uid, err := strconv.Atoi(ids[0])

			if err != nil {
				return "", fmt.Errorf("Error setting ownership: %s", err.Error())
			}

			err = os.Chown(dataset.Mountpoint, uid, -1)

			if err != nil {
				return "", fmt.Errorf("Error setting ownership: %s", err.Error())
			} else {
				fmt.Printf("Updated ownership of %s to %d\n", dataset.Mountpoint, uid)
			}
		} else {
			return "", fmt.Errorf("Invalid format for owners")
		}
	}

	annotations[annDataset] = zfsPath

	return dataset.Mountpoint, nil
}
