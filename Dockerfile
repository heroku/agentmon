FROM golang:alpine

ADD . /go/src/github.com/heroku/agentmon

RUN cd /go/src/github.com/heroku/agentmon \
 && go build ./... \
 && go install github.com/heroku/agentmon/cmd/...

