package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pion/interceptor"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

type PcConfig struct {
	IceServers         []map[string]string
	IceTransportPolicy webrtc.ICETransportPolicy
}

func main() {
	pcConfig := PcConfig{}
	// url := "https://global.broadcaster.stunner.cc"
	url := "http://localhost:4000"
	resp, err := http.Get(url + "/api/pc-config")

	if err != nil {
		panic("Couldn't get peer connection config")
	}

	json.NewDecoder(resp.Body).Decode(&pcConfig)
	if err != nil {
		panic("Couldn't read response body")
	}

	pionPcConfig := webrtc.Configuration{}

	for i := 0; i < len(pcConfig.IceServers); i++ {
		iceServer := pcConfig.IceServers[i]
		pionIceServer := webrtc.ICEServer{}
		pionIceServer.URLs = []string{iceServer["urls"]}
		pionIceServer.Username = iceServer["username"]
		pionIceServer.Credential = iceServer["credential"]
		pionIceServer.CredentialType = webrtc.ICECredentialTypePassword
		pionPcConfig.ICEServers = append(pionPcConfig.ICEServers, pionIceServer)
	}

	pionPcConfig.ICETransportPolicy = pcConfig.IceTransportPolicy

	// create peer connection

	// we don't want to use simulcast interceptors
	interceptorRegistry := &interceptor.Registry{}
	mediaEngine := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// We'll only use H264 but you can also define your own
	if err = mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
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

	peerConnection, err := api.NewPeerConnection(pionPcConfig)
	if err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("New track: %s", track.Codec().MimeType)
		for {
			// do we need to call this if we ignor read packets anyway?
			_, _, err := track.ReadRTP()
			if err != nil {
				panic(err)
			}
		}
	})

	// add transceivers
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		panic(err)
	}

	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		panic(err)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	// go func() {
	// 	rtcpBuf := make([]byte, 1500)
	// 	for {
	// 		if _, _, rtcpErr := audioTr.Receiver().Read(rtcpBuf); rtcpErr != nil {
	// 			return
	// 		}
	// 	}
	// }()

	// go func() {
	// 	rtcpBuf := make([]byte, 1500)
	// 	for {
	// 		if _, _, rtcpErr := videoTr.Receiver().Read(rtcpBuf); rtcpErr != nil {
	// 			return
	// 		}
	// 	}
	// }()

	// create offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	fmt.Println(peerConnection.LocalDescription().SDP)

	resp, err = http.Post(url+"/api/whep", "application/SDP", bytes.NewBufferString(peerConnection.LocalDescription().SDP))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(body))

	if err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(body)}); err != nil {
		panic(err)
	}

	// block forever
	select {}
}
