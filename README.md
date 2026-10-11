# 有听有记 Voxi

一个 macOS 会议助手，用 [`mygo`](https://github.com/egoist/mygo) 的 `ui` 包绘制原生界面：录制会议音频，把录音管理成一个资料库，调用语音识别把它转写成带时间线的文字，再在转写之上生成摘要、脑图、问答和笔记。

## 构建与运行

```sh
go run ./cmd/voxi                                       # 开发：直接跑
go run github.com/egoist/mygo/cmd/mygo@v0.3.5 build .   # 打包成 .app
```

需要 Go 1.27+。mygo 是普通 Go module 依赖，由 `go.mod` 拉取，**不需要克隆 mygo 仓库**。

## 目录结构

```
cmd/voxi/         程序入口
internal/app/     界面与状态：录音库、播放器，以及录制 / 转写 / 摘要 / 脑图 / 问答 / 笔记
internal/audio/   音频捕获、AAC 编码与播放（macOS 部分走 ScreenCaptureKit + purego，无需 cgo）
internal/store/   录音库的 JSON 索引与数据模型
internal/asr/     火山引擎「豆包语音」录音文件识别
internal/llm/     OpenAI 兼容的大模型调用（摘要 / 脑图 / 问答 / 连接验证）
internal/tunnel/  cloudflared quick tunnel，供 ASR 从公网取音频
```

`internal/` 下的包不被外部导入；界面代码集中在 `internal/app`，因为它需要访问应用状态的私有字段。

## 录制

左侧边栏顶部有「录制」按钮，下面一行小字显示当前要捕获的音频源；音频源——麦克风，以及系统输出（扬声器或耳机正在播放的声音）——改在右上角齿轮打开的设置里勾选。点击录制开始捕获所选音频源，点击停止结束。所选音频源会被混合成一个 AAC（`.m4a`）文件，并通过 `ExtAudioFile`（AudioToolbox，经 purego 驱动——无需 cgo）边录边编码，而不是全部堆在内存里，因此无论录多久内存占用都保持平稳。输出为 48 kHz 立体声、128 kbps，体积只有未压缩 WAV 的一小部分。

音频捕获的排障日志默认关闭；设置环境变量 `VOXI_DEBUG=1` 会把它追加写入 `~/voxi-debug.log`。

## 设置

点侧边栏右上角的齿轮打开设置弹窗，分三部分：

- **录制**：勾选要捕获的音频源（麦克风、系统输出）。
- **语音识别（豆包语音）**：填写 [豆包语音](https://console.volcengine.com/speech/new/experience/asr) 的 API Key 与 cloudflared 可执行文件路径（默认 `/opt/homebrew/bin/cloudflared`），用于录音转写。
- **大模型（LLM）**：配置一个 OpenAI 兼容的接入点——Base URL、API Key、模型 ID，驱动摘要 / 脑图 / 问答；可点「验证连接」向该地址发一个最小请求实测是否可用。

设置持久化在应用数据目录的 `settings.json`。注意：ASR 与 LLM 的 API Key 以**明文**保存在该文件里，界面上也直接明文显示（文件仅当前用户可读）；如果在意，别在会被备份 / 同步或共享的机器上填入敏感密钥。

## 管理与识别

左侧是录音库，右侧是选中录音的详情。详情页顶部是可拖动进度条的播放器，下面是分页签：

- **转写**：调用豆包语音把录音识别成带时间戳的分段文字；点时间戳跳转播放、点文字就地编辑（自动保存）、点说话人改名（可选「仅此条」或「全部相同」）；顶部「重新识别」可重跑（会覆盖当前转写）。录音文件原样（`.m4a`）提交，不做转码（见下）。
- **摘要**：把转写发给大模型，生成「概览 + 关键要点 + 待办事项」，生成指令（prompt）可编辑。
- **脑图**：把转写整理成可展开/折叠的大纲树。
- **问答**：基于转写内容多轮提问，回答流式逐字返回。
- **笔记**：每条录音一段自由文本，自动保存。

录音库、转写、摘要、脑图、问答和笔记都持久化在应用数据目录的 `recordings.json` 里；重新识别后，已生成的摘要和脑图会被标记为「可能过期」，提示重新生成。

> 识别与大模型都已接入真实服务，在设置里配置：
>
> - **语音识别**用火山引擎「豆包语音」的录音文件识别（auc）。该接口从公网 URL 拉取音频，所以识别时会在本地起一个只绑 `127.0.0.1`、路径带随机 token 的临时 HTTP 服务，再用 [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) quick tunnel 临时映射到公网供火山拉取，识别完即关闭。**需预先安装 cloudflared**（`brew install cloudflared`）；GUI `.app` 不继承终端 `$PATH`，默认按 `/opt/homebrew/bin/cloudflared` 等常见位置查找，装在别处可在设置里指定路径。注意：识别期间录音会短暂地在一个不可猜测的公网地址上可被取到、并由火山服务器下载，隐私敏感的录音请斟酌。
> - **大模型**走设置里填的 OpenAI 兼容接入点（摘要 / 脑图非流式，问答流式）。

音频捕获使用 ScreenCaptureKit，仅支持 macOS；混入麦克风需要 macOS 15 及以上。在其他平台上窗口依然会打开，并说明无法捕获。

以实时热重载方式开发，或打包：

```sh
go run github.com/egoist/mygo/cmd/mygo@v0.3.5 dev .
go run github.com/egoist/mygo/cmd/mygo@v0.3.5 build .
```

`go install github.com/egoist/mygo/cmd/mygo@v0.3.5` 之后可以直接写成 `mygo dev .` / `mygo build .`。

该应用会录制麦克风和系统音频，因此它的应用包声明了 `NSMicrophoneUsageDescription` 和 `NSAudioCaptureUsageDescription`（见 `mygo.json`）。macOS 只会把这些权限授予 `Info.plist` 中带有相应用途说明的应用包，所以录制功能需在 `mygo build` 产出的 `.app` 中才能工作——裸 `go run` 出来的二进制不行，系统会在它访问麦克风时将其终止。请用 `mygo build`（或 `dev`）来测试录制。
