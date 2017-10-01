FROM golang:1.8
RUN apt-get update && apt-get install -y telnet
RUN mkdir -p $GOPATH/src/github.com/klebervirgilio/simple-healthchecker-go
EXPOSE 4040