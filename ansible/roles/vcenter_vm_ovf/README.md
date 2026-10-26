# vcenter_vm_ovf

Deploys a VM from an OVA through vCenter. cloud-init (through the OVA's vApp properties) seeds the hostname and one sudo user with key-only SSH. The role then sets CPU and memory, grows the root disk, powers the VM on, and waits for VMware Tools to report an IPv4 address, which it publishes as `vcenter_vm_ovf_ip`. A VM that already exists under the name is left as it is and only powered on.

Part of the `lab` mechanism library: a generic, parameterised role. Supply site data (vCenter credentials, placement, the pinned OVA, keys) from your inventory and SOPS, not from the role.

## Requirements

- Runs on the control host (`delegate_to: localhost`) against the vCenter API. `pyvmomi` must be importable by Ansible's Python.
- An OVA whose vApp properties cloud-init reads: `instance-id`, `hostname`, `public-keys` and base64 `user-data`, as Ubuntu's cloud-image OVAs do. After the deploy the role reads the VM's vApp properties back from vCenter and fails unless each one holds the value it set, so an OVA without them fails instead of booting unseeded. Import-spec warnings are printed but not fatal: Ubuntu's OVA carries vendor `extraConfig` keys (such as `enableMPTSupport`) that vCenter can't parse and drops.
- cloud-init can't set a static address through user-data on these images, so the VM boots with DHCP on the port group it's deployed onto. Write a static network config yourself and apply it with `network.yml`.

## Task files

| File | What it does |
| --- | --- |
| `main.yml` | Download and sha256-check the OVA (cached on the control host), deploy it powered off, size it, power it on, wait for its IPv4. |
| `network.yml` | Move the first NIC to `vcenter_vm_ovf_target_network` and reboot the guest, so a network config written but not applied takes effect. |
| `rename.yml` | Rename the VM to `vcenter_vm_ovf_new_name` (a rebuild keeps the old one running under another name). |
| `poweroff.yml` | Power the VM off and keep it (a rebuild's old VM, until the new one is verified). |
| `absent.yml` | Power the VM off and delete it. |

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `vcenter_vm_ovf_hostname` | `""` | vCenter host/IP. |
| `vcenter_vm_ovf_username` | `administrator@vsphere.local` | SSO user. |
| `vcenter_vm_ovf_password` | `""` | SSO password (SOPS). |
| `vcenter_vm_ovf_validate_certs` | `false` | Verify the vCenter API certificate. |
| `vcenter_vm_ovf_datacenter` | `""` | Datacenter. |
| `vcenter_vm_ovf_esxi_hostname` | `""` | ESXi host the VM is placed on. |
| `vcenter_vm_ovf_datastore` | `""` | Datastore. |
| `vcenter_vm_ovf_folder` | `""` | VM folder; empty uses the datacenter root. |
| `vcenter_vm_ovf_name` | `""` | VM name in vCenter. |
| `vcenter_vm_ovf_guest_hostname` | the VM name | Guest hostname. |
| `vcenter_vm_ovf_ova_url` / `_ova_sha256` | `""` | Pinned OVA and its sha256. |
| `vcenter_vm_ovf_cache_dir` | `~/.cache/vcenter_vm_ovf` | OVA cache on the control host. |
| `vcenter_vm_ovf_ovf_network` | `VM Network` | The OVA's network name. |
| `vcenter_vm_ovf_network` | `""` | Port group to deploy onto. |
| `vcenter_vm_ovf_num_cpus` / `_memory_mb` / `_disk_gb` | `2` / `2048` / `16` | Sizing; the root disk only grows. |
| `vcenter_vm_ovf_user` | `ansible` | The cloud-init sudo user. |
| `vcenter_vm_ovf_ssh_authorized_keys` | `[]` | Its SSH keys (required). |
| `vcenter_vm_ovf_target_network` | `""` | `network.yml`: port group to move onto. |
| `vcenter_vm_ovf_new_name` | `""` | `rename.yml`: the new name. |
