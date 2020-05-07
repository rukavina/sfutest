package sfu

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pion/webrtc/v2"
)

//request modes consts
const (
	RequestModePublisher = "publisher"
	RequestModeViewer    = "viewer"
)

//SDPRequest is json http request
type SDPRequest struct {
	SDP          webrtc.SessionDescription `json:"sdp"`
	Mode         string                    `json:"mode"`
	PublisherKey string                    `json:"publisherKey"`
}

//SDPResponse is json http response
type SDPResponse struct {
	SDP     webrtc.SessionDescription `json:"sdp"`
	Success bool                      `json:"success"`
	Error   string                    `json:"error"`
}

//Server is http server and engine wrapper
type Server struct {
	Engine *Engine
}

//HandleSDP is sdp endpoint to receive publisher and viewer sdp offers and return answer
func (s *Server) HandleSDP(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	var req SDPRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("Invalid request payload: %s", body)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("New SDP request mode: %q, key: %q", req.Mode, req.PublisherKey)
	offer := req.SDP
	var answer webrtc.SessionDescription
	if req.Mode == RequestModePublisher {
		answer, err = s.Engine.createPublisherPC(req.PublisherKey, offer)
	} else {
		answer, err = s.Engine.createViewerPC(req.PublisherKey, offer)
	}

	res := SDPResponse{
		SDP:     answer,
		Success: err == nil,
	}
	if err != nil {
		log.Printf("Response error: %+v", err)
		res.Error = err.Error()
	}

	js, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
