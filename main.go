// Steps:
// - plays a video, listening on localhost
// - loads an html page with a <video> element
// - can operate without manual ffmpeg writing(?)
// - can switch between files
package main

import (
  "fmt"
  "sync"
  "io"
  "log"
  "net/http"
  "github.com/nareix/joy4/format"
  "github.com/nareix/joy4/av/avutil"
  "github.com/nareix/joy4/av/pubsub"
  "github.com/nareix/joy4/format/rtmp"
  "github.com/nareix/joy4/format/flv"
  "github.com/gorilla/websocket"
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

var clients = make(map[*websocket.Conn]bool) // connected clients
var broadcast = make(chan Message) // broadcast channel
var upgrader = websocket.Upgrader{}

type Message struct {
  Username string `json:"username"`
  Message string `json:"message"`
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
  // Upgrade initial GET request to a websocket
  ws, err := upgrader.Upgrade(w, r, nil)
  if err != nil {
    log.Fatal(err)
  }
  defer ws.Close()
  clients[ws] = true
  for {
    var msg Message
    // read in new message as json and map to a message object
    err := ws.ReadJSON(&msg)
    if err != nil {
      log.Printf("error: %v", err)
      delete(clients, ws)
      break
    }
    // send received message to broadcast channel
    broadcast <- msg
  }
}

func handleMessages() {
  for {
    // grab next message from broadcast channel
    msg := <-broadcast
    // send out to every client that is connected
    for client := range clients {
      err := client.WriteJSON(msg)
      if err != nil {
        log.Printf("error: %v", err)
        client.Close()
        delete(clients, client)
      }
    }
  }
}

func main() {
  // putting chat code in main for now - no idea how go works w/ abstracting
  // stuff out yet

  // fileserver to serve http + assets
  fs := http.FileServer(http.Dir("./public"))
  http.Handle("/", fs)
  http.HandleFunc("/ws", handleConnections)
  // start listening for incoming chat messages
  go handleMessages()
  // start server on localhost 8000 and log errors
  log.Println("http server started on :8000")
  err := http.ListenAndServe(":8000", nil)
  if err != nil {
    log.Fatal("ListenAndServe: ", err)
  }


  // below is nate's rtmp code

  server := &rtmp.Server{}

  l := &sync.RWMutex{}
  type Channel struct {
    que *pubsub.Queue
  }
  channels := map[string]*Channel{}

  server.HandlePlay = func(conn *rtmp.Conn) {
    fmt.Println("Handle play")
    fmt.Println(conn.URL.Path)
    l.RLock()
    ch := channels[conn.URL.Path]
    l.RUnlock()

    if ch != nil {
      cursor := ch.que.Latest()
      avutil.CopyFile(conn, cursor)
    }
  }

  server.HandlePublish = func(conn *rtmp.Conn) {
    // This seems to *take* video from somewhere.
    fmt.Println("Handle publish")
    streams, _ := conn.Streams()

    l.Lock()
    ch := channels[conn.URL.Path]
    if ch == nil {
      ch = &Channel{}
      ch.que = pubsub.NewQueue()
      ch.que.WriteHeader(streams)
      channels[conn.URL.Path] = ch
    } else {
      ch = nil
    }
    l.Unlock()
    if ch == nil {
      return
    }

    avutil.CopyPackets(ch.que, conn)

    l.Lock()
    delete(channels, conn.URL.Path)
    l.Unlock()
    ch.que.Close()
  }

  // http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
  //   fmt.Println("got a request for the html page")
  //   w.Header().Set("Content-Type", "text/html")
  //   w.WriteHeader(200)
  //   // slurp a file and write it
  // })

  http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
    fmt.Println("got a request for the stream over HTTP")
    l.RLock()
    // Hardcoding this for now. unsure of direction of channels map.
    ch := channels["/movie"]
    l.RUnlock()

    if ch != nil {
      w.Header().Set("Content-Type", "video/x-flv")
      w.Header().Set("Transfer-Encoding", "chunked")
      w.Header().Set("Access-Control-Allow-Origin", "*")
      w.WriteHeader(200)
      flusher := w.(http.Flusher)
      flusher.Flush()

      muxer := flv.NewMuxerWriteFlusher(writeFlusher{httpflusher: flusher, Writer: w})
      cursor := ch.que.Latest()

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
