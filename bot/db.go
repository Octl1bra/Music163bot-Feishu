package bot

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SongInfo 歌曲信息
type SongInfo struct {
	gorm.Model
	MusicID      int
	SongName     string
	SongArtists  string
	SongAlbum    string
	FileExt      string
	MusicSize    int
	PicSize      int
	EmbPicSize   int
	BitRate      int
	Duration     int    // 秒
	FileKey      string // 文件消息 file_key
	VoiceFileKey string // 语音消息 file_key
	ImageKey     string // 封面 image_key
	FromUserID   string // open_id
	FromChatID   string // chat_id
}

func initDB(config map[string]string) (err error) {
	database := "cache.db"
	if config["Database"] != "" {
		database = config["Database"]
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("%s?_pragma=busy_timeout(5000)", database)), &gorm.Config{
		Logger:      NewLogger(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		return err
	}
	err = db.Table("song_infos").AutoMigrate(&SongInfo{})
	if err != nil {
		return err
	}
	MusicDB = db.Table("song_infos")
	return err
}
