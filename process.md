In DevOops folder :
# # Build (only if necessary)
    docker compose build
# # Start containers
    docker compose up -d
# # Check if everything is running
    docker ps
# Check logs if something goes wrong
    docker compose logs
# Stop containers
    docker compose down


Run the app locally: 
    cd itu-minitwit-api/
    go run .

run simulator in separate terminal:
# Create a virtual environment
python3 -m venv venv

# Activate the virtual environment
source venv/bin/activate

# Install requests inside the virtual environment
pip install requests

# Run your Python script
python3 minitwit_simulator.py http://localhost:9090

# Deactivate the virtual environment (when finished)
deactivate

testing:
    pytest refactored_minitwit_tests.py

# # Step 1: Navigate to your project
# cd "./DevOops"

# # Step 2: Ensure your environment variable is set
export DOCKER_USERNAME="your-docker-hub-username"


# docker compose build

# docker compose up -d

# docker ps

# build the docker container locally
docker compose -f docker-compose_local.yml up -d --build
# stop container
docker stop minitwit_api
docker stop minitwit_app
# remove container
docker rm minitwit_api
docker rm minitwit_app

# inspect the database
docker exec -it minitwit_api sh
apt-get update && apt-get install sqlite3
sqlite3 minitwit.db
# run to see what tables you have there
.tables
# delete with drop table commands, and restart the app


# to debug the app locally via Delve 
# install delve
    go install github.com/go-delve/delve/cmd/dlv@latest
# check version
    dlv version
# in itu-minitwit create "launch.json"
{
    "version": "0.2.0",
    "configurations": [
      {
        "name": "Launch",
        "type": "go",
        "request": "launch",
        "mode": "debug",
        "program": "${workspaceFolder}",
        "env": {},
        "args": []
      }
    ]
}
# press F5 in the itu-minitwit-api while having the docker app running and docker api closed 
# go to localhost:8080/{method} to start debugging

# 

# 