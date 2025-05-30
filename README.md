### DevOps ITU Spring 2025 
### Group M MiniTwit Repository
#### 5 humans to develop: Olivia, Iulia, Max, Tore and Gwen
#### Many ways to say Oops: Romanian, Dutch, Danish, English, Mandarin

Authors: Iulia Maria Negrila, Max Bezemer, Olivia Burca, Tore Kjelds, Su Mei Gwen Ho \
Emails: iune@itu.dk, mbez@itu.dk, olbu@itu.dk, tokj@itu.dk, suho@itu.dk \
LLM Co-Authors: Gemini, ChatGPT \

Here at MiniTwit, you will find a nifty Tweeter-esque app allowing you to register as a user, follow others, post to your timeline, and snoop on others' timelines. \
You can just see a public timeline without signing in if you rather not give us your data.

This is a containerised set-up which only requires Docker to run.

To run the App and API with the associated logging and monitoring services, in the head directory run:
`docker-compose -f docker-compose_local.yml up`
This should spin up all containers needed and allow you to access the app via a browser on the local host.

If you would like to run tests run:
`docker-compose -f docker-compose_test_local.yml up`
This test environment includes, the API simulator tests, the App unit tests and the e2e tests. \
A summary of passed/failed tests is printed in the terminal.

Ports:\
App               http://localhost:8080/ \
API               http://localhost:7070/ \
Prometheus        http://localhost:9090/ \
Grafana           http://localhost:3000/ \
Kibana Dashboard  http://localhost:5601/ \

Use the default log-ins for Kibana and Grafana when you run the environments locally:\
Grafana: Admin / Admin \
Kibana: Admin / Admin \

Deployment is automated using Github Actions. The deployment workflow can be triggered by pushing to Main, or by manually running the Action. 

Have fun! \
DevOops
