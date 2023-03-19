import Head from 'next/head'
import Image from 'next/image'
import { Inter } from 'next/font/google'
import styles from '@/styles/Home.module.css'
import React, { useState, useEffect, useRef } from 'react';

// const inter = Inter({ subsets: ['latin'] })


export default function Home() {

  const ws = useRef()
  const subParent = useRef()
  const pubParent = useRef()
  const pc = useRef()
  let userid;
  let pcsub; 
  let ae; 
  let ve;

  useEffect(() => {
    console.log('useEffect: page loading')
    subParent.current = document.getElementById("sub")
    pubParent.current = document.getElementById("pub")
    pc.current = new RTCPeerConnection()

    const WIDTH = 640;
    const HEIGHT = 320;


    userid = 0;
    pcsub = new Map();
    ae = new Map();
    ve = new Map();

  }, [])




  function legacy() {
    console.log('legacy()')
    if (typeof window !== 'undefined'){
      document.getElementById("menu").style.display = 'none';
      ws.current = startws()
    }
  }
  function selectFlexatar(pth) {
    console.log('selectFlexatar()')
    // todo: proper pae to select ftar 
    if (typeof window !== 'undefined'){
      document.getElementById("menu").style.display = 'none';
      ws.current = startws(pth)

    }
  }
  
  function startws(ftar) {
    console.log("startws()")
    var loc = window.location,
    uri;
  
    if (loc.protocol === "https:") {
      uri = "wss:";
    } else {
      uri = "ws:";
    }
    uri += "//" + loc.host + "/ws"; // + "?ftar=1234";
  
    if (ftar != null) uri += "?ftar=" + ftar;
  
    ws.current = new WebSocket(uri);
    ws.current.onopen = function (event) {
  
      // remove own loopback
      if (ae.has(userid)) {
        pubParent.current.removeChild(ae.get(userid));
        ae.delete(userid);
      }
      if (ve.has(userid)) {
        pubParent.current.removeChild(ve.get(userid));
        ve.delete(userid);
      }
  
      // remove own publisher
      // pubParent.innerHTML = '';
  
      pcsub = new Map();
      userid = 0;
  
      pub();
    }
  
    ws.current.onclose = function (e) {
      ve.forEach(hide);
      setTimeout(function () {
        startws();
      }, 1000);
    };
  
    ws.current.onmessage = function (event) {
      var data = event.data;
      var pl = JSON.parse(data);
  
      switch (pl.action) {
        case "pub":
          pub(pl.id, pl.data)
          return
        case "sub":
          sub(pl.id, pl.data)
          return
        case "inv":
          inv(pl.id)
          return
        case "del":
          del(pl.id)
          return
        default:
          console.log("unexpected", pl.action)
      }
    }
    return ws.current;
  }
  
  function del(id) {
    if (pcsub.has(id)) {
      pcsub.delete(id);
      if (id == userid) {
        pubParent.current.removeChild(ae.get(id));
        pubParent.current.removeChild(ve.get(id));
      } else {
        subParent.current.removeChild(ae.get(id));
        subParent.current.removeChild(ve.get(id));
      }
      return
    }
  }
  
  function sub(id, ans) {
    if (pcsub.has(id)) {
      let d = atob(ans);
      let pl = JSON.parse(d);
      let newpc = pcsub.get(id);
      newpc.setRemoteDescription(pl)
      return
    }
    console.log("no such id " + id);
  }
  
  function inv(id) {
    if (pcsub.has(id)) {
      console.log("already invited " + id);
      return
    }
    let parent = subParent.current;
    if (id == userid) parent = pubParent.current;
  
    let newpc = new RTCPeerConnection();
    newpc.ontrack = function (event) {
      let kind = event.track.kind;
      var el;
  
      if (kind == "audio") {
        if (ae.has(id)) {
          //console.log("replacing audio " + id);
          el = ae.get(id);
        } else {
          //console.log("new audio " + id);
          el = document.createElement(kind);
          ae.set(id, el);
          parent.appendChild(el);
        }
  
        if (userid == id) {
          console.log("muting own audio, userid=", id);
          el.muted = true;
        }
  
      } else {
        if (ve.has(id)) {
          //console.log("replacing video " + id);
          el = ve.get(id);
          el.hidden = false;
        } else {
          //console.log("new video " + id);
          el = document.createElement(kind);
  
          el.height = HEIGHT;
          el.width = WIDTH;
  
          ve.set(id, el);
          parent.appendChild(el);
        }
      }
  
      el.srcObject = event.streams[0]
      el.autoplay = true
    }
  
    //newpc.oniceconnectionstatechange = e => console.log(newpc.iceConnectionState)
  
    // Offer to receive 1 audio, and 1 video track
    newpc.addTransceiver('video', {
      direction: 'sendrecv'
    })
    newpc.addTransceiver('audio', {
      direction: 'sendrecv'
    })
  
    newpc.createOffer()
      .then(offer => {
        newpc.setLocalDescription(offer);
        pcsub.set(id, newpc);
        let js = {
          "action": "sub",
          "id": id,
          "data": offer,
        }
        ws.current.send(JSON.stringify(js))
      })
  
  }
  
  
  function pub(uid, ans) {
    //console.log("pub()" + ans);
    if (ans != null) {
      userid = uid;
      console.log("userid=" + userid);
      let d = atob(ans);
      let pl = JSON.parse(d);
      pc.setRemoteDescription(pl)
      return
    }
  
    // start publishing
    navigator.mediaDevices.getUserMedia({
      video: true,
      audio: true
    })
      .then(stream => {
        let stream_settings = stream.getVideoTracks()[0].getSettings();
        let stream_width = stream_settings.width;
        let stream_height = stream_settings.height;
  
        const video = document.createElement('video');
        pubParent.current.replaceChildren(video);
  
        video.height = HEIGHT;
        video.width = WIDTH;
  
        video.srcObject = stream
        stream.getTracks().forEach(track => pc.addTrack(track, stream));
        video.play();
        video.muted = true;
  
        pc.createOffer()
          .then(offer => {
            pc.setLocalDescription(offer)
            let js = {
              "action": "pub",
              "data": offer
            }
            ws.current.send(JSON.stringify(js))
  
            //alert("OFFER: " + offer)
            return
          })
          .catch(e => console.log(e));
      }).catch(e => console.log(e));
  }
  
  function hide(v) {
    v.hidden = true;
  }

  return (
    <div>
      <div id="menu">
        <button onClick={() => legacy()}> With Webcam </button>
        <button onClick={() => selectFlexatar('/opt/flexapix/flexatar/static3/flx/flx_opnLeo_Emo_01.p')}> With Flexatar #1</button>
        <button onClick={() => selectFlexatar('/opt/flexapix/flexatar/static3/flx/flx_opnLeo_Emo_09_venik.p')}> With Flexatar #2</button>
      </div>

      <p>videos from others</p>
      <div id="sub"></div>

      <p>own videos: as captured and as seen by others</p>
      <div id="pub">

      </div>

    </div>
  );
}
