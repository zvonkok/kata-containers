#!/bin/bash

switch_type=${switch_type:-PEX890xx}

get_iommu_group() {
  local bdf="$1"
  local group_path
  group_path="$(readlink -f "/sys/bus/pci/devices/$bdf/iommu_group" 2>/dev/null || true)"
  if [ -z "$group_path" ]; then
    echo "N/A"
  else
    basename "$group_path"
  fi
}


get_upstream_switch_id() {
  local bdf="$1"
  local dev_path="/sys/bus/pci/devices/$bdf"
  local parent_path="$dev_path"

  # We’ll keep going up until we find no more parents
  # or we reach a PCI root device
  while true; do
    # symlink parent
    local parent
    parent="$(dirname "$(readlink -f "$parent_path")")"
    # If the parent is the same as the device's directory,
    # we have no further to go
    if [[ "$parent" == "$parent_path" ]]; then
      break
    fi
    # The parent directory’s basename should be the BDF for the parent
    local parent_bdf
    parent_bdf="$(basename "$parent")"

    # If parent_bdf doesn’t look like a BDF, we likely
    # reached the top (e.g. 'pci0000:00'). Stop here.
    if ! [[ "$parent_bdf" =~ [0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-9a-fA-F] ]]; then
      break
    fi

    # Check parent’s device type with lspci
    local dev_type
    dev_type="$(lspci -s "$parent_bdf" -D 2>/dev/null | awk -F': ' '{print $2}' || true)"
    # If this parent is the root port or upstream switch,
    # we might want to stop here. Adjust logic as needed.
    # Common keywords are "Root Port", "Upstream Port"
    if ! [[ "$dev_type" =~ ${switch_type} ]] ; then
      # This is typically our upstream switch or root port
      #echo "$parent_bdf"
      basename "${parent_path}"
      return 0
    fi

    # Otherwise, keep going up
    parent_path="$parent"
  done

  # If we never found an upstream switch or root port,
  # we just return the device itself so that devices
  # under the same parent will match on themselves.
  echo "$bdf"
}


pci_devices=$(lspci -nD)

echo ""
echo "NV GPUs 0302 3D ----------------------"
pci_3d_devices=$(echo "$pci_devices" | grep 10de | grep 0302)
echo "$pci_3d_devices"
echo ""
echo "MX NICs 0207 IB ----------------------"
pci_ib_devices=$(echo "$pci_devices" | grep 15b3 | grep 0207)
echo "$pci_ib_devices"
echo ""
pci_3d_bdf=$(echo "$pci_3d_devices" | awk '{print $1}')
pci_ib_bdf=$(echo "$pci_ib_devices" | awk '{print $1}')

declare -A iommu_map
declare -A switch_map


for bdf in $pci_3d_bdf; do
  iommu_map[$bdf]=$(get_iommu_group "$bdf")
done

for bdf in $pci_ib_bdf; do
  iommu_map[$bdf]=$(get_iommu_group "$bdf")
done

echo "GPU - IOMMU GROUP MAP -----------------"
for bdf in $pci_3d_bdf; do
  echo "$bdf" "${iommu_map["$bdf"]}"
done
echo ""

echo "NIC - IOMMU GROUP MAP -----------------"
for bdf in $pci_ib_bdf; do
  echo "$bdf" "${iommu_map["$bdf"]}"
done
echo ""

for bdf in $pci_3d_bdf; do
  switch_map[$bdf]=$(get_upstream_switch_id "$bdf")
done

for bdf in $pci_ib_bdf; do
  switch_map[$bdf]=$(get_upstream_switch_id "$bdf")
done

echo "GPU - SWITCH MAP ----------------------"
for bdf in $pci_3d_bdf; do
  echo "$bdf" "${switch_map[$bdf]}"
done
echo ""

echo "NIC - SWITCH MAP ----------------------"
for bdf in $pci_ib_bdf; do
  echo "$bdf" "${switch_map[$bdf]}"
done
echo ""


for gpu in $pci_3d_bdf; do
  for nic in $pci_ib_bdf; do
    if [[ "${switch_map["$gpu"]}" == "${switch_map["$nic"]}" ]]; then
      echo "GPU: $gpu /dev/vfio/${iommu_map["$gpu"]}  <--->  NIC: $nic /dev/vfio/${iommu_map["$nic"]}"
    fi
  done
done