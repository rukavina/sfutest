/* eslint-env browser */
var log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

window.createSession = isPublisher => {
  let pc = new RTCPeerConnection({
    iceServers: [
      {
        urls: 'stun:stun.l.google.com:19302'
      }
    ]
  })
  pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
  pc.onicecandidate = event => {
    if (event.candidate === null) {
      let sdp = btoa(JSON.stringify(pc.localDescription))
      document.getElementById('localSessionDescription').value = sdp
      postSDP(sdp).then(response => {
        console.log(response)
        response.text().then(text => document.getElementById('remoteSessionDescription').value = text);        
      })
    }
  }

  if (isPublisher) {
    navigator.mediaDevices.getUserMedia({ video: true, audio: false })
      .then(stream => {
        pc.addStream(document.getElementById('video1').srcObject = stream)
        pc.createOffer()
          .then(d => pc.setLocalDescription(d))
          .catch(log)
      }).catch(log)
  } else {
    pc.addTransceiver('video', {'direction': 'recvonly'})
    pc.createOffer()
      .then(d => pc.setLocalDescription(d))
      .catch(log)

    pc.ontrack = function (event) {
      var el = document.getElementById('video1')
      el.srcObject = event.streams[0]
      el.autoplay = true
      el.controls = true
    }
  }

  window.startSession = () => {
    let sd = document.getElementById('remoteSessionDescription').value
    if (sd === '') {
      return alert('Session Description must not be empty')
    }

    try {
      pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
    } catch (e) {
      alert(e)
    }
  }

  let btns = document.getElementsByClassName('createSessionButton')
  for (let i = 0; i < btns.length; i++) {
    btns[i].style = 'display: none'
  }

  document.getElementById('signalingContainer').style = 'display: block'
}


function postSDP(sdp, url = `http://localhost:8080/sdp`) {
  // Default options are marked with *
    return fetch(url, {
        method: 'POST',
        body: sdp, // body data type must match "Content-Type" header
    })
}