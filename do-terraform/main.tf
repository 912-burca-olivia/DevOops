
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

resource "digitalocean_droplet" "devoops" {
  image  = "docker-20-04"
  name   = "devoops-terraform"
  region = "fra1"
  size   = "s-1vcpu-2gb"

  ssh_keys = [var.ssh_fingerprint]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(var.private_key_path)
    host        = self.ipv4_address
  }

  provisioner "remote-exec" {
    inline = [
      "echo 'Connection successful!' > /root/terraform-test.txt"
      # here we can add the configuration for the files needed on the droplet
      # still have to figure out how to include green/blue passive/active in this
    ]
  }
}

variable "do_token" {}
variable "ssh_fingerprint" {}
variable "private_key_path" {}
# variable "docker_compose_file" {
#   default = file("docker-compose.yml")
# }
