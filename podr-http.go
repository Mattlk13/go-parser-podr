package main

import (
    "os"
    "fmt"
    "time"
    "gapple"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "github.com/gin-gonic/gin"
    "github.com/go-redis/redis"
    "github.com/blackjack/syslog"
)

const (
    C_HOST = "0.0.0.0"
    C_PORT = "8803"
    C_TYPE = "tcp"
)

const ENV_PREF = "prod"

var DB string = os.Getenv("PODR_MONGO_DB")

func main() {

    r := gin.Default()

    // Start parser
    r.GET("/v1/start", func(c *gin.Context) {
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

        //collections := session.DB(DB).C("PODR_products_final")
        collections := session.DB("parser").C(gapple.MakeTimeMonthlyPrefix("PODR_price"))
        session.SetMode(mgo.Monotonic, true)

        num, err := collections.Find(bson.M{"date": gapple.MakeTimePrefix("")}).Count()

        if err != nil {
            panic(err)
        }

        if num > 1 {
            syslog.Err("PODR brands allready parsed today")
            fmt.Println("PODR brands allready parsed today")
            c.JSON(200, gin.H{
                "message": "PODR brands allready parsed today",
            })
        } else {
            c.JSON(200, gin.H{
                "message": "r.GET(\"/start\", func(c *gin.Context)",
            })

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
    })

    r.GET("/v1/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "message": "pong",
        })
    })

    // Parse single product page
    r.GET("/", func(c *gin.Context) {
        c.String(200, "GA")
    })

    r.Run(":"+C_PORT)
}