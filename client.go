package whepclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/pion/interceptor"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	Pc *webrtc.PeerConnection

	url      string
	pcConfig webrtc.Configuration
}

func New(url string, pcConfig webrtc.Configuration) (Client, error) {
	// create peer connection

	// we don't want to use simulcast interceptors
	interceptorRegistry := &interceptor.Registry{}
	mediaEngine := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// We'll only use H264 but you can also define your own
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	if err := webrtc.ConfigureNack(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	if err := webrtc.ConfigureRTCPReports(interceptorRegistry); err != nil {
		panic(err)
	}

	if err := webrtc.ConfigureTWCCSender(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	if err := mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.SDESMidURI}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	if err := mediaEngine.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.SDESMidURI}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))

	pc, err := api.NewPeerConnection(pcConfig)
	if err != nil {
		panic(err)
	}

	client := Client{
		url:      url,
		Pc:       pc,
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
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	fmt.Println(client.Pc.LocalDescription().SDP)

	resp, err := http.Post(client.url+"/api/whep", "application/SDP", bytes.NewBufferString(client.Pc.LocalDescription().SDP))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(body))

	if err = client.Pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(body)}); err != nil {
		panic(err)
	}
}
