package signal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

// HTTPSDPServer starts a HTTP Server that consumes SDPs
func HTTPSDPServer(port int, staticDir string) (offerChan, answerChan chan string) {
	offerChan = make(chan string)
	answerChan = make(chan string)
	http.HandleFunc("/sdp", func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		offerChan <- string(body)
		answer := <-answerChan
		fmt.Fprintf(w, answer)
	})

	//server static files
	if staticDir != "" {
		fs := http.FileServer(http.Dir(staticDir))
		http.Handle("/", fs)
	}

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
		if err != nil {
			panic(err)
		}
	}()

	return
}
