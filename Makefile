weChat:
	go build -o weChat && ./weChat
load:
	artillery run load.yml
clean:
	go clean
