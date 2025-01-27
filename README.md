# WHEP Client

A simple WHEP client.


# Example

```go
package main

import (
	"fmt"
	"os"

	whepclient "github.com/elixir-webrtc/whep-client"
	"github.com/pion/webrtc/v4"
)

func main() {
	url := os.Args[1]

	client, err := whepclient.New(url, webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	client.Pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	client.Pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("New track: %s\n", track.Codec().MimeType)
		for {
			_, _, err := track.ReadRTP()
			if err != nil {
				panic(err)
			}
		}
	})

	err = client.Connect()
	if err != nil {
		panic(err)
	}

	// block forever
	select {}
}
```

Then call:

```
go run main.go https://yourWhepEndpointURL
```