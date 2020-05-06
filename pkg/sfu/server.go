package sfu

import (
	"encoding/json"
	"io/ioutil"
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
	SDP  webrtc.SessionDescription `json:"sdp"`
	Mode string                    `json:"mode"`
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	offer := req.SDP
	var answer webrtc.SessionDescription
	if req.Mode == RequestModePublisher {
		answer, err = s.Engine.createPublisherPC(offer)
	} else {
		answer, err = s.Engine.createViewerPC(offer)
	}

	res := SDPResponse{
		SDP:     answer,
		Success: err == nil,
	}
	if err != nil {
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
