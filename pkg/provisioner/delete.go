// Modifications copyright (C) 2019 Cream Finance IT Austria GmbH
package provisioner

import (
	"fmt"
	"regexp"

	log "github.com/Sirupsen/logrus"
	zfs "github.com/simt2/go-zfs"
	"k8s.io/api/core/v1"
)

// Delete removes a given volume from the server
func (p ZFSProvisioner) Delete(volume *v1.PersistentVolume) error {
	err := p.deleteVolume(volume)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"volume": volume.Spec.NFS.Path,
	}).Info("Deleted volume")
	return nil
}

// deleteVolume deletes a ZFS dataset from the server
func (p ZFSProvisioner) deleteVolume(volume *v1.PersistentVolume) error {
	// Retrieve annotation from persistent volume
	// Possible attack point by changing the annotation?!
	// (do we need to encrypt the path?)

	datasetName, ok := volume.ObjectMeta.Annotations[annDataset]

	if !ok {
		return fmt.Errorf("Unable to find dataset annotation")
	}

	log.WithFields(log.Fields{
		"datasetName": datasetName,
		"volume": volume,
	}).Info("Parent Dataset")

	// retrieve the dataset
	preDataset, err := zfs.GetDataset(datasetName)

	if err != nil {
		// it's probably already deleted here, just log
		fmt.Printf("Unable to get dataset: %v", err)
		return nil
	}

	var dataset *zfs.Dataset

	matched, _ := regexp.MatchString(`.+\/` + volume.Name, preDataset.Name)

	if matched {
		dataset = preDataset
	}

	if dataset == nil {
		err = fmt.Errorf("Volume %v could not be found", &volume)
	}

	if err != nil {
		return fmt.Errorf("Retrieving ZFS dataset for deletion failed with: %v", err.Error())
	}

	err = dataset.Destroy(zfs.DestroyRecursive)

	if err != nil {
		return fmt.Errorf("Deleting ZFS dataset failed with: %v", err.Error())
	}

	/* Delete created snapshot */
	snapshotName, ok := volume.ObjectMeta.Annotations[annSnapshot]

	if ok {
		// find the snapshot
		snapshot, err := zfs.GetDataset(snapshotName)

		if err != nil {
			return fmt.Errorf("Finding ZFS snapshot failed with: %v", err.Error())
		}

		if snapshot.Type != zfs.DatasetSnapshot {
			return fmt.Errorf("Snapshot is no snapshot?! (%s)", snapshotName)
		}

		err = snapshot.Destroy(zfs.DestroyRecursive)

		if err != nil {
			return fmt.Errorf("Deleting ZFS snapshot failed with: %v", err.Error())
		}
	}




	return nil
}
