package scheduler

import  (
  "database/sql"
  "encoding/json"
  "log"
  "os"
  "os/exec"
  "path/filepath"
  "strconv"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

type Probe struct {
  Format Format `json:"format"`
}

type Format struct {
  Duration string `json:"duration"`
}

type VideoFile struct {
  id int
  filename string
  channel string
  playcount int
  duration int
  lastplayed time.Time
}

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

// This function ensures the database has the correct schema, deleting and recreating tables if
// needed.
func ensureSchema(conn *sql.DB, force bool) {
  log.Println("ensuring schema. this may delete play stats.")
  createSql := `CREATE TABLE IF NOT EXISTS videos(
    id INTEGER PRIMARY KEY,
    filename TEXT NOT NULL,
    channel TEXT NOT NULL,
    playcount INTEGER DEFAULT 0,
    duration INTEGER NOT NULL,
    lastplayed DATETIME
  ); 
  CREATE TABLE IF NOT EXISTS schedules(
    id INTEGER PRIMARY KEY,
    channel TEXT,
    vids TEXT,
    start DATETIME
  )`
  dropSql := "DROP TABLE IF EXISTS videos; DROP TABLE IF EXISTS schedules"
  if force {
    _, err := conn.Exec(dropSql)
    if err != nil {
      log.Println("failed to clear out database tables")
      panic(err)
    }
  }

  _, err := conn.Exec(createSql)
  if err != nil {
    log.Println("failed to create database tables")
    panic(err)
  }
}

func determineDuration(filepath string) int {
  ffprobeCmd := exec.Command("ffprobe", "-v",  "quiet", "-print_format", "json", "-show_format", "-show_streams", filepath)
  out, err := ffprobeCmd.Output()
  if err != nil {
    panic(err)
  }
  var probe Probe

  json.Unmarshal(out, &probe)

  d, err := strconv.ParseFloat(probe.Format.Duration, 32)
  if err != nil {
    log.Println("could not parse video file's duration!")
    panic(err)
  }

  duration := int(d)

  log.Println("duration found for", filepath, duration)

  return duration
}

// Given an absolute path to a directory with channels and a relative path to a dbfile within that
// dir, this function syncs the disk with the database.
func syncLibrary(tvdir string, conn *sql.DB, resetdb bool) {
  // goals:
  // - no video file in db that isn't on disk
  // - no video file on disk not noted in db
  // - play count and last played for every file
  log.Println("scanning", tvdir, "for video files")
  ensureSchema(conn, resetdb)
  allvids := "SELECT id, channel, filename FROM videos"
  videoRows, err := conn.Query(allvids)
  if err != nil { panic(err) }
  defer videoRows.Close() // TODO cargo coding

  var staleIDs []int

  for videoRows.Next() {
    video := VideoFile{}
    err := videoRows.Scan(&video.id, &video.channel, &video.filename)
    if err != nil { panic(err) }
    log.Println("found", video.channel, video.filename)
    videoPath := filepath.Join(tvdir, "channels", video.channel, video.filename)
    _, err = os.Stat(videoPath)
    if err != nil {
      log.Println("found non-existent file with id", video.id, videoPath)
      staleIDs = append(staleIDs, video.id)
    }
  }

  for _, id := range staleIDs {
    stmt, err := conn.Prepare("DELETE FROM videos WHERE id = ?")
    if err != nil { panic(err) }
    defer stmt.Close() // cargo coding
    _, err2 := stmt.Exec(id)
    if err2 != nil { panic(err2) }
  }

  err3 := filepath.Walk(filepath.Join(tvdir, "channels"), func(path string, info os.FileInfo, err error) error {
    if err != nil { return err }
    if info.IsDir() { return nil }
    channelPath, filename := filepath.Split(path)
    channel := filepath.Base(channelPath)
    log.Printf("Found file %v on channel %v on disk", filename, channel)
    row := conn.QueryRow("SELECT id FROM videos WHERE channel = ? AND filename = ?", channel, filename)
    video := VideoFile{}
    err2 := row.Scan(&video.id)
    if err2 == sql.ErrNoRows {
      log.Println("File", filename, "not in DB, gonna insert")
      duration := determineDuration(path)
      _, err3 := conn.Exec("INSERT INTO videos (channel, filename, duration) VALUES (?, ?, ?)", channel, filename, duration)
      if err3 != nil {
        panic(err3)
      }
    } else if err2 != nil {
      panic(err2)
    }

    return nil
  })

  if err3 != nil {
    log.Println("failed to walk the channels directory")
    panic(err3)
  }
}

func generateSchedule(start time.Time) {
  // TODO get list of channels
}

func StartScheduler(tvdir string, dbfile string, resetdb bool) {
  var dbpath = filepath.Join(tvdir, dbfile)
  log.Println("connecting to db with dbpath:", dbpath)
  conn, err := sql.Open("sqlite3", dbpath)
  if err != nil {
    panic(err)
  } else if conn == nil {
    panic("no connection error but connection is nil")
  }
  syncLibrary(tvdir, conn, resetdb)

  // TODO time to actually generate a schedule
  /*
    questions
    - how to manage the 24 hour loop
    - how to store the schedule

    should schedule go in db? this would allow you to see past schedules. it's not worth doing just
    to enable seeing past schedules imo. i like it in db because:
    - if server restarts it can look for current schedule
    - i'm not super confident in my ability to safely share memory in Go

    so then i'm going to need a schema for this.

    more questions:
    - how to know length of video file? can ffmpeg report on this? TODO
    - how to actually ... store the schedule?
      so a scheudle is:
      {channel_name: [vid id, vid id, vid id, vid id],
       channel_name: [vid id, vid id, vid id, vid id]}

      i could...

      CREATE TABLE schedule(
        id INTEGER PRIMARY KEY,
        channel TEXT,
        vid_ids TEXT
        start DATETIME
      )

      they're all gonna have a matching start.

      the vid_ids thing is definitely a sql smell. the problem is i need to capture ordering, which
      sql isn't awesome at. the "correct" solution is probably storing each vid_id with a start time
      but honestly that's brittle anyway?

      thinking more, the problem comes back to vid lengths. i don't want timestamps for each
      scheduled vid id but i do want offsets, yeah? or at least i need to have video lengths stored
      for each video file.

      i need to be able to say: given channel C and timestamp T, at what point in what video file should i be streaming?

      answer would be: find the schedule row for channel C whose start is (max(start < T)), compute
      diff between T and start, sum the vid_ids' lengths until sum > T, then seek and stream vidfile.

      but, I have to start by answering how i can compute vidfile length.
  */

  insertTape()
}

