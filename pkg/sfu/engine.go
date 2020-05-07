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
	publishConns    *ConnectionsMap
}

//NewEngine Creates New Engine
func NewEngine(api *webrtc.API, rtcpPLIInterval time.Duration, pcConfig webrtc.Configuration) *Engine {
	return &Engine{
		api:             api,
		rtcpPLIInterval: rtcpPLIInterval,
		pcConfig:        pcConfig,
		publishConns:    NewConnectionsMap(),
	}
}

func (s *Engine) publisherConn(publisherKey string) (*webrtc.PeerConnection, bool) {
	conn, ok := s.publishConns.Load(publisherKey)
	if ok && conn.peerConnection.ConnectionState() == webrtc.PeerConnectionStateConnected {
		return conn.peerConnection, ok
	}
	return nil, false
}

func (s *Engine) localTrack(publisherKey string) (*webrtc.Track, bool) {
	conn, ok := s.publishConns.Load(publisherKey)
	if ok && conn.localTrack != nil {
		return conn.localTrack, ok
	}
	return nil, false
}

func (s *Engine) createPublisherPC(publisherKey string, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	answer := webrtc.SessionDescription{}
	_, ok := s.publisherConn(publisherKey)
	if ok {
		return answer, fmt.Errorf("Publisher already exists for key %q", publisherKey)
	}
	// Create a new RTCPeerConnection
	var err error
	pc, err := s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return answer, fmt.Errorf("Error creating publisher PC: %v", err)
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Publisher %q Peer conn. state change: %s\n", publisherKey, state)
		if state == webrtc.PeerConnectionStateDisconnected {
			s.publishConns.Delete(publisherKey)
			log.Printf("Publisher %q Peer conn. removed\n", publisherKey)
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Publisher %q ICE conn. state change: %s\n", publisherKey, state)
	})

	// Allow us to receive 1 video track
	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		return answer, fmt.Errorf("Error adding publisher %q Transceiver: %v", publisherKey, err)
	}

	// Set the remote SessionDescription
	err = pc.SetRemoteDescription(offer)
	if err != nil {
		return answer, fmt.Errorf("Error setting publisher %q remote desc.: %v", publisherKey, err)
	}

	// Create answer
	answer, err = pc.CreateAnswer(nil)
	if err != nil {
		return answer, fmt.Errorf("Error creating publisher %q answer: %v", publisherKey, err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = pc.SetLocalDescription(answer)
	if err != nil {
		return answer, fmt.Errorf("Error setting publisher %q local desc.: %v", publisherKey, err)
	}

	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	pc.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(s.rtcpPLIInterval)
			for range ticker.C {
				if rtcpSendErr := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
					log.Printf("RTCP %q err: %v\n", publisherKey, rtcpSendErr)
				}
			}
		}()

		// Create a local track, all our SFU clients will be fed via this track
		localTrack, err := pc.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
		if err != nil {
			log.Printf("Publisher %q error creating new track: %v", publisherKey, err)
			return
		}
		s.publishConns.Store(publisherKey, &Connection{
			peerConnection: pc,
			localTrack:     localTrack,
		})
		log.Printf("Publisher %q New remote track and its local track created", publisherKey)

		rtpBuf := make([]byte, 1400)
		for {
			if pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
				log.Printf("Peer connection %q not connected, but in status: %q, breaking the loop", publisherKey, pc.ConnectionState())
				break
			}

			i, err := remoteTrack.Read(rtpBuf)
			if err != nil {
				log.Printf("remote track %q read error: %v", publisherKey, err)
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
				log.Printf("local track %q write error: %v", publisherKey, err)
			}
		}
	})

	s.publishConns.Store(publisherKey, &Connection{
		peerConnection: pc,
	})
	log.Printf("Publisher PC %q created and answer returned", publisherKey)
	return answer, nil
}

func sender(pc *webrtc.PeerConnection) *webrtc.RTPSender {
	senders := pc.GetSenders()
	log.Printf("found %d senders on the peer connection", len(senders))
	if len(senders) == 0 {
		return nil
	}
	return senders[0]
}

func receiver(pc *webrtc.PeerConnection) *webrtc.RTPReceiver {
	receivers := pc.GetReceivers()
	log.Printf("found %d receivers on the peer connection", len(receivers))
	if len(receivers) == 0 {
		return nil
	}
	return receivers[0]
}

func trackFromSender(pc *webrtc.PeerConnection) *webrtc.Track {
	sender := sender(pc)
	if sender == nil {
		return nil
	}
	return sender.Track()
}

func trackFromReceiver(pc *webrtc.PeerConnection) *webrtc.Track {
	receiver := receiver(pc)
	if receiver == nil {
		return nil
	}
	return receiver.Track()
}

func (s *Engine) createViewerPC(publisherKey string, offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	answer := webrtc.SessionDescription{}
	localTrack, ok := s.localTrack(publisherKey)
	if !ok {
		return answer, fmt.Errorf("Publisher %q does not exist yet", publisherKey)
	}
	// Create a new PeerConnection
	viewerPC, err := s.api.NewPeerConnection(s.pcConfig)
	if err != nil {
		return answer, fmt.Errorf("Error creating viewer %q PC: %v", publisherKey, err)
	}
	_, err = viewerPC.AddTrack(localTrack)
	if err != nil {
		return answer, fmt.Errorf("Error adding track %q: %v", publisherKey, err)
	}

	// Set the remote SessionDescription
	err = viewerPC.SetRemoteDescription(offer)
	if err != nil {
		return answer, fmt.Errorf("Error setting remote desc %q: %v", publisherKey, err)
	}

	// Create answer
	answer, err = viewerPC.CreateAnswer(nil)
	if err != nil {
		return answer, fmt.Errorf("Error creating viewer answer %q: %v", publisherKey, err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = viewerPC.SetLocalDescription(answer)
	if err != nil {
		return answer, fmt.Errorf("Error setting local desc %q: %v", publisherKey, err)
	}

	log.Printf("Viewer PeerConnection for %q created and answer returned", publisherKey)
	return answer, nil
}
