package bot

import (
	"fmt"
	"strconv"
	"time"

	"github.com/XiaoMengXinX/Music163Api-Go/api"
	"gorm.io/gorm"
)

// 限制查询速度及并发
var statLimiter = make(chan bool, 1)

func printAbout(msg incomingMsg) (err error) {
	_, err = replyText(msg, fmt.Sprintf(aboutText, config["runtimeVer"], config["BinVersionName"], config["commitSHA"], config["buildTime"], config["buildArch"]))
	return err
}

func processRmCache(args string, msg incomingMsg) (err error) {
	musicID := parseMusicID(args)
	if musicID == 0 {
		musicID = parseProgramID(args)
		if musicID = getProgramRealID(musicID); musicID == 0 {
			return err
		}
	}
	db := MusicDB.Session(&gorm.Session{})
	var songInfo SongInfo
	err = db.Where("music_id = ?", musicID).First(&songInfo).Error
	if err == nil {
		db.Where("music_id = ?", musicID).Delete(&songInfo)
		_, err = replyText(msg, fmt.Sprintf(rmcacheReport, songInfo.SongName))
		return err
	}
	_, err = replyText(msg, noCache)
	return err
}

func processAnyMusic(args string, mode string, msg incomingMsg) (err error) {
	if args == "" {
		_, err = replyText(msg, inputIDorKeyword)
		return err
	}
	musicID, _ := strconv.Atoi(args)
	if musicID == 0 {
		musicID = parseMusicID(args)
	}
	if musicID != 0 {
		return processMusic(musicID, mode, msg)
	}
	searchResult, _ := api.SearchSong(data, api.SearchSongConfig{
		Keyword: args,
		Limit:   10,
	})
	if len(searchResult.Result.Songs) == 0 {
		_, err = replyText(msg, noResults)
		return err
	}
	return processMusic(searchResult.Result.Songs[0].Id, mode, msg)
}

func processStatus(msg incomingMsg) (err error) {
	statLimiter <- true
	defer func() {
		time.Sleep(time.Millisecond * 500)
		<-statLimiter
	}()
	db := MusicDB.Session(&gorm.Session{})
	var fromCount, chatCount int64
	var lastRecord SongInfo
	db.Where("from_user_id = ?", msg.SenderOpenID).Count(&fromCount)
	db.Where("from_chat_id = ?", msg.ChatID).Count(&chatCount)
	db.Last(&lastRecord)

	_, err = replyText(msg, fmt.Sprintf(statusInfo, lastRecord.ID, chatCount, fromCount))
	return err
}
