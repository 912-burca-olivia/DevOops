
# build and destroy this with 
# terraform init
# terraform validate
# terraform apply -auto-approve
# terraform destroy


terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

provider "digitalocean" {
  token = var.do_token
}

variable "do_token" {}
variable "ssh_fingerprint" {}
variable "private_key_path" {}

resource "digitalocean_droplet" "devoops_green" {
  image    = "docker-20-04"
  name     = "devoops-green"
  region   = "fra1"
  size     = "s-4vcpu-8gb"
  ssh_keys = [var.ssh_fingerprint]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(var.private_key_path)
    host        = self.ipv4_address
  }

  provisioner "remote-exec" {
    inline = [
      "echo 'Green droplet provisioned.' > /root/terraform-test.txt"
    ]
  }
}

resource "digitalocean_droplet" "devoops_blue" {
  image    = "docker-20-04"
  name     = "devoops-blue"
  region   = "fra1"
  size     = "s-4vcpu-8gb"
  ssh_keys = [var.ssh_fingerprint]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(var.private_key_path)
    host        = self.ipv4_address
  }

  provisioner "remote-exec" {
    inline = [
      "echo 'Blue droplet provisioned.' > /root/terraform-test.txt"
    ]
  }
}

resource "digitalocean_floating_ip" "active_ip" {
  region = "fra1"
}

resource "digitalocean_floating_ip_assignment" "active_ip_assign" {
  ip_address = digitalocean_floating_ip.active_ip.ip_address
  droplet_id = digitalocean_droplet.devoops_green.id
}

resource "digitalocean_floating_ip" "passive_ip" {
  region = "fra1"
}

resource "digitalocean_floating_ip_assignment" "passive_ip_assign" {
  ip_address = digitalocean_floating_ip.passive_ip.ip_address
  droplet_id = digitalocean_droplet.devoops_blue.id
}

output "active_floating_ip" {
  value = digitalocean_floating_ip.active_ip.ip_address
}

output "passive_floating_ip" {
  value = digitalocean_floating_ip.passive_ip.ip_address
}
