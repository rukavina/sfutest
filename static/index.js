class SessionManager {

  constructor(videoEl, logEl, signalingUrlEl, onConnected = null) {
    this.videoEl = videoEl
    this.logEl = logEl
    this.signalingUrlEl = signalingUrlEl
    this.pc = new RTCPeerConnection({
      iceServers: [
        {
          urls: 'stun:stun.l.google.com:19302'
        }
      ]
    })
    this.pc.oniceconnectionstatechange = e => this.log(this.pc.iceConnectionState)
    this.onConnected = onConnected
  }

  broadcast(publisherKey) {
    let self = this
    navigator.mediaDevices.getUserMedia({ video: true, audio: false })
      .then(stream => {
        self.pc.addStream(self.videoEl.srcObject = stream)
        self.offerSDP(true, publisherKey)
      }).catch(err => self.log(err))    
  }

  watch(publisherKey) {
    let self = this
    this.pc.addTransceiver('video', { 'direction': 'recvonly' })
    this.offerSDP(false, publisherKey)
    this.pc.ontrack = function (event) {
      self.videoEl.srcObject = event.streams[0]
      self.videoEl.autoplay = true
      self.videoEl.controls = true
    }    
  }

  log(msg) {
    console.log(msg)
    this.logEl.innerHTML += msg + "\n"
  }

  postSDP(publisherKey, sdp, isPublisher, url) {
    let req = {
      'sdp': sdp,
      'mode': isPublisher ? 'publisher' : 'viewer',
      'publisherKey': publisherKey,
    }
    return fetch(url, {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        "Content-Type": "application/json",
      },
    })
  }  

  offerSDP(isPublisher, publisherKey) {
    let self = this
    this.pc.createOffer()
      .then(function (sdp) {
        self.pc.setLocalDescription(sdp)
        self.postSDP(publisherKey, sdp, isPublisher, self.signalingUrlEl.value).then(res => res.json())
          .then(response => {
            console.log("response:", response)
            if (response.success) {
              try {
                self.pc.setRemoteDescription(new RTCSessionDescription(response.sdp))
                if(self.onConnected){
                  self.onConnected()
                }
              } catch (e) {
                alert(e)
              }              
            } else {
              alert("Error connecting: " + response.error)
            }
          })
          .catch(error => console.error('Error:', error));
      })
      .catch(err => self.log(err))
  }  


}