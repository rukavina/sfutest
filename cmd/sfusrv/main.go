package main

import (
	"flag"
	"log"
	"net/http"
	"sfutest/pkg/sfu"
	"strconv"
	"time"

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

	m := webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.NewRTPVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	sfuEngine := sfu.NewEngine(api, rtcpPLIInterval, peerConnectionConfig)

	sfu.NewServer(sfuEngine, "../../static")

	log.Printf("SFU server up and running, open UI @ http://localhost:%d\n\n", *port)

	err := http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	if err != nil {
		panic(err)
	}
}
