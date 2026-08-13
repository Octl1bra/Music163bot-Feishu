package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	marker "github.com/XiaoMengXinX/163KeyMarker"
	"github.com/XiaoMengXinX/Music163Api-Go/api"
	"github.com/XiaoMengXinX/Music163Api-Go/types"
	downloader "github.com/XiaoMengXinX/SimpleDownloader"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// 限制并发任务数
var musicLimiter = make(chan bool, 4)

// 飞书 IM 文件上传上限
const maxIMFileSize = 30 * 1024 * 1024

func processMusic(musicID int, mode string, msg incomingMsg) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	if mode == modeVoice && !ffmpegExists() {
		_, err = replyText(msg, ffmpegMissing)
		return err
	}

	d := downloader.NewDownloader().SetSavePath(cacheDir).SetBreakPoint(true)
	d.SetTimeOut(time.Duration(int64(downloaderTimeout)) * time.Second)

	var songInfo SongInfo
	var progressMsgID string

	sendFailed := func(err error) {
		var errText string
		if strings.Contains(fmt.Sprintf("%v", err), md5VerFailed) || strings.Contains(fmt.Sprintf("%v", err), downloadTimeout) {
			errText = "%v"
		} else {
			errText = uploadFailed
		}
		errStr := fmt.Sprintf("%v", err)
		if config["APP_SECRET"] != "" {
			errStr = strings.ReplaceAll(errStr, config["APP_SECRET"], "APP_SECRET")
		}
		body := fmt.Sprintf(musicInfoMsg+"\n"+errText, songInfo.SongName, songInfo.SongAlbum, songInfo.FileExt, float64(songInfo.MusicSize)/1024/1024, errStr)
		if progressMsgID != "" {
			logErr(patchCard(progressMsgID, errorCard(fetchInfoFailed, body)))
		} else {
			_, e := replyText(msg, body)
			logErr(e)
		}
	}

	db := MusicDB.Session(&gorm.Session{})
	err = db.Where("music_id = ?", musicID).First(&songInfo).Error
	if err == nil && hasCachedKey(songInfo, mode) {
		return sendCachedMusic(songInfo, mode, msg)
	}

	progressMsgID, err = replyCard(msg, progressCard(waitForDown, fmt.Sprintf("MusicID: %d", musicID)))
	if err != nil {
		return err
	}

	musicLimiter <- true
	defer func() {
		<-musicLimiter
	}()

	// 拿到执行权后再查一次缓存, 避免重复下载
	err = db.Where("music_id = ?", musicID).First(&songInfo).Error
	if err == nil && hasCachedKey(songInfo, mode) {
		logErr(patchCard(progressMsgID, progressCard(hitCache, fmt.Sprintf("%s - %s", songInfo.SongName, songInfo.SongArtists))))
		err = sendCachedMusic(songInfo, mode, msg)
		if err != nil {
			sendFailed(err)
			return err
		}
		return deleteMessage(progressMsgID)
	}
	cacheHit := err == nil // 记录已存在但缺少当前发送方式的 file_key

	logErr(patchCard(progressMsgID, progressCard(fetchInfo, fmt.Sprintf("MusicID: %d", musicID))))

	b := api.NewBatch(
		api.BatchAPI{
			Key:  api.SongDetailAPI,
			Json: api.CreateSongDetailReqJson([]int{musicID}),
		},
		api.BatchAPI{
			Key:  api.SongUrlAPI,
			Json: api.CreateSongURLJson(api.SongURLConfig{Ids: []int{musicID}}),
		},
	)
	if b.Do(data).Error != nil {
		sendFailed(b.Error)
		return b.Error
	}
	_, result := b.Parse()

	var songDetail types.SongsDetailData
	_ = json.Unmarshal([]byte(result[api.SongDetailAPI]), &songDetail)

	var songURL types.SongsURLData
	_ = json.Unmarshal([]byte(result[api.SongUrlAPI]), &songURL)

	if len(songDetail.Songs) == 0 || len(songURL.Data) == 0 {
		logErr(patchCard(progressMsgID, errorCard(fetchInfoFailed, fmt.Sprintf("MusicID: %d", musicID))))
		return errors.New(fetchInfoFailed)
	}
	if songURL.Data[0].Url == "" {
		logErr(patchCard(progressMsgID, errorCard(getUrlFailed, fmt.Sprintf("MusicID: %d", musicID))))
		return errors.New(getUrlFailed)
	}

	songInfo.FromChatID = msg.ChatID
	songInfo.FromUserID = msg.SenderOpenID

	songInfo.MusicID = musicID
	songInfo.Duration = songDetail.Songs[0].Dt / 1000
	songInfo.SongName = songDetail.Songs[0].Name // 解析歌曲信息
	songInfo.SongArtists = parseArtist(songDetail.Songs[0])
	songInfo.SongAlbum = songDetail.Songs[0].Al.Name
	url := songURL.Data[0].Url
	// 从 URL 中移除查询参数以正确获取扩展名
	baseURL := url
	if queryIndex := strings.Index(url, "?"); queryIndex != -1 {
		baseURL = url[:queryIndex]
	}
	switch path.Ext(path.Base(baseURL)) {
	case ".mp3":
		songInfo.FileExt = "mp3"
	case ".flac":
		songInfo.FileExt = "flac"
	default:
		songInfo.FileExt = "mp3"
	}
	songInfo.MusicSize = songURL.Data[0].Size
	songInfo.BitRate = 8 * songURL.Data[0].Size / (songDetail.Songs[0].Dt / 1000)

	if picRes, err := http.Head(songDetail.Songs[0].Al.PicUrl); err == nil {
		songInfo.PicSize = int(picRes.ContentLength)
	} else {
		logrus.Errorln(err)
	}

	musicInfoBody := func() string {
		return fmt.Sprintf(musicInfoMsg, songInfo.SongName, songInfo.SongAlbum, songInfo.FileExt, float64(songInfo.MusicSize)/1024/1024)
	}
	logErr(patchCard(progressMsgID, progressCard(downloading, musicInfoBody())))

	hostReplacer := strings.NewReplacer("m8.", "m7.", "m801.", "m701.", "m804.", "m701.", "m704.", "m701.")

	timeStamp := time.Now().UnixMicro()
	musicFileName := fmt.Sprintf("%d-%s", timeStamp, path.Base(baseURL))

	task, _ := d.NewDownloadTask(url)
	host := task.GetHostName()
	task.ReplaceHostName(hostReplacer.Replace(host)).ForceHttps().ForceMultiThread()
	errCh := task.SetFileName(musicFileName).DownloadWithChannel()

	updateStatus := func(task *downloader.DownloadTask, ch chan error, statusTitle string) (err error) {
		var lastUpdateTime int64
	loop:
		for {
			select {
			case err = <-ch:
				break loop
			default:
				writtenBytes := task.GetWrittenBytes()
				if task.GetFileSize() == 0 || writtenBytes == 0 || time.Now().Unix()-lastUpdateTime < 5 {
					continue
				}
				body := musicInfoBody() + fmt.Sprintf(downloadStatus, task.CalculateSpeed(time.Millisecond*500), float64(writtenBytes)/1024/1024, float64(task.GetFileSize())/1024/1024, (writtenBytes*100)/task.GetFileSize())
				_ = patchCard(progressMsgID, progressCard(statusTitle, body))
				lastUpdateTime = time.Now().Unix()
			}
		}
		return err
	}

	err = updateStatus(task, errCh, downloading)
	if err != nil {
		if config["ReverseProxy"] != "" {
			ch := task.WithResolvedIpOnHost(config["ReverseProxy"]).DownloadWithChannel()
			err = updateStatus(task, ch, redownloading)
			if err != nil {
				sendFailed(err)
				task.CleanTempFiles()
				return err
			}
		} else {
			sendFailed(err)
			task.CleanTempFiles()
			return err
		}
	}

	isMD5Verified, _ := verifyMD5(cacheDir+"/"+musicFileName, songURL.Data[0].Md5)
	if !isMD5Verified && songURL.Data[0].Md5 != "" {
		err = os.Remove(cacheDir + "/" + musicFileName)
		if err != nil {
			logrus.Errorln(err)
		}
		sendFailed(fmt.Errorf("%s\n%s", md5VerFailed, retryLater))
		return nil
	}

	var picPath, resizePicPath string
	p, _ := d.NewDownloadTask(songDetail.Songs[0].Al.PicUrl)
	err = p.SetFileName(fmt.Sprintf("%d-%s", timeStamp, path.Base(songDetail.Songs[0].Al.PicUrl))).Download()
	if err != nil {
		logrus.Errorln(err)
	} else {
		picPath = cacheDir + "/" + fmt.Sprintf("%d-%s", timeStamp, path.Base(songDetail.Songs[0].Al.PicUrl))
		var err error
		resizePicPath, err = resizeImg(picPath)
		if err != nil {
			logrus.Errorln(err)
		}
	}

	var musicPic string
	picStat, err := os.Stat(picPath)
	if picStat != nil && err == nil {
		if picStat.Size() > 2*1024*1024 {
			musicPic = resizePicPath
			embPicStat, _ := os.Stat(resizePicPath)
			songInfo.EmbPicSize = int(embPicStat.Size())
		} else {
			musicPic = picPath
			songInfo.EmbPicSize = songInfo.PicSize
		}
	} else {
		logrus.Errorln(err)
	}

	var pic *os.File = nil

	if picStat != nil && err == nil {
		pic, _ = os.Open(musicPic)
		defer pic.Close()
	}

	var replacer = strings.NewReplacer("/", " ", "?", " ", "*", " ", ":", " ", "|", " ", "\\", " ", "<", " ", ">", " ", "\"", " ")
	var newDir = cacheDir + "/" + fmt.Sprintf("%d", timeStamp)
	fileName := replacer.Replace(fmt.Sprintf("%v - %v.%v", strings.Replace(songInfo.SongArtists, "/", ",", -1), songInfo.SongName, songInfo.FileExt))
	var filePath = newDir + "/" + fileName
	err = os.Mkdir(newDir, os.ModePerm)
	if err != nil {
		sendFailed(err)
		return err
	}
	err = os.Rename(cacheDir+"/"+musicFileName, filePath)
	if err != nil {
		filePath = cacheDir + "/" + musicFileName
	}

	mark := marker.CreateMarker(songDetail.Songs[0], songURL.Data[0])

	file, _ := os.Open(filePath)
	defer file.Close()

	err = marker.AddMusicID3V2(file, pic, mark)
	if err != nil {
		file, _ = os.Open(filePath)
		defer file.Close()
		err = marker.AddMusicID3V2(file, nil, mark)
	}
	if err != nil {
		sendFailed(err)
		return err
	}

	// 上传封面图用于信息卡片
	if songInfo.ImageKey == "" && picPath != "" {
		imageKey, err := uploadIMImage(picPath)
		if err != nil {
			logrus.Errorln(err)
		} else {
			songInfo.ImageKey = imageKey
		}
	}

	// 文件方式超过飞书上限时自动回退语音方式
	if mode == modeFile {
		if stat, err := os.Stat(filePath); err == nil && stat.Size() > maxIMFileSize {
			if !ffmpegExists() {
				sendFailed(fmt.Errorf("文件大小 %.2fMB 超过飞书 30MB 上限, 且未找到 ffmpeg 无法转换语音", float64(stat.Size())/1024/1024))
				return nil
			}
			_, e := replyText(msg, fmt.Sprintf(fileTooLarge, float64(stat.Size())/1024/1024))
			logErr(e)
			mode = modeVoice
		}
	}

	cleanup := func() {
		for _, f := range []string{filePath, resizePicPath, picPath} {
			if f == "" {
				continue
			}
			if err := os.Remove(f); err != nil {
				logrus.Errorln(err)
			}
		}
		if err := os.RemoveAll(newDir); err != nil {
			logrus.Errorln(err)
		}
	}
	defer cleanup()

	logErr(patchCard(progressMsgID, progressCard(uploading, musicInfoBody())))

	switch mode {
	case modeVoice:
		opusPath := filePath + ".opus"
		logErr(patchCard(progressMsgID, progressCard(converting, musicInfoBody())))
		err = convertToOpus(filePath, opusPath)
		if err != nil {
			sendFailed(err)
			return err
		}
		defer os.Remove(opusPath)
		opusName := strings.TrimSuffix(fileName, "."+songInfo.FileExt) + ".opus"
		fileKey, err := uploadIMFile(opusPath, "opus", opusName, songInfo.Duration*1000)
		if err != nil {
			sendFailed(err)
			return err
		}
		songInfo.VoiceFileKey = fileKey
	case modeFile:
		fileKey, err := uploadIMFile(filePath, "stream", fileName, songInfo.Duration*1000)
		if err != nil {
			sendFailed(err)
			return err
		}
		songInfo.FileKey = fileKey
	}

	if cacheHit {
		err = db.Save(&songInfo).Error
	} else {
		err = db.Create(&songInfo).Error // 写入歌曲缓存
	}
	if err != nil {
		logrus.Errorln(err)
	}

	err = sendMusicMessage(songInfo, mode, msg)
	if err != nil {
		sendFailed(err)
		return err
	}

	// 进度卡片更新为歌曲信息卡片
	return patchCard(progressMsgID, infoCard(songInfo, songInfo.ImageKey))
}

// hasCachedKey 判断缓存记录是否包含当前发送方式的 file_key
func hasCachedKey(songInfo SongInfo, mode string) bool {
	if mode == modeVoice {
		return songInfo.VoiceFileKey != ""
	}
	return songInfo.FileKey != ""
}

// sendMusicMessage 使用 file_key 发送歌曲消息 (语音/文件)
func sendMusicMessage(songInfo SongInfo, mode string, msg incomingMsg) (err error) {
	if mode == modeVoice {
		_, err = replyAudio(msg, songInfo.VoiceFileKey)
	} else {
		_, err = replyFile(msg, songInfo.FileKey)
	}
	return err
}

// sendCachedMusic 缓存命中时发送歌曲消息与信息卡片
func sendCachedMusic(songInfo SongInfo, mode string, msg incomingMsg) (err error) {
	err = sendMusicMessage(songInfo, mode, msg)
	if err != nil {
		return err
	}
	_, err = replyCard(msg, infoCard(songInfo, songInfo.ImageKey))
	return err
}

// ffmpegExists 检查 ffmpeg 是否可用
func ffmpegExists() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// convertToOpus 将音频转换为飞书要求的 opus 格式
// 默认 48kHz 双声道 128k; 若客户端无法播放, 可在配置中调整为官方示例的 16000/1
func convertToOpus(src, dst string) error {
	bitrate := config["OpusBitrate"]
	if bitrate == "" {
		bitrate = "128k"
	}
	sampleRate := config["OpusSampleRate"]
	if sampleRate == "" {
		sampleRate = "48000"
	}
	channels := config["OpusChannels"]
	if channels == "" {
		channels = "2"
	}
	cmd := exec.Command("ffmpeg", "-y", "-i", src, "-acodec", "libopus", "-b:a", bitrate, "-ac", channels, "-ar", sampleRate, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 转换失败: %v\n%s", err, lastLines(string(output), 5))
	}
	return nil
}

// lastLines 截取文本末尾几行, 用于错误信息
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// getProgramRealIDFromArgs 解析 /program 参数
func parseProgramArg(args string) int {
	id, _ := strconv.Atoi(args)
	if id == 0 {
		id = parseProgramID(args)
	}
	return id
}
