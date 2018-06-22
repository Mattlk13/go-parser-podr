FROM golang:latest
RUN mkdir /app
RUN go get gopkg.in/mgo.v2
RUN go get gopkg.in/mgo.v2/bson
RUN go get golang.org/x/net/html
RUN go get github.com/blackjack/syslog
RUN go get github.com/parnurzeal/gorequest
RUN go get github.com/gin-gonic/gin
RUN go get github.com/go-redis/redis
ADD . /app/
WORKDIR /app
RUN cp -r src/gapple /go/src
RUN go build gapple
RUN go install gapple
RUN go build -o podr-http
EXPOSE 8803
CMD ["/app/podr-http"]