package scheduler

import  (
  "log"
  "os/exec"
  "path/filepath"
)

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

func syncChannels(tvdir string, dbfile string) {
  // goals:
  // - no video file in db that isn't on disk
  // - no video file on disk not noted in db
  // - play count and last played for every file
  log.Println("scanning", tvdir, "for video files")
}

func StartScheduler (tvdir string, dbfile string) {
  syncChannels(tvdir, dbfile)


  insertTape()
}

