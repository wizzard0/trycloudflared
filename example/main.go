package main

import (
	"context"
	"fmt"
	"github.com/wizzard0/trycloudflared"
	"net/http"
	"time"
)

func main() {
	fmt.Println("trycloudflared demo starting.")
	// todo random port blah blah?
	PORT := 8910
	go startHelloHttpServer(PORT)
	ctx, cancel := context.WithCancel(context.Background())
	url, err := trycloudflared.CreateCloudflareTunnel(ctx, PORT)
	if err != nil {
		panic(err)
	}
	fmt.Println("Embedded HTTP server is available at: " + url)
	select {
	case <-time.After(time.Second * 10):
		fmt.Println("10 seconds passed, graceful shutdown")
	}
	cancel()
	select {
	case <-time.After(time.Second * 100):
		fmt.Println("100 seconds passed, shutting down")
	}
}

func startHelloHttpServer(port int) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		timeString := time.Now().Format("2006-01-02 15:04:05")
		_, err := fmt.Fprintf(w, "Hello, world! Today it's "+timeString+" and this is served via Cloudflare Tunnel.")
		if err != nil {
			return
		}
	})
	_ = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
