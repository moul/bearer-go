package bearer_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	bearer "github.com/Bearer/bearer-go"
	"go.uber.org/zap"
)

func Example() {
	bearer.ReplaceGlobals(bearer.Init(os.Getenv("BEARER_SECRETKEY")))

	// perform request
	resp, err := http.Get("...")
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp)
}

func Example_advanced() {
	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agent := &bearer.Agent{
		SecretKey: os.Getenv("BEARER_SECRETKEY"),
		Logger:    logger,
		Transport: http.DefaultTransport,
		Context:   ctx,
	}
	defer agent.Flush()
	client := &http.Client{Transport: agent}

	// perform request
	resp, err := client.Get("...")
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp)
}
