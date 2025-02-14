### DevOps ITU Spring 2025 
### Welcome to DevOops!
#### 5 humans to develop: Olivia, Iulia, Max, Tore and Gwen
#### Many ways to say Oops: Romanian, Dutch, Danish, English

Here at MiniTwit, you will find a nifty (to-be) Tweeter-esque app allowing you to register as a user, follow others, post to your timeline, and snoop on others' timelines. 
You can just see a public timeline without signing in if you rather not give us your data.

To run the app, in the Head folder:
`$ Docker compose build`
`$ Docker compose up`.
This should spin up a container and allow you to access the app via a browser on the local host.

If you would like to run tests, in the itu-minitwit folder, while the container is running:
`$ pytest -v refactored_minitwit_tests.py `.
Note that the Go test suite is currently not full functional, but nothing will break if you care to have a look anyway
`$ go test -v`.

Have fun! 
DevOops
