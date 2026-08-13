package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// botOpenID 机器人自身的 open_id, 用于判断群消息是否 @ 了机器人
var botOpenID string

// fetchBotInfo 获取机器人自身信息 (open_id)
func fetchBotInfo() error {
	resp, err := larkClient.Get(context.Background(), "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		return err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  struct {
			OpenID  string `json:"open_id"`
			AppName string `json:"app_name"`
		} `json:"bot"`
	}
	if err = json.Unmarshal(resp.RawBody, &result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("获取机器人信息失败: %s", result.Msg)
	}
	botOpenID = result.Bot.OpenID
	return nil
}

// incomingMsg 统一的入站消息上下文
type incomingMsg struct {
	MessageID    string // 触发消息 ID (回复的目标)
	ChatID       string
	ChatType     string // p2p / group
	SenderOpenID string
	Text         string // 去除 @ 占位符后的纯文本
	ParentID     string // 被回复消息的 ID
	Direct       bool   // true 时群聊中直接回复而非话题回复 (-here 后缀)
}

// textContent 序列化文本消息内容
func textContent(text string) string {
	b, _ := json.Marshal(map[string]string{"text": text})
	return string(b)
}

// sendMessage 向会话发送消息, 返回消息 ID
func sendMessage(chatID, msgType, content string) (string, error) {
	resp, err := larkClient.Im.V1.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("发送消息失败: %s", resp.Msg)
	}
	return *resp.Data.MessageId, nil
}

// replyMessage 回复消息, 返回消息 ID; messageID 为空时退化为直接发送
func replyMessage(msg incomingMsg, msgType, content string) (string, error) {
	if msg.MessageID == "" {
		return sendMessage(msg.ChatID, msgType, content)
	}
	// 群聊中以话题形式回复, 避免刷屏
	resp, err := larkClient.Im.V1.Message.Reply(context.Background(), larkim.NewReplyMessageReqBuilder().
		MessageId(msg.MessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			ReplyInThread(msg.ChatType == "group" && !msg.Direct).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		// 原消息可能已被撤回, 退化为直接发送
		return sendMessage(msg.ChatID, msgType, content)
	}
	return *resp.Data.MessageId, nil
}

func replyText(msg incomingMsg, text string) (string, error) {
	return replyMessage(msg, larkim.MsgTypeText, textContent(text))
}

func replyCard(msg incomingMsg, card string) (string, error) {
	return replyMessage(msg, larkim.MsgTypeInteractive, card)
}

// patchCard 更新已发送的卡片
func patchCard(messageID, card string) error {
	resp, err := larkClient.Im.V1.Message.Patch(context.Background(), larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(card).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("更新卡片失败: %s", resp.Msg)
	}
	return nil
}

// deleteMessage 撤回消息
func deleteMessage(messageID string) error {
	resp, err := larkClient.Im.V1.Message.Delete(context.Background(), larkim.NewDeleteMessageReqBuilder().
		MessageId(messageID).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("撤回消息失败: %s", resp.Msg)
	}
	return nil
}

// uploadIMFile 上传文件到飞书, durationMs 为音频时长(毫秒), 非音频传 0
func uploadIMFile(filePath, fileType, fileName string, durationMs int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	bodyBuilder := larkim.NewCreateFileReqBodyBuilder().
		FileType(fileType).
		FileName(fileName).
		File(file)
	if durationMs > 0 {
		bodyBuilder.Duration(durationMs)
	}
	resp, err := larkClient.Im.V1.File.Create(context.Background(), larkim.NewCreateFileReqBuilder().
		Body(bodyBuilder.Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("上传文件失败: %s", resp.Msg)
	}
	return *resp.Data.FileKey, nil
}

// uploadIMImage 上传图片, 返回 image_key
func uploadIMImage(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	resp, err := larkClient.Im.V1.Image.Create(context.Background(), larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.CreateImageImageTypeMessage).
			Image(file).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("上传图片失败: %s", resp.Msg)
	}
	return *resp.Data.ImageKey, nil
}

// replyAudio 以语音消息回复
func replyAudio(msg incomingMsg, fileKey string) (string, error) {
	b, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return replyMessage(msg, larkim.MsgTypeAudio, string(b))
}

// replyFile 以文件消息回复
func replyFile(msg incomingMsg, fileKey string) (string, error) {
	b, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return replyMessage(msg, larkim.MsgTypeFile, string(b))
}

// getMessage 获取指定消息, 返回消息类型与内容
func getMessage(messageID string) (msgType, content string, err error) {
	resp, err := larkClient.Im.V1.Message.Get(context.Background(), larkim.NewGetMessageReqBuilder().
		MessageId(messageID).
		Build())
	if err != nil {
		return "", "", err
	}
	if !resp.Success() {
		return "", "", fmt.Errorf("获取消息失败: %s", resp.Msg)
	}
	if len(resp.Data.Items) == 0 || resp.Data.Items[0].Body == nil {
		return "", "", fmt.Errorf("消息内容为空")
	}
	item := resp.Data.Items[0]
	return *item.MsgType, *item.Body.Content, nil
}

// downloadMessageResource 下载消息中的资源文件 (语音/文件)
func downloadMessageResource(messageID, fileKey, savePath string) error {
	resp, err := larkClient.Im.V1.MessageResource.Get(context.Background(), larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(fileKey).
		Type("file").
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("下载消息资源失败: %s", resp.Msg)
	}
	file, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.File)
	return err
}
