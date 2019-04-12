// Steps:
// - [x] plays a video, listening on localhost
// - [x] has a single video buffer instead of a map
// - [ ] can take a arg for working directory
// - [x] can call out to ffmpeg to publish files
// - [x] loads an html page
// - [ ] html page has flvjs on it
// - [ ] can switch between files
package main

import (
  "github.com/vilmibm/dreamtv/scheduler"
  "sync"
  "flag"
  "io"
  "log"
  "os"
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
  // TODO: can get number of clients from this:
  log.Printf("# clients connected: %v\n", len(clients));
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

  var tvdir string
  var dbfile string

  //var help = flag.Bool()
  var help = flag.Bool("help", false, "print usage")
  var resetdb = flag.Bool("resetdb", false, "set to true to force recreating the database tables")
  flag.StringVar(&tvdir, "tvdir", "/tvdir", "directory with channel directories and a db file")
  flag.StringVar(&dbfile, "dbfile", "dreamtv.db", "db file relative to tvdir")
  flag.Parse()

  if *help {
    flag.PrintDefaults()
    os.Exit(0)
  }

  // fileserver to serve http + assets
  fs := http.FileServer(http.Dir("./public"))
  http.Handle("/", fs)
  http.HandleFunc("/ws", handleConnections)

  rtmpServer := &rtmp.Server{}

  l := &sync.RWMutex{}
  type Channel struct {
    que *pubsub.Queue
  }
  var vbuf *Channel

  rtmpServer.HandlePlay = func(conn *rtmp.Conn) {
    log.Println("Handle play")

    if vbuf != nil {
      cursor := vbuf.que.Latest()
      avutil.CopyFile(conn, cursor)
    }
  }

  rtmpServer.HandlePublish = func(conn *rtmp.Conn) {
    log.Println("Handle publish")
    streams, _ := conn.Streams()

    l.Lock()
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
    log.Println("vbuf is %#v", vbuf)

    avutil.CopyPackets(vbuf.que, conn)

    vbuf.que.Close()
  }

  http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
    log.Println("got a request for the stream over HTTP")

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

  // Start up our various goroutines. The final server init can't be run with `go` since the program
  // will exit immediately if the main thread is finished. it's kind of arbitrary what server loop
  // we end on; just, one of them has to be the one that keeps the main thread alive.

  go rtmpServer.ListenAndServe()
  log.Println("rtmp server listening on 1935")
  go handleMessages() // start listening for incoming chat messages
  go scheduler.StartScheduler(tvdir, dbfile, *resetdb)

  log.Println("http server listening on 8089")
  http.ListenAndServe(":8089", nil)
}

