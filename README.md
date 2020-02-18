# Bearer Golang

:smile: Bearer API Client for Golang

[![GoDoc](https://godoc.org/github.com/Bearer/bearer-go?status.svg)](https://godoc.org/github.com/Bearer/bearer-go)
[![License](https://img.shields.io/badge/license-Apache--2.0-%2397ca00.svg)](https://github.com/Bearer/bearer-go/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Bearer/bearer-go)](https://goreportcard.com/report/github.com/Bearer/bearer-go)
[![GolangCI](https://golangci.com/badges/github.com/Bearer/bearer-go.svg)](https://golangci.com/r/github.com/Bearer/bearer-go)

## Installation

```console
go get github.com/Bearer/bearer-go
```

## Usage

Get your Bearer [Secret Key](https://app.bearer.sh/keys) and integration ID from the [Dashboard](https://app.bearer.sh/) and use the Bearer client as follows:

```golang
import "github.com/Bearer/bearer-go"

func main() {
        // configure the default HTTP client to use Bearer
        bearer.ReplaceGlobals(bearer.Init(os.Getenv("BEARER_SECRETKEY")))

        // then use your app normally:
        resp, _ := http.Get("...")
        fmt.Println("response: ", resp)
}

```

See more documentation and examples on [GoDoc](https://godoc.org/github.com/Bearer/bearer-go)

## Development

```console
# test
$ go test -v ./... -race

# lint
$ golint ./...
```
