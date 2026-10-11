# TODO

后续要做的功能。每条记录：现状（已有什么）、要做的事、风险与待定问题。

## 1. 支持直接导入音频文件

**现状**

- `asr.Transcribe(path, duration)` 只吃文件路径，`detectFormat` 已支持 `wav / mp3 / ogg / spx / amr / aac / m4a`，默认按 m4a 处理（`internal/asr/asr.go`）。
- 播放走 macOS `AVAudioPlayer`（`internal/audio/player_darwin.go`），上述格式同样能播。
- `store.Add(name, file, duration)` 登记一条记录，`File` 存绝对路径。

**要做的事**

- 加"导入音频"入口（录音页或侧边栏），选文件 → 探测时长 → 登记记录 → 复用现有转写流程。
- 时长可用 `AVAudioPlayer.Duration` 拿（需先 `loadPlayer`），不必自己解析容器。

**风险 / 待定**

- 复制进库目录 vs 原地引用：原地引用时用户删掉/移动源文件后记录就失效；建议默认复制，库目录由第 3 条配置决定。
- 大文件走 cloudflared 隧道上传的耗时与稳定性，需要给明确进度和超时提示。
- 非音频/损坏文件、超长文件的报错路径要覆盖。

## 2. 转写页顶部显示相关人（可折叠）

**现状**

- `store.Segment.Speaker` 已存在，ASR 输出形如 `说话人1`（`internal/asr/asr.go`）。
- 已有逐段改名和"同名的全部改"的 modal（`internal/app/transcriptedit.go`、`store.RenameSpeaker`）。

**要做的事**

- 从当前 transcript 的 segments 去重聚合出说话人列表（按首次出现顺序，附段数/时长），在转写页顶部渲染可折叠面板。
- 面板里点某人可改名，走已有的 `RenameSpeaker(..., global=true)`，改完联动刷新列表和正文。

**风险 / 待定**

- 列表必须是 segments 的派生结果，不要单独持久化，否则改名后两处会不一致。
- "改一个人"的语义要写清楚：标签是共享的，改名会改掉所有用该标签的段落。

## 3. 设置里支持配置录音文件存放路径

**现状**

- 库索引 `recordings.json` 和录音文件都落在 `mygo.App.Path(PathUserData)`（`internal/store/store.go`、`internal/app/settings.go`）。
- `Recording.File` 存的是绝对路径。

**要做的事**

- `Settings` 加录音目录字段；设置页加目录选择与校验（存在、可写）。
- 录制/导入时写入该目录；界面明确提示"改路径后旧录音是否迁移"。

**风险 / 待定**

- 绝对路径的硬伤：目录一搬，老记录全指向失效位置。建议改成 `File` 存相对路径 + 设置里存根目录，长期更稳。
- 索引文件是否跟着音频走：建议 `recordings.json` / `settings.json` 留在 app data，只有音频文件跟配置目录走，避免设置文件自我引用。
- 首次切换目录时提供一次性迁移（复制/移动 + 改写索引），否则用户体验会断。

## 4. 录制过程中支持暂停 / 继续

**现状**

- 状态机只有 `Idle / Starting / Recording / Stopping`（`internal/audio/recorder.go`）。
- `deliver` 已经在 `state != Recording && state != Starting` 时直接丢帧，所以"暂停"天然可以复用它来丢弃音频。
- 时长靠 `time.Since(r.started)` 算，`tick()` 每 500ms 刷新，`state != Recording` 就停。
- 音频是边到边编码的流式写入，文件时长由写入的样本数决定。

**要做的事**

- 加 `StatePaused`；暂停时**保持 capture 运行**（ScreenCaptureKit 重启有成本和权限风险），只让 `deliver` 丢帧。
- `Recorder` 记 `pausedAt` / `pausedTotal`，`Snapshot` 的 elapsed 排除暂停时间；`tick()` 把 `Paused` 也当作需要继续刷新的状态；`Stop()` 允许从 `Paused` 进入。
- UI 加暂停/继续按钮与状态文案，暂停时冻结计时显示。

**风险 / 待定**

- **mixer 的 pending 残留**：mixer 只吐出"所有源都已投递"的那一段。若在暂停瞬间一个源已投、另一个还没投，恢复后新样本会把暂停前的那半段拼进来，产生一小段跨暂停的杂音。暂停时要显式清掉/flush pending。
- 好消息是时长能自洽：文件长度按写入样本数算，只要 elapsed 也排除暂停时间，转写用的 `duration` 仍与音频实际长度一致。
- 暂停期间系统音频仍在投递（只是被丢弃），长时间暂停要不要干脆停掉 capture 值得实测后再定。
- 暂停时若采集设备被系统切换（拔耳机等），当前实现不会察觉，恢复后可能源已变。
