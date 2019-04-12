package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"sfutest/pkg/signal"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

const (
	rtcpPLIInterval = time.Second * 3
)

func main() {
	port := flag.Int("port", 8080, "http server port")
	flag.Parse()

	offerChan, answerChan := signal.HTTPSDPServer(*port, "../../static")

	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	sfu := &SFU{
		api:      api,
		pcConfig: peerConnectionConfig,
	}

	offer := webrtc.SessionDescription{}
	log.Printf("SDP srv up and running @ http://localhost:%d, waiting for an offer\n\n", *port)
	signal.Decode(<-offerChan, &offer)
	log.Println("SDP offer received")

	answer, err := sfu.createPublisherPC(offer)
	if err != nil {
		log.Printf("Error creating Publisher peer connection %v", err)
	}
	answerChan <- answer

	for {
		recvOnlyOffer := webrtc.SessionDescription{}
		log.Println("Waiting for new receive only sdp offer")
		signal.Decode(<-offerChan, &recvOnlyOffer)
		answer, err = sfu.createViewerPC(recvOnlyOffer)
		if err != nil {
			log.Printf("Error creating Viewer peer connection %v", err)
		}
		answerChan <- answer
	}
}

type SFU struct {
	api        *webrtc.API
	publishPC  *webrtc.PeerConnection
	pcConfig   webrtc.Configuration
	localTrack *webrtc.Track
}

func (s *SFU) createPublisherPC(offer webrtc.SessionDescription) (string, error) {
	if s.publishPC != nil {
		return "", fmt.Errorf("Publisher already exists")
	}
	// Create a new RTCPeerConnection
	var err error
	s.publishPC, err = s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return "", fmt.Errorf("Error creating publisher PC: %v", err)
	}

	// Allow us to receive 1 video track
	if _, err = s.publishPC.AddTransceiver(webrtc.RTPCodecTypeVideo); err != nil {
		return "", fmt.Errorf("Error adding publisher Transceiver: %v", err)
	}

	// Set the remote SessionDescription
	err = s.publishPC.SetRemoteDescription(offer)
	if err != nil {
		return "", fmt.Errorf("Error setting publisher remote desc.: %v", err)
	}

	// Create answer
	answer, err := s.publishPC.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("Error creating publisher answer: %v", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = s.publishPC.SetLocalDescription(answer)
	if err != nil {
		return "", fmt.Errorf("Error setting publisher local desc.: %v", err)
	}

	s.publishPC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Publisher ICE conn. state change: %s\n", state.String())
		if state == webrtc.ICEConnectionStateDisconnected {
			s.publishPC.Close()
			s.publishPC = nil
			s.localTrack = nil
		}
	})

	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	s.publishPC.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for range ticker.C {
				if rtcpSendErr := s.publishPC.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
					log.Printf("RTCP err: %v\n", rtcpSendErr)
				}
			}
		}()

		// Create a local track, all our SFU clients will be fed via this track
		s.localTrack, err = s.publishPC.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
		if err != nil {
			log.Printf("Publisher error creating new track: %v", err)
		}

		log.Println("Publisher New remote track and its local track created")

		rtpBuf := make([]byte, 1400)
		for {
			i, err := remoteTrack.Read(rtpBuf)
			if err != nil {
				log.Printf("remote track read error: %v", err)
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = s.localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
				log.Printf("local track write error: %v", err)
			}
		}
	})

	log.Println("Publisher PC created and answer returned")
	return signal.Encode(answer), nil
}

func (s *SFU) createViewerPC(offer webrtc.SessionDescription) (string, error) {
	if s.publishPC == nil {
		return "", fmt.Errorf("Publisher does not exist yet")
	}
	// Create a new PeerConnection
	viewerPC, err := s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return "", fmt.Errorf("Error creating viewer PC: %v", err)
	}

	_, err = viewerPC.AddTrack(s.localTrack)
	if err != nil {
		return "", fmt.Errorf("Error adding track: %v", err)
	}

	// Set the remote SessionDescription
	err = viewerPC.SetRemoteDescription(offer)
	if err != nil {
		return "", fmt.Errorf("Error setting remote desc: %v", err)
	}

	// Create answer
	answer, err := viewerPC.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("Error creating viewer answer: %v", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = viewerPC.SetLocalDescription(answer)
	if err != nil {
		return "", fmt.Errorf("Error setting local desc: %v", err)
	}

	log.Println("Viewer PeerConnection created and answer returned")
	return signal.Encode(answer), nil
}
