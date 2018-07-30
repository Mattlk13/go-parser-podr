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

func main() {

    var DB string = os.Getenv("PODR_MONGO_DB")

    client := redis.NewClient(&redis.Options{
        Addr: "192.168.65.243:6379",
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

    session, glob_err := mgo.Dial("mongodb://apidev:apidev@192.168.65.243:27017/parser")
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