package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var recognizeAPI = "https://music-recognize.vercel.app/api/recognize"

func recognizeMusic(msg incomingMsg) (err error) {
	if !ffmpegExists() {
		_, err = replyText(msg, ffmpegMissing)
		return err
	}
	if msg.ParentID == "" {
		_, err = replyText(msg, replyToVoice)
		return err
	}
	msgType, content, err := getMessage(msg.ParentID)
	if err != nil {
		return err
	}
	if msgType != "audio" {
		_, err = replyText(msg, replyToVoice)
		return err
	}
	var audioContent struct {
		FileKey string `json:"file_key"`
	}
	if err = json.Unmarshal([]byte(content), &audioContent); err != nil || audioContent.FileKey == "" {
		_, err = replyText(msg, replyToVoice)
		return err
	}

	fileName := fmt.Sprintf("%s-%d.opus", msg.ParentID, time.Now().Unix())
	filePath := fmt.Sprintf("%s/%s", cacheDir, fileName)
	err = downloadMessageResource(msg.ParentID, audioContent.FileKey, filePath)
	if err != nil {
		return err
	}
	defer os.Remove(filePath)

	// convert opus to mp3
	mp3Path := filePath + ".mp3"
	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, mp3Path)
	_ = cmd.Run()
	if _, err = os.Stat(mp3Path); err != nil {
		return err
	}
	defer os.Remove(mp3Path)

	newFile, err := os.Open(mp3Path)
	if err != nil {
		return err
	}
	defer newFile.Close()
	newBuf, _ := io.ReadAll(newFile)

	resp, err := uploadRecognizeFile(recognizeAPI, newBuf)
	if err != nil {
		return err
	}

	var result RecognizeResultData
	err = json.Unmarshal(resp, &result)

	if err != nil || len(result.Data.Result) == 0 {
		_, _ = replyText(msg, recognizeFailed)
		return err
	}

	musicID := result.Data.Result[0].Song.Id
	_, _ = replyText(msg, fmt.Sprintf("https://music.163.com/song/%d", musicID))

	return processMusic(musicID, modeVoice, msg)
}

func uploadRecognizeFile(url string, file []byte) ([]byte, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileWriter, err := bodyWriter.CreateFormFile("file", "")
	if err != nil {
		return nil, err
	}
	_, err = fileWriter.Write(file)
	if err != nil {
		return nil, err
	}
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()
	resp, err := http.Post(url, contentType, bodyBuf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return respBody, nil
}

type RecognizeResultData struct {
	Data struct {
		Result []struct {
			Song struct {
				Name string `json:"name"`
				Id   int    `json:"id"`
			} `json:"song"`
		} `json:"result"`
	} `json:"data"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}
