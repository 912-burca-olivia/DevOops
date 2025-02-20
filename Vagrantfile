Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-22.04"

  config.vm.provider "virtualbox" do |vb|
    vb.memory = "1024"
    vb.cpus = 2
  end

  config.vm.network "forwarded_port", guest: 8080, host: 8080
  config.vm.network "forwarded_port", guest: 9090, host: 9090

  config.vm.provision "shell", inline: <<-SHELL
    # Step 1: Remove old Docker versions
    for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do
      sudo apt-get remove -y $pkg
    done

    # Step 2: Add Docker's official GPG key
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc

    # Step 3: Add Docker repository
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    sudo apt-get update

    # Step 4: Install Docker and Docker Compose
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Step 5: Ensure Docker is running
    sudo systemctl enable docker
    sudo systemctl start docker

    # Step 6: Add vagrant user to Docker group
    sudo usermod -aG docker vagrant

    # Step 7: Clone GitHub repository
    git clone https://github.com/912-burca-olivia/DevOops.git /home/vagrant/DevOops

    # Step 8: Navigate to the correct directory and start the app
    cd /home/vagrant/DevOops
    sudo -u vagrant docker compose build
    sudo -u vagrant docker compose up -d
  SHELL

  config.vm.network "private_network", type: "dhcp"
end
