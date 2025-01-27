package whepclient

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pion/webrtc/v4"
)

func TestNew(t *testing.T) {
	_, err := New("", webrtc.Configuration{})
	if err != nil {
		t.Fatalf("New returned an error %v", err)
	}
}

func TestConnect(t *testing.T) {
	ts := newWHEPMockServer()
	defer ts.Close()

	client, err := New(ts.URL, webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Connect()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisconnect(t *testing.T) {
	ts := newWHEPMockServer()
	defer ts.Close()

	client, err := New(ts.URL, webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Connect()
	if err != nil {
		t.Fatal(err)
	}

	err = client.Disconnect()
	if err != nil {
		t.Fatal(err)
	}
}

func newWHEPMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			handleSDPOfferRequest(w, r)
		} else if r.Method == "DELETE" {
			handleTerminateRequest(w, r)
		}
	}))
}

func handleSDPOfferRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		panic(fmt.Sprintf("SDP offer has to be a POST request, got: %s\n", r.Method))
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/SDP" {
		panic(fmt.Sprintf("Content-Type has to be application/SDP, got: %s\n", contentType))
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	resourceId := generateResourceId(8)
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	err = pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: string(body)})
	if err != nil {
		panic(err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	w.Header().Add("location", "http://"+r.Host+"/resource/"+resourceId)
	w.Header().Add("Content-Type", "application/SDP")
	w.Header().Add("Status Code", "201")
	w.WriteHeader(http.StatusCreated)
	sent, err := w.Write([]byte(answer.SDP))
	if err != nil {
		panic(err)
	}

	if sent != len(answer.SDP) {
		panic(fmt.Sprintf("Could not write entire answer in response. Answer length: %v, wrote: %v", len(answer.SDP), sent))
	}
}

func handleTerminateRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		panic(fmt.Sprintf("Terminate session has to be a DELETE request, got: %s\n", r.Method))
	}

	w.WriteHeader(http.StatusOK)
}

func generateResourceId(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
