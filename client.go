package whepclient

import (
	"bytes"
	"io"
	"net/http"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	Pc *webrtc.PeerConnection

	url      string
	pcConfig webrtc.Configuration
}

func New(url string, pcConfig webrtc.Configuration) (Client, error) {
	// create peer connection
	interceptorRegistry := &interceptor.Registry{}
	mediaEngine := &webrtc.MediaEngine{}

	err := mediaEngine.RegisterDefaultCodecs()
	if err != nil {
		panic(err)
	}

	err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry)
	if err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))

	pc, err := api.NewPeerConnection(pcConfig)
	if err != nil {
		panic(err)
	}

	client := Client{
		Pc:       pc,
		url:      url,
		pcConfig: pcConfig,
	}

	return client, nil
}

func (client *Client) Connect() {
	// add transceivers
	_, err := client.Pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		panic(err)
	}

	_, err = client.Pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		panic(err)
	}

	// create offer
	offer, err := client.Pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	} else if err = client.Pc.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(client.Pc)

	// Block until ICE Gathering is complete, disabling trickle ICE
	<-gatherComplete

	resp, err := http.Post(client.url+"/api/whep", "application/SDP", bytes.NewBufferString(client.Pc.LocalDescription().SDP))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	if err = client.Pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(body)}); err != nil {
		panic(err)
	}
}
