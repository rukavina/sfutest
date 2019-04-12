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

	offerChan, answerChan := signal.HTTPSDPServer(*port, "../../static")

	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	sfu := &SFU{
		answerChan: answerChan,
		api:        api,
		pcConfig:   peerConnectionConfig,
	}

	offer := webrtc.SessionDescription{}
	log.Printf("SDP srv up and running @ http://localhost:%d, waiting for an offer\n\n", *port)
	signal.Decode(<-offerChan, &offer)
	log.Println("SDP offer received")

	err := sfu.createPublisherPC(offer)
	if err != nil {
		log.Printf("Error creating Publisher peer connection %v", err)
	}

	log.Println("Waiting for new local track")

	for {
		fmt.Println("")
		fmt.Println("Curl an base64 SDP to start sendonly peer connection")

		recvOnlyOffer := webrtc.SessionDescription{}

		log.Println("Waiting for new receive only sdp offer")
		signal.Decode(<-offerChan, &recvOnlyOffer)
		err = sfu.createViewerPC(recvOnlyOffer)
		if err != nil {
			log.Printf("Error creating Publisher peer connection %v", err)
		}
	}
}

type SFU struct {
	api        *webrtc.API
	publishPC  *webrtc.PeerConnection
	pcConfig   webrtc.Configuration
	answerChan chan string
}

func (s *SFU) createPublisherPC(offer webrtc.SessionDescription) error {
	// Create a new RTCPeerConnection
	var err error
	s.publishPC, err = s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return err
	}

	// Allow us to receive 1 video track
	if _, err = s.publishPC.AddTransceiver(webrtc.RTPCodecTypeVideo); err != nil {
		return err
	}

	log.Println("New Publisher PeerConnection created and added Transceiver")

	// Set the remote SessionDescription
	err = s.publishPC.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	// Create answer
	answer, err := s.publishPC.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = s.publishPC.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	log.Println("peerConnection offer accepted and answer created")
	s.answerChan <- signal.Encode(answer)

	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	s.publishPC.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for range ticker.C {
				if rtcpSendErr := s.publishPC.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
					fmt.Println(rtcpSendErr)
				}
			}
		}()

		// Create a local track, all our SFU clients will be fed via this track
		localTrack, err := s.publishPC.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
		if err != nil {
			log.Printf("On track error creating new track %v", err)
		}
		s.localTrackChan <- localTrack

		log.Println("New remote track and its local track created")

		rtpBuf := make([]byte, 1400)
		for {
			i, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				panic(readErr)
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
				panic(err)
			}
		}
	})

	return nil
}

func (s *SFU) createViewerPC(offer webrtc.SessionDescription) error {
	// Create a new PeerConnection
	viewerPC, err := s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return err
	}

	_, err = viewerPC.AddTrack(localTrack)
	if err != nil {
		return err
	}

	// Set the remote SessionDescription
	err = viewerPC.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	// Create answer
	answer, err := viewerPC.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = viewerPC.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	log.Println("Offer received, new PeerConnection created, localTrack added and answer created")
	s.answerChan <- signal.Encode(answer)
	return nil
}
