package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/XiaoMengXinX/Music163Api-Go/utils"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sirupsen/logrus"
)

// Start bot entry
func Start(conf map[string]string) (actionCode int) {
	config = conf
	defer func() {
		e := recover()
		if e != nil {
			logrus.Errorln(e)
			actionCode = 1
		}
	}()
	// 创建缓存文件夹
	dirExists(cacheDir)

	// 解析 bot 管理员配置 (open_id, 逗号分隔)
	botAdmin = []string{}
	for _, s := range strings.Split(config["BotAdmin"], ",") {
		if s = strings.TrimSpace(s); s != "" {
			botAdmin = append(botAdmin, s)
		}
	}

	// 初始化数据库
	err := initDB(config)
	if err != nil {
		logrus.Errorln(err)
		return 1
	}
	db, _ := MusicDB.DB()
	db.SetMaxOpenConns(1)
	defer db.Close()

	if config["MUSIC_U"] != "" {
		data = utils.RequestData{
			Cookies: []*http.Cookie{
				{
					Name:  "MUSIC_U",
					Value: config["MUSIC_U"],
				},
			},
		}
	}

	if maxRetryTimes, _ = strconv.Atoi(config["MaxRetryTimes"]); maxRetryTimes <= 0 {
		maxRetryTimes = 3
	}
	if downloaderTimeout, _ = strconv.Atoi(config["DownloadTimeout"]); downloaderTimeout <= 0 {
		downloaderTimeout = 60
	}

	if config["APP_ID"] == "" || config["APP_SECRET"] == "" {
		logrus.Errorln("请在配置文件中填写 APP_ID 与 APP_SECRET")
		return 1
	}

	logLevel := larkcore.LogLevelInfo
	if config["BotDebug"] == "true" {
		logLevel = larkcore.LogLevelDebug
	}

	larkClient = lark.NewClient(config["APP_ID"], config["APP_SECRET"], lark.WithLogLevel(logLevel))

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			go handleMessage(event)
			return nil
		}).
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return handleCardAction(event)
		})

	wsClient := larkws.NewClient(config["APP_ID"], config["APP_SECRET"],
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(logLevel),
	)

	logrus.Println("正在建立飞书长连接...")
	err = wsClient.Start(context.Background())
	if err != nil {
		logrus.Errorln(err)
		return 1
	}
	return 0
}

// handleMessage 处理 im.message.receive_v1 事件
func handleMessage(event *larkim.P2MessageReceiveV1) {
	defer func() {
		if e := recover(); e != nil {
			logrus.Errorln(e)
		}
	}()
	m := event.Event.Message
	if m == nil || m.MessageType == nil || *m.MessageType != larkim.MsgTypeText {
		return
	}
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*m.Content), &content); err != nil {
		return
	}

	msg := incomingMsg{
		MessageID: strVal(m.MessageId),
		ChatID:    strVal(m.ChatId),
		ChatType:  strVal(m.ChatType),
		Text:      strings.TrimSpace(regMention.ReplaceAllString(content.Text, "")),
		ParentID:  strVal(m.ParentId),
	}
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		msg.SenderOpenID = strVal(event.Event.Sender.SenderId.OpenId)
	}
	if msg.Text == "" {
		return
	}

	if strings.HasPrefix(msg.Text, "/") {
		handleCommand(msg)
		return
	}

	// 分享链接嗅探
	if strings.Contains(msg.Text, "music.163.com") || strings.Contains(msg.Text, "163cn.tv") || strings.Contains(msg.Text, "163cn.link") {
		text := msg.Text
		if strings.Contains(text, "163cn.tv") || strings.Contains(text, "163cn.link") {
			text = getRedirectUrl(text)
		}
		id := parseMusicID(text)
		if id != 0 {
			logErr(processMusic(id, modeVoice, msg))
		} else if id = parseProgramID(text); id != 0 {
			if id = getProgramRealID(id); id != 0 {
				logErr(processMusic(id, modeVoice, msg))
			}
		}
		return
	}

	// 私聊里发纯 ID 或关键词以外的内容不响应, 与 TG 版行为一致
}

// handleCommand 解析并分发命令
func handleCommand(msg incomingMsg) {
	fields := strings.Fields(msg.Text)
	command := strings.TrimPrefix(fields[0], "/")
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, fields[0]))

	switch command {
	case "music", "netease":
		logErr(processAnyMusic(args, modeVoice, msg))
	case "music-file", "netease-file":
		logErr(processAnyMusic(args, modeFile, msg))
	case "program":
		if musicID := getProgramRealID(parseProgramArg(args)); musicID != 0 {
			logErr(processMusic(musicID, modeVoice, msg))
		}
	case "program-file":
		if musicID := getProgramRealID(parseProgramArg(args)); musicID != 0 {
			logErr(processMusic(musicID, modeFile, msg))
		}
	case "search":
		logErr(processSearch(args, msg))
	case "lyric":
		logErr(processLyric(args, msg))
	case "recognize":
		logErr(recognizeMusic(msg))
	case "about":
		logErr(printAbout(msg))
	case "status":
		logErr(processStatus(msg))
	case "help", "start":
		_, err := replyText(msg, helpText)
		logErr(err)
	case "rmcache":
		if in(msg.SenderOpenID, botAdmin) {
			logErr(processRmCache(args, msg))
		}
	case "reload":
		if in(msg.SenderOpenID, botAdmin) {
			_, _ = replyText(msg, reloading)
			logrus.Println("收到 reload 命令, 进程退出交由 supervisor 重启")
			os.Exit(0)
		}
	}
}

// handleCardAction 处理卡片按钮回调
func handleCardAction(event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if event.Event == nil || event.Event.Action == nil || event.Event.Context == nil {
		return nil, nil
	}
	value := event.Event.Action.Value
	if value["action"] != "music" {
		return nil, nil
	}
	musicID, _ := strconv.Atoi(strFromAny(value["id"]))
	if musicID == 0 {
		return nil, nil
	}
	mode := strFromAny(value["mode"])
	if mode != modeFile {
		mode = modeVoice
	}

	msg := incomingMsg{
		MessageID: event.Event.Context.OpenMessageID,
		ChatID:    event.Event.Context.OpenChatID,
	}
	if event.Event.Operator != nil {
		msg.SenderOpenID = event.Event.Operator.OpenID
	}
	go func() {
		defer func() {
			if e := recover(); e != nil {
				logrus.Errorln(e)
			}
		}()
		logErr(processMusic(musicID, mode, msg))
	}()

	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "info",
			Content: "正在获取歌曲...",
		},
	}, nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strFromAny(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func logErr(err error) {
	if err != nil {
		logrus.Errorln(err)
	}
}
