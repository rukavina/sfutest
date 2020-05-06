# SFU Test
SFU WEBRTC test based on https://github.com/pion/webrtc https://github.com/pion/webrtc/tree/master/examples/sfu-minimal

## Installation

`go get github.com/rukavina/sfutest`

## Run

```bash
cd cmd/sfusrv
go run main.go
```

Then in browser open http://localhost:9090 and click `Publish a Broadcast` to connect broadcaster first, and then in another tab the same page and click `Join a Broadcast`
You should connect 1 broadcaster and many viewers. 