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

# -------------------------------
# Variables
# -------------------------------
variable "do_token" {
  description = "DigitalOcean API token"
}


# -------------------------------
# SSH Key Resource
# -------------------------------

variable "ssh_pub_key" {
  description = "Public SSH key for GitHub Actions"
}

resource "digitalocean_ssh_key" "github_actions_key" {
  name       = "github-actions-key"
  public_key = var.ssh_pub_key
}
# -------------------------------
# Droplets
# -------------------------------
resource "digitalocean_droplet" "devoops_green" {
  image    = "docker-20-04"
  name     = "devoops-green"
  region   = "fra1"
  size     = "s-4vcpu-8gb"
  ssh_keys = [digitalocean_ssh_key.github_actions_key.id]
}

resource "digitalocean_droplet" "devoops_blue" {
  image    = "docker-20-04"
  name     = "devoops-blue"
  region   = "fra1"
  size     = "s-4vcpu-8gb"
  ssh_keys = [digitalocean_ssh_key.github_actions_key.id]
}

# -------------------------------
# Floating IPs
# -------------------------------
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

# -------------------------------
# Outputs
# -------------------------------
output "active_floating_ip" {
  value = digitalocean_floating_ip.active_ip.ip_address
}

output "passive_floating_ip" {
  value = digitalocean_floating_ip.passive_ip.ip_address
}
