// Steps:
// - [x] plays a video, listening on localhost
// - [x] has a single video buffer instead of a map
// - [ ] can take a arg for working directory
// - [ ] can call out to ffmpeg to publish files
// - [ ] loads an html page with a <video> element
// - [ ] can operate without manual ffmpeg writing(?)
// - [ ] can switch between files
package main

import (
  "fmt"
  "sync"
  "io"
  "os/exec"
  "net/http"
  "github.com/nareix/joy4/format"
  "github.com/nareix/joy4/av/avutil"
  "github.com/nareix/joy4/av/pubsub"
  "github.com/nareix/joy4/format/rtmp"
  "github.com/nareix/joy4/format/flv"
)

func init() {
  format.RegisterAll()
}

type writeFlusher struct {
  httpflusher http.Flusher
  io.Writer
}

func (self writeFlusher) Flush() error {
  self.httpflusher.Flush()
  return nil
}

func getCmd(filepath string) string {
  // TODO support -ss option for seeking
  return fmt.Sprintf("ffmpeg -re -i %s -c copy -f flv rtmp://localhost/movie", filepath)
}

func main() {
  server := &rtmp.Server{}

  l := &sync.RWMutex{}
  type Channel struct {
    que *pubsub.Queue
  }
  var vbuf *Channel

  server.HandlePlay = func(conn *rtmp.Conn) {
    fmt.Println("Handle play")
    fmt.Println(conn.URL.Path)
    l.RLock()
    l.RUnlock()

    if vbuf != nil {
      cursor := vbuf.que.Latest()
      avutil.CopyFile(conn, cursor)
    }
  }

  server.HandlePublish = func(conn *rtmp.Conn) {
    fmt.Println("Handle publish")
    streams, _ := conn.Streams()

    l.Lock()
    fmt.Println("vbuf is %#v", vbuf)
    if vbuf == nil {
      vbuf = &Channel{}
      vbuf.que = pubsub.NewQueue()
      vbuf.que.WriteHeader(streams)
    } else {
      vbuf = nil
    }
    l.Unlock()
    if vbuf == nil {
      return
    }
    fmt.Println("vbuf is %#v", vbuf)

    avutil.CopyPackets(vbuf.que, conn)

    vbuf.que.Close()
  }

  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Println("got a request for the html page")
    w.Header().Set("Content-Type", "text/html")
    w.WriteHeader(200)
    // slurp a file and write it 
  })

  http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
    fmt.Println("got a request for the stream over HTTP")
    fmt.Println("vbuf is %#v", vbuf)

    if vbuf != nil {
      w.Header().Set("Content-Type", "video/x-flv")
      w.Header().Set("Transfer-Encoding", "chunked")
      w.Header().Set("Access-Control-Allow-Origin", "*")
      w.WriteHeader(200)
      flusher := w.(http.Flusher)
      flusher.Flush()

      muxer := flv.NewMuxerWriteFlusher(writeFlusher{httpflusher: flusher, Writer: w})
      cursor := vbuf.que.Latest()

      avutil.CopyFile(muxer, cursor)
    } else {
      http.NotFound(w, r)
    }
  })

  fmt.Println("http Listening on 8089")
  go http.ListenAndServe(":8089", nil)

  fmt.Println("rtmp listening on 1935")

  // The default rtmp port is 1935
  server.ListenAndServe()

  // OG examples:

  // ffmpeg -re -i movie.flv -c copy -f flv rtmp://localhost/movie
  // ffmpeg -f avfoundation -i "0:0" .... -f flv rtmp://localhost/screen
  // ffplay http://localhost:8089/movie
  // ffplay http://localhost:8089/screen
}

/*

  NOTES

  Nervous about the lock. I saw some weird locked behavior around opening streams (but not consistently).

  I got this working with these steps:
  - ./dreamtv
  - ffmpeg -re -i /home/vilmibm/Dropbox/vid/VHS/cyborg.flv -c copy -f flv rtmp://localhost/movie
  - vlc open stream rtmp://localhost:1935/movie

*/
