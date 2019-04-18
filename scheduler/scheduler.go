package scheduler

import  (
  "database/sql"
  "log"
  "os"
  "os/exec"
  "path/filepath"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

type VideoFile struct {
  id int
  filename string
  channel string
  playcount int
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

// This function ensures the database has the correct schema, deleting and re-created tables if
// needed.
func ensureSchema(conn *sql.DB, force bool) {
  log.Println("ensuring schema. this may delete play stats.")
  createSql := `CREATE TABLE IF NOT EXISTS videos(
    id INTEGER PRIMARY KEY,
    filename TEXT NOT NULL,
    channel TEXT NOT NULL,
    playcount INTEGER DEFAULT 0,
    lastplayed DATETIME
  )`
  dropSql := "DROP TABLE IF EXISTS videos"
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
      _, err3 := conn.Exec("INSERT INTO videos (channel, filename) VALUES (?, ?)", channel, filename)
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

  insertTape()
}

