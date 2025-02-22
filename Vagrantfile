Vagrant.configure("2") do |config|
  config.vm.box = 'digital_ocean'  

  config.ssh.private_key_path = "~/.ssh/id_rsa" 
  config.vm.synced_folder ".", "/vagrant", disabled: true

  # DigitalOcean Provider Configuration
  config.vm.provider :digital_ocean do |provider|
    provider.token = ENV["DIGITAL_OCEAN_TOKEN"]        
    provider.ssh_key_name = ENV["SSH_KEY_NAME"]          
    provider.image = 'ubuntu-22-04-x64'                 
    provider.region = 'fra1'                          
    provider.size = 's-2vcpu-2gb'                      
  end

  config.vm.network "forwarded_port", guest: 8080, host: 8080
  config.vm.network "forwarded_port", guest: 5001, host: 5001

  config.vm.provision "shell", inline: <<-SHELL

    # Add Docker's official GPG key
    apt-get update
    apt-get install -y ca-certificates curl gnupg lsb-release git
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc

    # Add Docker repository
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}") stable" |  tee /etc/apt/sources.list.d/docker.list > /dev/null

    apt-get update

    # Install Docker, Docker Compose etc
    apt-get install -y git docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Ensure Docker is running
    systemctl enable docker
    systemctl start docker

    # Clone GitHub repository
    git clone https://github.com/912-burca-olivia/DevOops.git /home/vagrant/DevOops

    # Navigate to the correct directory and start the app
    cd /home/vagrant/DevOops
    docker compose up -d --build
    

  SHELL
end
