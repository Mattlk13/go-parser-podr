package gapple

import (
    //"os"
    "io"
    "fmt"
    "time"
    "bytes"
    "strconv"
    "strings"
    "net/http"
    //"encoding/json"
    //"io/ioutil"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "golang.org/x/net/html"
    "github.com/go-redis/redis"
    //"github.com/json-iterator/go"
    "github.com/blackjack/syslog"
    "github.com/parnurzeal/gorequest"
)

// Add month prefix
func MakeTimeMonthlyPrefix(coll string) string {
    t := time.Now()
    ti := t.Format("01-2006")
    fin := ti + "_" + coll
    return fin
}

// A time prefix before collection name
func MakeTimePrefix(coll string) string {
    t := time.Now()
    ti := t.Format("02-01-2006")
    if coll == "" {
        return ti
    }
    fin := ti + "_" + coll
    return fin
}

// Render node
func renderNode(node *html.Node) string {
    var buf bytes.Buffer
    w := io.Writer(&buf)
    err := html.Render(w, node)
    if err != nil {
        syslog.Critf("Error: %s", err)
    }
    return buf.String()
}

// Get tag context
// TODO: prevent endless loop
func extractContext(s string) string {
    z := html.NewTokenizer(strings.NewReader(s))

    for {
        tt := z.Next()
        switch tt {
            case html.ErrorToken:
                syslog.Critf("links.go extractContext() error: %s", z.Err())
                syslog.Critf("String: %s", s)
                return ""
            case html.TextToken:
                text := string(z.Text())
                return text
        }
    }
}

// https://www.podrygka.ru/catalog/?PAGEN_1=661
func ExtractCat(redis_cli *redis.Client, glob_session *mgo.Session) {

    fmt.Println("START", redis_cli, glob_session)

    var Navi []string

    type Product struct {
        Articul, Name, Price, OldPrice, Country, Img, Brand, Navi, Url, Date string
    }

    type Price struct {
        Name, Price, OldPrice, Date string
    }

    var f func(*html.Node, *mgo.Session)
    var f1 func(*html.Node, *mgo.Session)
    var f2 func(*html.Node, *mgo.Session, *Product)
    var f3 func(*html.Node, *mgo.Session, *Product)
    var f4 func(*html.Node, *mgo.Session, *Product)

    f = func(node *html.Node, session *mgo.Session) {

        if node.Type == html.ElementNode && node.Data == "div" {
            for _, a := range node.Attr {
                if a.Key == "class" {
                    if strings.Contains(a.Val, "products-list row") {
                        f1(node, session)
                    }
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f(c, session)
        }
    }

    // products-list row DIV node
    f1 = func(node *html.Node, session *mgo.Session) {

        if node.Type == html.ElementNode && node.Data == "a" {
            for _, a := range node.Attr {
                if a.Key == "href" {
                    if strings.Contains(a.Val, "/catalog/") && !strings.Contains(a.Val, "mark-is-msk-new") {
                        if !strings.Contains(a.Val, "mark-is-msk") {
                            fmt.Println("")
                            fmt.Println("f1", "https://www.podrygka.ru"+a.Val)
                            request := gorequest.New()
                            resp, body, errs := request.Get("https://www.podrygka.ru"+a.Val).
                                Retry(3, 5 * time.Second, http.StatusBadRequest, http.StatusInternalServerError).
                                End()
                            _ = resp
                            if errs != nil {
                                syslog.Critf("links.go request.Get(BrandUrl) error: %s", errs)
                            }

                            doc, err := html.Parse(strings.NewReader(string(body)))

                            if err != nil {
                                syslog.Critf("links.go html.Parse error: %s", errs)
                            }

                            // Product page body
                            Navi = []string{}
                            product := Product{}
                            price := Price{}
                            f2(doc, glob_session, &product)
                            product.Navi = strings.Join(Navi, ";")
                            product.Url = "https://www.podrygka.ru"+a.Val
                            price.Name = product.Name
                            price.Price = product.Price
                            price.Date = MakeTimePrefix("")
                            product.Date = MakeTimePrefix("")
                            fmt.Println(product)

                            // Insert new product
                            c := session.DB("parser").C("PODR_products_final")
                            d := session.DB("parser").C(MakeTimeMonthlyPrefix("PODR_price"))
                            session.SetMode(mgo.Monotonic, true)

                            // check double product
                            num, err := c.Find(bson.M{"name": product.Name}).Count()
                            if err != nil {
                                syslog.Critf("links.go error find by code: %s", err)
                            }
                            // check double price (today)
                            num_p, err := d.Find(bson.M{"name": price.Name, "date": price.Date}).Count()
                            if err != nil {
                                syslog.Critf("links.go find price double: %s", err)
                            }

                            // New product
                            if num < 1 {

                                err := c.Insert(product)
                                if err != nil {
                                    syslog.Critf("links.go error insert product: %s", err)
                                }
                                fmt.Println("AU Inserted", product.Name)
                                syslog.Syslog(syslog.LOG_INFO, "AU Inserted: "+product.Name)

                                // Today double price ignore
                                if num_p < 1 {
                                    err = d.Insert(price)
                                    if err != nil {
                                        syslog.Critf("links.go error insert price: %s", err)
                                    }
                                }

                            // Double
                            } else {

                                // Update product
                                err = c.Update(bson.M{"name": product.Name}, product)
                                if err != nil {
                                    syslog.Critf("links.go c.Update error: %s", err)
                                }
                                fmt.Println("AU Updated", product.Name)
                                syslog.Syslog(syslog.LOG_INFO, "AU Updated: "+product.Name)

                                // Insert price on double
                                if num_p < 1 {
                                    err = d.Insert(price)
                                    if err != nil {
                                        syslog.Critf("links.go d.Insert error: %s", err)
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f1(c, session)
        }
    }

    // Extract product page
    f2 = func(node *html.Node, session *mgo.Session, product *Product) {

        if node.Type == html.ElementNode && node.Data == "a" {
            for _, a := range node.Attr {
                if a.Val == "breadcrumbs-item" {
                    contents := renderNode(node)
                    contents = extractContext(contents)
                    contents = strings.Trim(contents, " ")
                    Navi = append(Navi, contents)
                }
            }
        }

        // Breadcrumbs end
        if node.Type == html.ElementNode && node.Data == "span" {
            for _, a := range node.Attr {
                if a.Val == "breadcrumbs-item" {
                    contents := renderNode(node)
                    contents = extractContext(contents)
                    contents = strings.Trim(contents, " ")
                    Navi = append(Navi, contents)
                }
            }
        }

        // Name + 1st try brand extract
        if node.Type == html.ElementNode && node.Data == "h1" {
            for _, a := range node.Attr {
                if a.Key == "data-showifproduct" {
                    contents := renderNode(node)
                    contents = extractContext(contents)
                    product.Name = strings.Trim(contents, " ")
                    if strings.Contains(product.Name, "`") {
                        brand := strings.Split(product.Name, "`")
                        product.Brand = brand[1]
                    }
                }
            }
        }
                                                                                                    
        if node.Type == html.ElementNode && node.Data == "span" {
            for _, a := range node.Attr {
                if strings.Contains(a.Val, "price__item--current") {
                    contents := renderNode(node)
                    //contents = extractContext(contents)
			contents = strings.Replace(contents, "<span class=\"price_value\">", "", -1)
                    contents = strings.Replace(contents, "</span>", "", -1)
                    contents = strings.Replace(contents, "<span class=\"price__item price__item--current\">", "", -1)
                    contents = strings.Replace(contents, "\n", "", -1)
                    contents = strings.Replace(contents, "\r", "", -1)
                    contents = strings.Replace(contents, "\t", "", -1)
                    contents = strings.Replace(contents, "<span class=\"rouble\">", "", -1)
			contents = strings.Replace(contents, "р.", "", -1)

                    product.Price = strings.Trim(contents, " ")
                    fmt.Println("CURRENTPRICE", product.Price)
                }
            }
        }

        if node.Type == html.ElementNode && node.Data == "span" {
            for _, a := range node.Attr {
                if strings.Contains(a.Val, "price__item--old") {
                    contents := renderNode(node)
                    //contents = extractContext(contents)
                    contents = strings.Replace(contents, "</span>", "", -1)
			contents = strings.Replace(contents, "<span class=\"price_value\">", "", -1)
			contents = strings.Replace(contents, "<span class=\"price__item price__item--old\">", "", -1)
                    contents = strings.Replace(contents, "", "", -1)
                    contents = strings.Replace(contents, "\n", "", -1)
                    contents = strings.Replace(contents, "\r", "", -1)
                    contents = strings.Replace(contents, "\t", "", -1)
                    contents = strings.Replace(contents, "<span class=\"rouble\">", "", -1)
			contents = strings.Replace(contents, "р.", "", -1)

                    product.OldPrice = strings.Trim(contents, " ")
                    fmt.Println("OLDPRICE DETECTED", product.OldPrice)
                }
            }
        }

        if node.Type == html.ElementNode && node.Data == "div" {
            for _, a := range node.Attr {
                if a.Val == "product-detail__gallery-slider-item" {
                    if len(product.Img) < 3 {
                        f3(node, glob_session, product)
                    }
                }
            }
        }

        // Second try brand extract
        if node.Type == html.ElementNode && node.Data == "div" {
            for _, a := range node.Attr {
                if a.Val == "product-detail__brand" {
                    f4(node, glob_session, product)
                }
            }
        }

        if node.Type == html.ElementNode && node.Data == "div" {
            for _, a := range node.Attr {
                if a.Val == "product-detail__articul" {
                    contents := renderNode(node)
                    //contents = extractContext(contents)
                    contents = strings.Replace(contents, "<div class=\"product-detail__articul\">", "", -1)
                    contents = strings.Replace(contents, "</div>", "", -1)
                    contents = strings.Replace(contents, "\n", "", -1)
                    contents = strings.Replace(contents, "\r", "", -1)
                    contents = strings.Replace(contents, "\t", "", -1)
                    contents = strings.Replace(contents, "<span>", "", -1)
                    contents = strings.Replace(contents, "</span>", "", -1)
                    contents = strings.Replace(contents, "Артикул", "", -1)
                    contents = strings.Replace(contents, "<span class=\"value\">", "", -1)

                    product.Articul = strings.Trim(contents, " ")
                }
            }
        }

        if node.Type == html.ElementNode && node.Data == "div" {
            for _, a := range node.Attr {
                if a.Val == "product-detail__country" {
                    contents := renderNode(node)
                    //contents = extractContext(contents)
                    contents = strings.Replace(contents, "<div class=\"product-detail__country\">", "", -1)
                    contents = strings.Replace(contents, "</div>", "", -1)
                    contents = strings.Replace(contents, "\n", "", -1)
                    contents = strings.Replace(contents, "\r", "", -1)
                    contents = strings.Replace(contents, "\t", "", -1)
                    contents = strings.Replace(contents, "<span>", "", -1)
                    contents = strings.Replace(contents, "</span>", "", -1)
                    contents = strings.Replace(contents, "Страна:", "", -1)
                    contents = strings.Replace(contents, "<span class=\"value\">", "", -1)

                    product.Country = strings.Trim(contents, " ")
                }
            }
        }



        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f2(c, session, product)
        }
    }

    // Extract product pages
    f3 = func(node *html.Node, session *mgo.Session, product *Product) {
        if node.Type == html.ElementNode && node.Data == "img" {
            for _, a := range node.Attr {
                if a.Key == "src" {
                    product.Img = "https://www.podrygka.ru" + a.Val
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f3(c, session, product)
        }
    }

    // Brand extract
    f4 = func(node *html.Node, session *mgo.Session, product *Product) {
        if node.Type == html.ElementNode && node.Data == "img" {
            for _, a := range node.Attr {
                if a.Key == "alt" {
                    product.Brand = a.Val
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f4(c, session, product)
        }
    }



    // *************************
    i := 1
    for i < 690 {
        fmt.Println("I =:", i)
        //fmt.Println("https://www.podrygka.ru/catalog/?PAGEN_1="+strconv.Itoa(i))
        if i == 661 {break}
        request := gorequest.New()
        resp, body, errs := request.Get("https://www.podrygka.ru/catalog/?PAGEN_1="+strconv.Itoa(i)).
            Retry(3, 5 * time.Second, http.StatusBadRequest, http.StatusInternalServerError).
            End()
        _ = resp
        if errs != nil {
            syslog.Critf("/catalog/?PAGEN_1 request.Get(BrandUrl) error: %s", errs)
        }

        doc, err := html.Parse(strings.NewReader(string(body)))

        if err != nil {
            syslog.Critf("links.go html.Parse error: %s", errs)
        }

        fmt.Println("")
        fmt.Println("https://www.podrygka.ru/catalog/?PAGEN_1="+strconv.Itoa(i))
        syslog.Syslog(syslog.LOG_INFO, "https://www.podrygka.ru/catalog/?PAGEN_1="+strconv.Itoa(i))
        f(doc, glob_session)

        i++
    }
}

func ExtractLinks(glob_session *mgo.Session, url string, redis_cli *redis.Client) {

    var f func(*html.Node, *mgo.Session)
    var f1 func(*html.Node, *mgo.Session)

    // 
    f = func(node *html.Node, session *mgo.Session) {
        if node.Type == html.ElementNode && node.Data == "a" {
            for _, a := range node.Attr {
                if a.Key == "href" {
                    if strings.Contains(a.Val, "catalog") {

                        request := gorequest.New()
                        resp, body, errs := request.Get("https://www.podrygka.ru"+a.Val).
                            Retry(3, 5 * time.Second, http.StatusBadRequest, http.StatusInternalServerError).
                            End()
                        _ = resp
                        if errs != nil {
                            syslog.Critf("links.go request.Get(BrandUrl) error: %s", errs)
                        }

                        doc, err := html.Parse(strings.NewReader(string(body)))

                        if err != nil {
                            syslog.Critf("links.go html.Parse error: %s", errs)
                        }

                        fmt.Println("https://www.podrygka.ru"+a.Val)
                        f1(doc, glob_session)

                        /*
                        err := redis_cli.Publish("podrBrandChannel", "https://www.podrygka.ru"+a.Val).Err()
                        if err != nil {
                            fmt.Println(`redis_cli.Publish("podrBrandChannel", a.Val).Err()`)
                        }
                        */
                    }
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f(c, session)
        }
    }

    f1 = func(node *html.Node, session *mgo.Session) {
        if node.Type == html.ElementNode && node.Data == "a" {
            for _, a := range node.Attr {
                if a.Val == "breadcrumbs-item" {
                    contents := renderNode(node)
                    contents = extractContext(contents)
                    fmt.Println(contents)
                }
            }
        }

        // iterate inner nodes recursive
        for c := node.FirstChild; c != nil; c = c.NextSibling {
            f1(c, session)
        }
    }

    // Brands
    request := gorequest.New()
    resp, body, errs := request.Get(url).
        Retry(3, 5 * time.Second, http.StatusBadRequest, http.StatusInternalServerError).
        End()
    _ = resp
    if errs != nil {
        syslog.Critf("links.go request.Get(BrandUrl) error: %s", errs)
    }

    doc, err := html.Parse(strings.NewReader(string(body)))

    if err != nil {
        syslog.Critf("links.go html.Parse error: %s", errs)
    }

    f(doc, glob_session)
}
