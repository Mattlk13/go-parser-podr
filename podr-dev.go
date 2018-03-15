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
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    pong, err := client.Ping().Result()
    fmt.Println(pong, err)

    pubsub := client.Subscribe("podrBrandChannel")
    //pubsub_a := client.Subscribe("podrBrandChannelA")
    defer pubsub.Close()
    //defer pubsub_a.Close()

    // Subscribe
    subscr, err := pubsub.ReceiveTimeout(time.Second)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(subscr)

    /*
    subscr_a, err_a := pubsub_a.ReceiveTimeout(time.Second)
    if err != nil {
        fmt.Println(err_a)
    }
    fmt.Println(subscr_a)
    */

    session, glob_err := mgo.Dial("mongodb://apidev:apidev@localhost:27017/parser")
    defer session.Close()

    if glob_err != nil {
        syslog.Critf("Error: %s", glob_err)
    }
    if DB == "" {
        DB = "parser"
    }

    gapple.ExtractCat(client, session)
    //gapple.ExtractLinks(session, "https://www.auchan.ru/pokupki/hoztovary.html", client, "hoztovary")
    //gapple.ExtractLinks(session, "https://www.podrygka.ru/local/components/taber/symbol.filter/templates/.default/lazyload.ajax.php", client)

    // Link receive loop
    // TODO: goroutine
    for {
        msg, err := pubsub.ReceiveMessage()
        if err != nil {
            fmt.Println("ERROR pubsub: ", err)
        }
        fmt.Println(msg.Channel, " pubsub MSG RCV: ", msg.Payload)
        //gapple.ExtractNavi(session, msg.Payload)
    }

    // Link receive loop
    // TODO: goroutine
    /*
    for {
        msg_a, err_a := pubsub_a.ReceiveMessage()
        if err != nil {
            fmt.Println("ERROR pubsub_a: ", err_a)
        }
        fmt.Println(msg_a.Channel, " pubsub_a MSG RCV: ", msg_a.Payload)
        fmt.Println("pubsub_a", session, msg_a.Payload)
        //gapple.ExtractNavi(session, msg.Payload)
    }
    */
}