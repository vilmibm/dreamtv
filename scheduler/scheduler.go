package scheduler

import  (
  "database/sql"
  "log"
  "os/exec"
  "path/filepath"
  _ "github.com/mattn/go-sqlite3"
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
  // TODO for each file in db, ensure it exists on disk
  // TODO walk dir, adding files as needed
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

