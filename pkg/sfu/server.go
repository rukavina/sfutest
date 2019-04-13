package sfu

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pion/webrtc/v2"
)

//Server is http server and engine wrapper
type Server struct {
	port      int
	staticDir string
	engine    *Engine
}

//request modes consts
const (
	RequestModePublisher = "publisher"
	RequestModeViewer    = "viewer"
)

//SDPRequest is json http request
type SDPRequest struct {
	SDP  string `json:"sdp"`
	Mode string `json:"mode"`
}

//SDPResponse is json http response
type SDPResponse struct {
	SDP     string `json:"sdp"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// NewServer starts a HTTP Server that consumes SDPs
func NewServer(engine *Engine, staticDir string) *Server {
	s := &Server{
		engine:    engine,
		staticDir: staticDir,
	}

	http.HandleFunc("/sdp", func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var req SDPRequest
		err := json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		offer := webrtc.SessionDescription{}
		SDPDecode(req.SDP, &offer)
		var answer webrtc.SessionDescription
		if req.Mode == RequestModePublisher {
			answer, err = s.engine.createPublisherPC(offer)
		} else {
			answer, err = s.engine.createViewerPC(offer)
		}

		res := SDPResponse{
			SDP:     SDPEncode(&answer),
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
	})

	//server static files
	if staticDir != "" {
		fs := http.FileServer(http.Dir(staticDir))
		http.Handle("/", fs)
	}
	return s
}
