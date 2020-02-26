package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	bearer "github.com/Bearer/bearer-go"
	circleci "github.com/jszwedko/go-circleci"
	"moul.io/godev"
)

func main() {
	bearer.ReplaceGlobals(bearer.Init(os.Getenv("BEARER_SECRETKEY")))

	hc := &http.Client{
		Timeout: time.Second * 1800,
	}
	cc := &circleci.Client{Token: os.Getenv("CIRCLE_TOKEN"), HTTPClient: hc}

	builds, err := cc.ListRecentBuilds(5, 0)
	if err != nil {
		log.Printf("err: %#v", err)
		return
	}

	fmt.Println(godev.PrettyJSON(builds))
}
