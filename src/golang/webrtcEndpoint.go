package main

import "C"
import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/pkg/media"
	"math/rand"
	"unsafe"
)

var defaultStunServer = []string{"stun:stun.l.google.com:19302"}
var instance webrtcEndpoint

type webrtcEndpoint struct {
	videoTrack *webrtc.Track
	localBase64Offer string
}

func (endpoint *webrtcEndpoint) NewWebRtcEndpoint(base64offer string, stunsUrl []string)(string, error) {

	offer := webrtc.SessionDescription{}
	b, err := base64.StdEncoding.DecodeString(base64offer)
	if err != nil {
		return "", errors.New("Unable to decode base64 ")
	}

	err = json.Unmarshal(b, &offer)
	if err != nil {
		return "", errors.New("Unable to decode JSON offer")
	}
	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	err = mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		return "", errors.New("Unable to add offer to SDP description")
	}

	// Search for VP8 Payload type. If the offer doesn't support VP8 exit since
	// since they won't be able to decode anything we send them
	var payloadType uint8
	for _, videoCodec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo) {
		fmt.Printf("codec type", videoCodec.Name) // VP8 VP9 H264
		//if videoCodec.Name == "VP8" {
		//	payloadType = videoCodec.PayloadType
		//	break
		//}
		if videoCodec.Name == "H264" {
			payloadType = videoCodec.PayloadType
			break
		}
	}
	if payloadType == 0 {
		return "", errors.New("Remote peer doesnot support H264 or VP8 codec")
	}

	// Create a new RTCPeerConnection
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: stunsUrl,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Create a video track
	videoTrack, err := peerConnection.NewTrack(payloadType, rand.Uint32(), "video", "pion")
	endpoint.videoTrack = videoTrack
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}


	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser

	b, err = json.Marshal(answer)
	if err != nil {
		panic(err)
	}
	localOffer := base64.StdEncoding.EncodeToString(b)
	return localOffer, nil

}

func (endpoint *webrtcEndpoint) submitFrame(frame []byte) error {
	//fmt.Println(frame)
	return endpoint.videoTrack.WriteSample(media.Sample{Data: frame, Samples: 1})
}

//export NewWebrtc
func NewWebrtc(base64Offer *C.char) *C.char {
	offer := C.GoString(base64Offer);
	instance = webrtcEndpoint{}
	localOffer, err := instance.NewWebRtcEndpoint(offer, defaultStunServer)
	if err != nil {
		panic(err)
	}
	return C.CString(localOffer)
}

//export submitFrame
func submitFrame(frame *C.uchar, len C.int) C.int {
	buffer := unsafe.Pointer(frame)
	data := C.GoBytes(buffer, len);
	err := instance.submitFrame(data)
	if( err != nil) {
		return 1
	}
	return 0
}


func main() {}