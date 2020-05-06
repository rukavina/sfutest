package sfu

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v2"
)

//Engine is in charge to manage webrtc peers and tracks
type Engine struct {
	api             *webrtc.API
	rtcpPLIInterval time.Duration
	pcConfig        webrtc.Configuration

	publishPC  *webrtc.PeerConnection
	localTrack *webrtc.Track
}

//NewEngine Creates New Engine
func NewEngine(api *webrtc.API, rtcpPLIInterval time.Duration, pcConfig webrtc.Configuration) *Engine {
	return &Engine{
		api:             api,
		rtcpPLIInterval: rtcpPLIInterval,
		pcConfig:        pcConfig,
	}
}

func (s *Engine) createPublisherPC(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	answer := webrtc.SessionDescription{}
	if s.publishPC != nil && s.publishPC.ConnectionState() == webrtc.PeerConnectionStateConnected {
		return answer, fmt.Errorf("Publisher already exists")
	}
	// Create a new RTCPeerConnection
	var err error
	s.publishPC, err = s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return answer, fmt.Errorf("Error creating publisher PC: %v", err)
	}

	s.publishPC.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Publisher Peer conn. state change: %s\n", state.String())
	})

	s.publishPC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Publisher ICE conn. state change: %s\n", state.String())
	})

	// Allow us to receive 1 video track
	if _, err = s.publishPC.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return answer, fmt.Errorf("Error adding publisher Transceiver: %v", err)
	}

	// Set the remote SessionDescription
	err = s.publishPC.SetRemoteDescription(offer)
	if err != nil {
		return answer, fmt.Errorf("Error setting publisher remote desc.: %v", err)
	}

	// Create answer
	answer, err = s.publishPC.CreateAnswer(nil)
	if err != nil {
		return answer, fmt.Errorf("Error creating publisher answer: %v", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = s.publishPC.SetLocalDescription(answer)
	if err != nil {
		return answer, fmt.Errorf("Error setting publisher local desc.: %v", err)
	}

	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	s.publishPC.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(s.rtcpPLIInterval)
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
			if s.publishPC == nil || s.publishPC.ConnectionState() == webrtc.PeerConnectionStateClosed || s.publishPC.ConnectionState() == webrtc.PeerConnectionStateDisconnected {
				break
			}

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
	return answer, nil
}

func (s *Engine) createViewerPC(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	answer := webrtc.SessionDescription{}
	if s.publishPC == nil {
		return answer, fmt.Errorf("Publisher does not exist yet")
	}
	// Create a new PeerConnection
	viewerPC, err := s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return answer, fmt.Errorf("Error creating viewer PC: %v", err)
	}

	_, err = viewerPC.AddTrack(s.localTrack)
	if err != nil {
		return answer, fmt.Errorf("Error adding track: %v", err)
	}

	// Set the remote SessionDescription
	err = viewerPC.SetRemoteDescription(offer)
	if err != nil {
		return answer, fmt.Errorf("Error setting remote desc: %v", err)
	}

	// Create answer
	answer, err = viewerPC.CreateAnswer(nil)
	if err != nil {
		return answer, fmt.Errorf("Error creating viewer answer: %v", err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = viewerPC.SetLocalDescription(answer)
	if err != nil {
		return answer, fmt.Errorf("Error setting local desc: %v", err)
	}

	log.Println("Viewer PeerConnection created and answer returned")
	return answer, nil
}
