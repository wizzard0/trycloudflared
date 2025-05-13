# trycloudflared

zero-configuration library to use Cloudflare Tunnel in your Go application without deploying a separate service.

Note that this ties you to a particular version of cloudflared, so you may want to use a separate service in production in case they break the API.

as seen in: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/do-more-with-tunnels/trycloudflare/#use-trycloudflare

```sh
cloudflared tunnel --url http://localhost:12345
```

equivalent code if you want to use it in your Go application:

```go
package main

import (
	"fmt"
	"context"
	"github.com/wizzard0/trycloudflared"
)

func main() {
	// start your http server on port 12345
	// e.g. http.ListenAndServe(":12345", nil)
	// ...
	ctx, cancel := context.WithCancel(context.Background())
	
	// this will expose localhost:12345 via https://something.trycloudflare.com
	url, err := trycloudflared.CreateCloudflareTunnel(ctx, 12345)
	if err != nil {
		panic(err)
	}

	// url is the URL of the tunnel
	fmt.Println(url)
	// do something with the tunnel
	// ...
	// cancel the tunnel when done
	cancel()
}
```