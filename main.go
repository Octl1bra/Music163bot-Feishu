package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/Octl1bra/Music163bot-Feishu/bot"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

var config map[string]string

var _ConfigPath *string

var (
	runtimeVer   = fmt.Sprintf(runtime.Version())                     // 编译环境
	_VersionName = ""                                                 // 程序版本
	commitSHA    = ""                                                 // 编译哈希
	buildTime    = ""                                                 // 编译日期
	buildArch    = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH) // 运行环境
)

// LogFormatter 自定义 log 格式
type LogFormatter struct{}

// Format 自定义 log 格式
func (s *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timestamp := time.Now().Local().Format("2006/01/02 15:04:05")
	var msg string
	msg = fmt.Sprintf("%s [%s] %s (%s:%d)\n", timestamp, strings.ToUpper(entry.Level.String()), entry.Message, path.Base(entry.Caller.File), entry.Caller.Line)
	return []byte(msg), nil
}

func init() {
	logrus.SetFormatter(new(LogFormatter))
	logrus.SetReportCaller(true)
	dirExists("./log")
	timeStamp := time.Now().Local().Format("2006-01-02")
	logFile := fmt.Sprintf("./log/%v.log", timeStamp)
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logrus.Errorln(err)
	}
	output := io.MultiWriter(os.Stdout, file)
	logrus.SetOutput(output)

	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	_ConfigPath = f.String("e", ".env", "env 文件路径 (不存在时仅读取环境变量)")
	_ = f.Parse(os.Args[1:])

	logrus.Printf("Music163bot-Feishu %s", _VersionName)

	config = readConfig(*_ConfigPath)
	initConfig(config)

	if config["LogLevel"] != "" {
		level, err := logrus.ParseLevel(config["LogLevel"])
		if err != nil {
			logrus.Errorln(err)
		} else {
			logrus.SetLevel(level)
		}
	}
}

func main() {
	switch bot.Start(config) {
	case 1:
		logrus.Fatal("Unexpected error")
	default:
		os.Exit(0)
	}
}

// envKeys 环境变量名 -> bot 包内部配置键
var envKeys = map[string]string{
	"APP_ID":           "APP_ID",
	"APP_SECRET":       "APP_SECRET",
	"MUSIC_U":          "MUSIC_U",
	"BOT_ADMIN":        "BotAdmin",
	"BOT_DEBUG":        "BotDebug",
	"DATABASE":         "Database",
	"LOG_LEVEL":        "LogLevel",
	"MAX_RETRY_TIMES":  "MaxRetryTimes",
	"DOWNLOAD_TIMEOUT": "DownloadTimeout",
	"REVERSE_PROXY":    "ReverseProxy",
	"OPUS_BITRATE":     "OpusBitrate",
	"OPUS_SAMPLE_RATE": "OpusSampleRate",
	"OPUS_CHANNELS":    "OpusChannels",
}

// readConfig 读取配置: 环境变量优先, .env 文件兜底 (godotenv.Load 不覆盖已存在的环境变量)
func readConfig(path string) map[string]string {
	if _, err := os.Stat(path); err == nil {
		if err := godotenv.Load(path); err != nil {
			logrus.Errorln("读取 env 文件失败，请检查文件格式")
			logrus.Fatal(err)
		}
	}
	config := make(map[string]string)
	for env, key := range envKeys {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			config[key] = value
		}
	}
	return config
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			logrus.Errorf("mkdir %v failed: %v\n", path, err)
		}
		return false
	}
	logrus.Errorf("Error: %v\n", err)
	return false
}

func initConfig(config map[string]string) {
	config["BinVersionName"] = _VersionName
	config["runtimeVer"] = runtimeVer
	config["buildTime"] = buildTime
	config["commitSHA"] = commitSHA
	config["buildArch"] = buildArch
}
