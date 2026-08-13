package bot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/XiaoMengXinX/Music163Api-Go/api"
	"github.com/XiaoMengXinX/Music163Api-Go/types"
	"github.com/sirupsen/logrus"
)

func processLyric(args string, msg incomingMsg) (err error) {
	var progressMsgID string
	sendFailed := func() {
		if progressMsgID != "" {
			logErr(patchCard(progressMsgID, errorCard(getLrcFailed, args)))
		} else {
			_, e := replyText(msg, getLrcFailed)
			logErr(e)
		}
	}
	if args == "" {
		_, err = replyText(msg, inputContent)
		return err
	}
	progressMsgID, err = replyCard(msg, progressCard(fetchingLyric, args))
	if err != nil {
		return err
	}

	musicID := parseMusicID(args)
	if musicID == 0 {
		searchResult, _ := api.SearchSong(data, api.SearchSongConfig{
			Keyword: args,
			Limit:   5,
		})
		if len(searchResult.Result.Songs) == 0 {
			logErr(patchCard(progressMsgID, errorCard(noResults, args)))
			return err
		}
		musicID = searchResult.Result.Songs[0].Id
	}

	b := api.NewBatch(api.BatchAPI{
		Key:  api.SongLyricAPI,
		Json: api.CreateSongLyricReqJson(musicID),
	}, api.BatchAPI{
		Key:  api.SongDetailAPI,
		Json: api.CreateSongDetailReqJson([]int{musicID}),
	}).Do(data)
	if b.Error != nil {
		sendFailed()
		return b.Error
	}

	_, result := b.Parse()
	var lyric types.SongLyricData
	var detail types.SongsDetailData
	_ = json.Unmarshal([]byte(result[api.SongLyricAPI]), &lyric)
	_ = json.Unmarshal([]byte(result[api.SongDetailAPI]), &detail)

	if lyric.Lrc.Lyric != "" && len(detail.Songs) != 0 {
		var replacer = strings.NewReplacer("/", " ", "?", " ", "*", " ", ":", " ", "|", " ", "\\", " ", "<", " ", ">", " ", "\"", " ")
		lrcName := fmt.Sprintf("%s - %s.lrc", replacer.Replace(parseArtist(detail.Songs[0])), replacer.Replace(detail.Songs[0].Name))
		lrcPath := fmt.Sprintf("%s/%s", cacheDir, lrcName)
		file, err := os.OpenFile(lrcPath, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			sendFailed()
			return err
		}
		defer func(file *os.File) {
			_ = file.Close()
		}(file)
		write := bufio.NewWriter(file)
		_, _ = write.WriteString(lyric.Lrc.Lyric)
		err = write.Flush()
		if err != nil {
			sendFailed()
			return err
		}
		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				logrus.Errorln(err)
			}
		}(lrcPath)

		fileKey, err := uploadIMFile(lrcPath, "stream", lrcName, 0)
		if err != nil {
			sendFailed()
			return err
		}
		_, err = replyFile(msg, fileKey)
		if err != nil {
			return err
		}
		return deleteMessage(progressMsgID)
	}
	sendFailed()
	return
}
