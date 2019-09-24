#
# Virtual Machine node
#

# Use locally pre-fetched image
resource "libvirt_volume" "os-img" {
  name    = "os-img"
  pool    = "default"
  source  = "/var/lib/libvirt/images/${lookup(var.os_images, var.os)}"
  format  = "raw"
}

# Create a network for our VMs
resource "libvirt_network" "vm_network" {
   name       = "vm_network"
   addresses  = ["172.28.128.0/24"]
}

# Create main disk
resource "libvirt_volume" "gravity" {
  name            = "gravity-disk-${count.index}"
  base_volume_id  = libvirt_volume.os-img.id
  pool            = "${var.storage_pool}"
  size            = "${var.disk_size}"
  count           = "${var.nodes}"
}

# Use CloudInit to add our ssh-key to the instance
resource "libvirt_cloudinit_disk" "commoninit" {
  name      = "commoninit-${count.index}.iso"
  user_data = "${templatefile("${path.module}/cloudinit/${split(":","${var.os}").0}.cfg", {
    ssh_pub_key = "${file(var.ssh_pub_key_path)}",
    ip_address = "172.28.128.${count.index+3}",
    hostname = "gravity${count.index}"
  })}"
  count = "${var.nodes}"
}

# Create the machine
resource "libvirt_domain" "domain-gravity" {
  name      = "gravity${count.index}"
  memory    = "${var.memory}"
  vcpu      = "${var.cpu}"
  cpu       = {
    mode = "host-passthrough"
  }
  count     = "${var.nodes}"
  cloudinit = "${element(libvirt_cloudinit_disk.commoninit.*.id, count.index)}"

  network_interface {
    hostname        = "gravity${count.index}"
    network_id      = "${libvirt_network.vm_network.id}"
    addresses       = ["172.28.128.${count.index+3}"]
    mac             = "6E:02:C0:21:62:5${count.index+3}"
    wait_for_lease  = true
  }

  # IMPORTANT
  # Ubuntu can hang if an isa-serial is not present at boot time.
  # If you find your CPU 100% and never is available this is why
  console {
    type        = "pty"
    target_port = "0"
    target_type = "serial"
  }

  console {
    type        = "pty"
    target_type = "virtio"
    target_port = "1"
  }

  disk {
    volume_id = "${element(libvirt_volume.gravity.*.id, count.index)}"
  }
}
