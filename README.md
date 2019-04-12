# SFU Test
SFU WEBRTC test based on https://github.com/pion/webrtc https://github.com/pion/webrtc/tree/master/examples/sfu-minimal

## Installation

`go get github.com/rukavina/sfutest`

## Run

```bash
cd cmd/sfusrv
go run main.go
```

Then in browser open http://localhost:8080 and click `Publish` to connect broadcaster first and then `Start`, and in another tab the same page and click `Join` and `Start`
You should connect 1 broadcaster and many viewers. The limitation currently is that you need to connect 