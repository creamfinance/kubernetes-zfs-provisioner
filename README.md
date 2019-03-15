# kubernetes-zfs-provisioner

zfs-provisioner is an out of cluster external provisioner for Kubernetes. It creates ZFS datasets and shares them via NFS to make them mountable to pods. Currently all ZFS attributes are inherited from the parent dataset, different storage classes for e.g. cached/non-cached datasets or manually setting attributes via annotations should follow in the future. This provisioner is considered highly **experimental** and is still under development.

For more information about external storage in kubernetes, see [kubernetes-incubator/external-storage](https://github.com/kubernetes-incubator/external-storage).

## Usage
The provisioner can be configured via the following environment variables:

| Variable | Description | Default |
| :------: | :---------- | :-----: |
| `ZFS_PROVISIONER_NAME` | Name of the provisioner. Change only if you want to run multiple instances. | `creamfinance.com/zfs` |
| `ZFS_KUBE_CONF` | Path to the kubernetes config file which will be used to connect to the cluster. |`kube.conf` |
| `ZFS_METRICS_PORT` | Port on which to export Prometheus metrics. | `8080` |

## Notes
### Annotations
The dataset that was created is saved in the persistent volume as a annotation `creamfinance.com/zfs-dataset`.
On delete the dataset name is checked against the PV Name - so only datasets that match exactly the name of the volume will actually be deleted to hinder manipulation.

The second annotation that is available is `creamfinance.com/zfs-owner` which can be added to a pvc to set the ownership of the root folder.

### Storage space
The provisioner uses the `reflimit` and `refquota` ZFS attributes to limit storage space for volumes.
The overProvision property in the storage class controls if reflimit is set or not, refquota is always set.

## Development

The tests need to manage ZFS datasets, create a testing pool on a disk image:

```
# Create a 10GB disk image
dd if=/dev/zero bs=1024m count=10 of=disk.img
```

### Linux

```
runcate --size 1G disk1.img
sudo zpool create pool1 $PWD/disk1.img -m $PWD/test
```

### Mac

```
# Mount the image as a block device, MacOS way
hdiutil attach -imagekey diskimage-class=CRawDiskImage -nomount disk.img
# Create zpool with mount in current directory
sudo zpool create -m $PWD/test -f test /dev/disk2
```
For development under other operating systems, adapt mount command and block device.

## Building

You need GO and go-dep

```
sudo apt install golang-go go-dep

# If $GOPATH is empty
mkdir -p ~/go
export GOPATH=$HOME/go

mkdir -p $GOPATH/src
ln -s $PATH_TO_REPO $GOPATH/src/kubernetes-zfs-provisioner
cd $GOPATH/src/kubernetes-zfs-provisioner

# Install dependencies
dep ensure

# Build
make build
```
