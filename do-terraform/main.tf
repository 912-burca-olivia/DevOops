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
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# -------------------------------
# Providers
# -------------------------------
provider "digitalocean" {
  token = var.do_token
}

provider "aws" {
  alias                       = "spaces"
  region                      = var.spaces_region
  access_key                  = var.spaces_access_key
  secret_key                  = var.spaces_secret_key
  skip_metadata_api_check     = true
  skip_credentials_validation = true
  endpoints {
    s3 = var.spaces_endpoint
  }
}
# -------------------------------
# Variables
# -------------------------------
variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
}

variable "ssh_pub_key" {
  description = "Public SSH key for GitHub Actions"
  type        = string
}

variable "spaces_access_key" {
  description = "Access key for DigitalOcean Spaces"
  type        = string
}

variable "spaces_secret_key" {
  description = "Secret key for DigitalOcean Spaces"
  type        = string
}

variable "spaces_region" {
  description = "Region of the DigitalOcean Space (e.g. fra1)"
  type        = string
}

variable "spaces_endpoint" {
  description = "Endpoint URL for the DigitalOcean Space"
  type        = string
}

# -------------------------------
# SSH Key Resource
# -------------------------------
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
# Spaces Bucket (Optional Example)
# -------------------------------
resource "aws_s3_bucket" "artifact_bucket" {
  provider = aws.spaces
  bucket   = "devoops-artifacts"
  acl      = "private"
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

output "spaces_bucket_name" {
  value = aws_s3_bucket.artifact_bucket.id
}
