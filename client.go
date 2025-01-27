package whepclient

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

var (
	ErrFailedToConnect    = errors.New("Failed to connect")
	ErrNoLocationHeader   = errors.New("No location header in the response")
	ErrFailedToDisconnect = errors.New("Failed to remove server resource")
)

type Client struct {
	Pc *webrtc.PeerConnection

	url      string
	pcConfig webrtc.Configuration
	location string
}

func New(url string, pcConfig webrtc.Configuration) (*Client, error) {
	interceptorRegistry := &interceptor.Registry{}
	mediaEngine := &webrtc.MediaEngine{}

	err := mediaEngine.RegisterDefaultCodecs()
	if err != nil {
		return nil, err
	}

	err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry)
	if err != nil {
		return nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry))

	pc, err := api.NewPeerConnection(pcConfig)
	if err != nil {
		return nil, err
	}

	client := &Client{
		Pc:       pc,
		url:      url,
		pcConfig: pcConfig,
	}

	return client, nil
}

func (client *Client) Connect() error {
	// add transceivers
	_, err := client.Pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		return err
	}

	_, err = client.Pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		return err
	}

	// create offer
	offer, err := client.Pc.CreateOffer(nil)
	if err != nil {
		return err
	} else if err = client.Pc.SetLocalDescription(offer); err != nil {
		return err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(client.Pc)

	// Block until ICE Gathering is complete, disabling trickle ICE
	<-gatherComplete

	resp, err := http.Post(client.url, "application/SDP", bytes.NewBufferString(client.Pc.LocalDescription().SDP))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		return ErrFailedToConnect
	}

	client.location = resp.Header.Get("location")
	if client.location == "" {
		return ErrNoLocationHeader
	}

	if err = client.Pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(body)}); err != nil {
		return err
	}

	return nil
}

func (client *Client) Disconnect() error {
	err := client.Pc.Close()
	if err != nil {
		// What should we do when Close returns an error?
		// Should this ever happen?
		return err
	}

	req, err := http.NewRequest("DELETE", client.location, nil)
	if err != nil {
		return ErrFailedToDisconnect
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ErrFailedToDisconnect
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ErrFailedToDisconnect
	}

	return nil
}
