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

## 5. 笔记支持时间标签，点击跳转到对应音频位置

**现状**

- `Recording.Notes` 是一个纯字符串，`notesView` 用一个 `TextArea` 编辑、`SetNotes` 随改随存（`internal/app/notes.go`、`internal/store/store.go`）。
- **跳转能力已经现成**：`a.seekTo(sec)` 就是"定位 + 开始播放"，转写页点段落时间戳用的就是它（`internal/app/actions.go`、`internal/app/playback.go`）。缺的只是"笔记里的时间 ↔ 那个秒数"的绑定。
- 当前播放位置有 `a.playPos`，播放中会跟着 `p.CurrentTime()` 更新，取"此刻时间"不用额外做事。

**要做的事（先定数据模型，见下）**

- 加一个"插入当前时间"的按钮：把当前 `playPos` 写成一个时间标签。
- 时间标签渲染成可点元素，点击调 `seekTo`。
- 时间显示格式统一（建议与转写页一致用 `formatSecs`）。

**风险 / 待定**

- **这是本条最关键的决策：笔记存成字符串还是结构化数据。**
  - 保持字符串 + 内联语法（如 `[00:12]`）：改动小、保留自由多行编辑，但 `TextArea` 是纯文本组件，**没法在里面内联可点击的标签**——只能在旁边再渲染一份解析出来的标签列表，或者做"点标签跳到正文对应位置"的两段式交互。手写或粘贴破坏语法时要能优雅降级。
  - 改成结构化的 `[]Note{At, Text}` 列表：时间标签是天然的可点元素，体验最好，但要迁移现有 `Notes` 字符串，并且失去"一整块自由文本"的编辑手感（除非做成分行列表）。
- 点标签是"跳转并播放"还是"只定位不播放"？现有 `seekTo` 是跳转即播放，在笔记场景下可能不合预期，值得单独定。
- 精度：转写段落是秒级，笔记标签沿用秒级即可，但要不要保留小数位（同一秒多条笔记）要定。
- 时间标签会和音频长度绑定，一旦将来支持替换/重录音频就会失准——现在不用处理，但数据结构上别写死成不可迁移的形式。
