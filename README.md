<h1 align="center">Music163bot-Feishu</h1>

<h4 align="center">一个用来下载/分享/搜索网易云歌曲的飞书 bot</h4>

<p align="center">移植自 <a href="https://github.com/XiaoMengXinX/Music163bot-Go">XiaoMengXinX/Music163bot-Go</a></p>

## ✨ 特性

- 分享链接嗅探（music.163.com / 163cn.tv / 163cn.link）
- 歌曲搜索（卡片按钮选择发送方式）
- 语音 / 文件双发送方式
  - `/music`：转为 opus 语音消息，飞书内直接播放
  - `/music-file`：发送带 163key/ID3 标签的 mp3/flac 原文件
- 下载进度实时卡片
- 歌词下载、听歌识曲、电台节目
- 下载无损 flac 音频（需设置网易云 VIP 账号的 MUSIC_U）
- 歌曲缓存（file_key 复用，二次请求秒发）

## 🤖 命令

```
/music <ID|关键词|链接>       获取歌曲 (语音方式)
/music-file <ID|关键词|链接>  获取歌曲 (文件方式)
/search <关键词>              搜索歌曲
/lyric <ID|关键词|链接>       获取歌词 (.lrc)
/program <节目ID>             获取电台节目
/program-file <节目ID>        获取电台节目 (文件方式)
/recognize                    回复一条语音消息以识别歌曲
/about /status /help
/rmcache <ID>                 清除歌曲缓存 (管理员)
/reload                       重启进程 (管理员, 需配合 supervisor/docker restart)
```

## ⚙️ 飞书应用配置

1. 在[飞书开放平台](https://open.feishu.cn/app)创建企业自建应用，启用机器人能力。
2. 权限（开通后需发布新版本）：
   - `im:message`（接收与发送消息）
   - `im:message:send_as_bot`（以应用身份发消息）
   - `im:resource`（上传下载图片、文件）
   - `im:message.group_msg`（接收群聊所有消息，敏感权限；不开通则群内只能收到 @ 机器人的消息，链接嗅探失效）
3. 事件订阅：选择「使用长连接接收事件」，添加 `im.message.receive_v1`。
4. 回调订阅：卡片回调 `card.action.trigger` 同样走长连接，无需公网地址。
5. 将 `APP_ID` / `APP_SECRET` 填入 `config.ini`。

## 🛠️ 部署

需要 `ffmpeg`（语音转码与听歌识曲）。

```bash
git clone https://github.com/Octl1bra/Music163bot-Feishu
cd Music163bot-Feishu
bash build.sh
cp config_example.ini config.ini  # 填写配置
./Music163bot-Feishu -c config.ini
```

或使用 docker compose（镜像内置 ffmpeg）：

```bash
cp config_example.ini config.ini  # 填写配置
docker compose up -d
```

## 📝 注意事项

- 飞书 IM 文件上传上限 30MB：超限的无损文件会自动回退为语音方式发送。
- 语音默认以 128k/48kHz/双声道 opus 转码；若你的客户端无法播放，将配置改为官方示例参数 `OpusBitrate=16k OpusSampleRate=16000 OpusChannels=1`。
- 相比 Telegram 版，inline 模式与动态热更新（yaegi）已移除。
