package main

import (
    "os"
    "fmt"
    "time"
    "gapple"
    "gopkg.in/mgo.v2"
    "github.com/go-redis/redis"
    "github.com/blackjack/syslog"
)

const (
    C_HOST = "0.0.0.0"
    C_PORT = "8803"
    C_TYPE = "tcp"
)

const ENV_PREF = "prod"

var DB string = os.Getenv("RIVE_MONGO_DB")

func main() {

    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
        Password: "",
        DB: 0,
    })

    pong, err := client.Ping().Result()
    fmt.Println(pong, err)

    pubsub := client.Subscribe("podrBrandChannel")
    defer pubsub.Close()

    // Subscribe
    subscr, err := pubsub.ReceiveTimeout(time.Second)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(subscr)

    session, glob_err := mgo.Dial("mongodb://apidev:apidev@localhost:27017/parser")
    defer session.Close()

    if glob_err != nil {
        syslog.Critf("Error: %s", glob_err)
    }
    if DB == "" {
        DB = "parser"
    }

    gapple.ExtractCat(client, session)

    for {
        msg, err := pubsub.ReceiveMessage()
        if err != nil {
            fmt.Println("ERROR pubsub: ", err)
        }
        fmt.Println(msg.Channel, " pubsub MSG RCV: ", msg.Payload)
    }
}