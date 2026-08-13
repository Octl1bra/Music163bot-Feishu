package bot

import (
	"regexp"

	"github.com/XiaoMengXinX/Music163Api-Go/utils"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	"gorm.io/gorm"
)

// MusicDB 音乐缓存数据库入口
var MusicDB *gorm.DB

// config 配置文件数据
var config map[string]string

// data 网易云 cookie
var data utils.RequestData

// larkClient 飞书 API 客户端
var larkClient *lark.Client

// botAdmin 管理员 open_id 列表
var botAdmin []string

var cacheDir = "./cache"

// maxRetryTimes 最大重试次数, downloaderTimeout 下载超时时间
var maxRetryTimes, downloaderTimeout int

var (
	reg1   = regexp.MustCompile(`(.*)song\?id=`)
	reg2   = regexp.MustCompile("(.*)song/")
	regP1  = regexp.MustCompile(`(.*)program\?id=`)
	regP2  = regexp.MustCompile("(.*)program/")
	regP3  = regexp.MustCompile(`(.*)dj\?id=`)
	regP4  = regexp.MustCompile("(.*)dj/")
	reg5   = regexp.MustCompile("/(.*)")
	reg4   = regexp.MustCompile("&(.*)")
	reg3   = regexp.MustCompile(`\?(.*)`)
	regInt = regexp.MustCompile(`\d+`)
	regUrl = regexp.MustCompile("(http|https)://[\\w\\-_]+(\\.[\\w\\-_]+)+([\\w\\-.,@?^=%&:/~+#]*[\\w\\-@?^=%&/~+#])?")
	// 群消息中的 @ 占位符
	regMention = regexp.MustCompile(`@_user_\d+`)
)

// 发送方式
const (
	modeVoice = "voice" // 语音消息 (opus, 可在飞书内直接播放)
	modeFile  = "file"  // 文件消息 (mp3/flac 原文件)
)

var (
	aboutText = `Music163bot-Feishu
移植自: https://github.com/XiaoMengXinX/Music163bot-Go
Github: https://github.com/Octl1bra/Music163bot-Feishu

[编译环境] %s
[编译版本] %s
[编译哈希] %s
[编译日期] %s
[运行环境] %s`
	musicInfoMsg = `%s
专辑: %s
%s %.2fMB`
	uploadFailed = `下载/发送失败
%v`
	statusInfo = `[统计信息]
数据库中总缓存歌曲数量: %d
当前对话缓存歌曲数量: %d
当前用户缓存歌曲数量: %d`
	rmcacheReport    = `清除 [%s] 缓存成功`
	inputKeyword     = "请输入搜索关键词"
	inputIDorKeyword = "请输入歌曲ID或歌曲关键词"
	inputContent     = "请输入歌曲关键词/歌曲分享链接/歌曲ID"
	noResults        = `未找到结果`
	noCache          = `歌曲未缓存`
	hitCache         = `命中缓存, 正在发送中...`
	getLrcFailed     = `获取歌词失败, 歌曲可能不存在或为纯音乐`
	getUrlFailed     = `获取歌曲下载链接失败`
	fetchInfo        = `正在获取歌曲信息...`
	fetchInfoFailed  = `获取歌曲信息失败`
	waitForDown      = `等待下载中...`
	downloading      = `下载中...`
	downloadStatus   = " %s\n%.2fMB/%.2fMB %d%%"
	redownloading    = `下载失败，尝试重新下载中...`
	uploading        = `下载完成, 发送中...`
	converting       = `正在转换为语音格式...`
	md5VerFailed     = "MD5校验失败"
	retryLater       = "请稍后重试"
	reloading        = "正在重启进程..."
	fetchingLyric    = "正在获取歌词中"
	downloadTimeout  = `下载超时`
	replyToVoice     = "请回复一条语音消息"
	recognizeFailed  = "识别失败，可能是录音时间太短"
	fileTooLarge     = "文件大小 %.2fMB 超过飞书 30MB 上限, 已自动改用语音方式发送"
	ffmpegMissing    = "未找到 ffmpeg, 无法转换语音格式, 请安装 ffmpeg 或使用 /music-file"
	searchTips       = "点击下方按钮选择发送方式"
	helpText         = `使用说明:
/music <ID|关键词|链接> - 获取歌曲 (语音方式)
/music-file <ID|关键词|链接> - 获取歌曲 (文件方式)
/search <关键词> - 搜索歌曲
/lyric <ID|关键词|链接> - 获取歌词
/program <节目ID> - 获取电台节目
/recognize - 回复语音消息以识别歌曲
/about - 关于
/status - 统计信息

群聊中默认以话题形式回复, 命令加 -here 后缀改为直接回复 (如 /music-here)
群聊中 @ 机器人 + 关键词/ID 可省略 /music 直接点歌
也可以直接发送网易云歌曲分享链接`
)
