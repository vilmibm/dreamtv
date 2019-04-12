package scheduler

import  (
  "log"
  "os/exec"
  "path/filepath"
)

// func init() { }

func insertTape() {
  log.Println("inserting tape")

  path, _ := filepath.Abs("./cyborg.flv")

  // TODO support -ss option for seeking
  ffmpegCmd := exec.Command("ffmpeg", "-re",  "-i", path, "-c", "copy", "-f", "flv", "rtmp://localhost/movie") // TODO dynamic
  out, err := ffmpegCmd.Output()
  if err != nil {
    panic(err)
  }
  log.Println(string(out))
}

func StartScheduler (tvdir string, dbfile string) {

  insertTape()
}

