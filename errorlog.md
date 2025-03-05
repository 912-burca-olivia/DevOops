## here we can maintain a log of errors from the simulator and our response (interpretation, action and time)

03-03-2025 11:30
errorTimeout: the api does not respond to a request in <0.3 seconds
It seems that problem lies in that we hash the password in the register. Which means that our request takes longer than 0.3 seconds

03-03-2025 19:30
Tweet errors might have come from that accounts weren't created fast enough due to previous error. Tweets are posted from an account that has not been created
