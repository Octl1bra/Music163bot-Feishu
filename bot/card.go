package bot

import (
	"encoding/json"
	"fmt"
)

// cardJSON 序列化卡片
// albumPicEnabled 是否在信息卡片中展示专辑封面
func albumPicEnabled() bool {
	return config["AlbumPic"] == "true"
}

func cardJSON(card map[string]interface{}) string {
	b, _ := json.Marshal(card)
	return string(b)
}

func cardHeader(template, title string) map[string]interface{} {
	return map[string]interface{}{
		"template": template,
		"title":    map[string]interface{}{"tag": "plain_text", "content": title},
	}
}

func cardText(content string) map[string]interface{} {
	return map[string]interface{}{
		"tag":  "div",
		"text": map[string]interface{}{"tag": "lark_md", "content": content},
	}
}

func cardNote(content string) map[string]interface{} {
	return map[string]interface{}{
		"tag": "note",
		"elements": []interface{}{
			map[string]interface{}{"tag": "plain_text", "content": content},
		},
	}
}

// progressCard 下载进度卡片
func progressCard(title, body string) string {
	return cardJSON(map[string]interface{}{
		"config":   map[string]interface{}{"wide_screen_mode": true},
		"header":   cardHeader("blue", title),
		"elements": []interface{}{cardText(body)},
	})
}

// errorCard 失败提示卡片
func errorCard(title, body string) string {
	return cardJSON(map[string]interface{}{
		"config":   map[string]interface{}{"wide_screen_mode": true},
		"header":   cardHeader("red", title),
		"elements": []interface{}{cardText(body)},
	})
}

// infoCard 歌曲信息卡片; 封面默认禁用, ALBUM_PIC=true 开启
func infoCard(songInfo SongInfo) string {
	imageKey := ""
	if albumPicEnabled() {
		imageKey = songInfo.ImageKey
	}
	elements := []interface{}{}
	if imageKey != "" {
		elements = append(elements, map[string]interface{}{
			"tag":     "img",
			"img_key": imageKey,
			"alt":     map[string]interface{}{"tag": "plain_text", "content": songInfo.SongAlbum},
			"mode":    "crop_center",
		})
	}
	elements = append(elements,
		cardText(fmt.Sprintf("**歌手:** %s\n**专辑:** %s\n**格式:** %s %.2fMB %.2fkbps",
			songInfo.SongArtists, songInfo.SongAlbum, songInfo.FileExt,
			float64(songInfo.MusicSize+songInfo.EmbPicSize)/1024/1024, float64(songInfo.BitRate)/1000)),
		map[string]interface{}{
			"tag": "action",
			"actions": []interface{}{
				map[string]interface{}{
					"tag":  "button",
					"text": map[string]interface{}{"tag": "plain_text", "content": "在网易云音乐打开"},
					"type": "primary",
					"url":  fmt.Sprintf("https://music.163.com/song?id=%d", songInfo.MusicID),
				},
			},
		},
	)
	return cardJSON(map[string]interface{}{
		"config":   map[string]interface{}{"wide_screen_mode": true},
		"header":   cardHeader("green", fmt.Sprintf("%s - %s", songInfo.SongName, songInfo.SongArtists)),
		"elements": elements,
	})
}

type searchItem struct {
	MusicID int
	Name    string
	Artists string
}

// searchCard 搜索结果卡片, 每首歌提供语音/文件两种发送按钮
func searchCard(keyword string, items []searchItem) string {
	var listText string
	var voiceButtons, fileButtons []interface{}
	for i, item := range items {
		listText += fmt.Sprintf("%d.「%s」 - %s\n", i+1, item.Name, item.Artists)
		voiceButtons = append(voiceButtons, map[string]interface{}{
			"tag":  "button",
			"text": map[string]interface{}{"tag": "plain_text", "content": fmt.Sprintf("🎵 %d", i+1)},
			"type": "default",
			"value": map[string]interface{}{
				"action": "music",
				"id":     fmt.Sprintf("%d", item.MusicID),
				"mode":   modeVoice,
			},
		})
		fileButtons = append(fileButtons, map[string]interface{}{
			"tag":  "button",
			"text": map[string]interface{}{"tag": "plain_text", "content": fmt.Sprintf("📄 %d", i+1)},
			"type": "default",
			"value": map[string]interface{}{
				"action": "music",
				"id":     fmt.Sprintf("%d", item.MusicID),
				"mode":   modeFile,
			},
		})
	}
	return cardJSON(map[string]interface{}{
		"config": map[string]interface{}{"wide_screen_mode": true},
		"header": cardHeader("blue", fmt.Sprintf("搜索: %s", keyword)),
		"elements": []interface{}{
			cardText(listText),
			cardNote("🎵 语音方式 / 📄 文件方式"),
			map[string]interface{}{"tag": "action", "actions": voiceButtons},
			map[string]interface{}{"tag": "action", "actions": fileButtons},
		},
	})
}
