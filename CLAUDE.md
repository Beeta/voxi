# 有听有记 Voxi

macOS 录音 → 转写 → 摘要 → 脑图 → 问答的桌面应用。Go + [mygo](https://github.com/egoist/mygo)，
入口 `cmd/voxi`，界面代码在 `internal/app`。构建、运行、签名与首次运行说明见 `README.md`。

## 版本与 CHANGELOG

版本号有实际后果，不只是元数据：它是 `mygo.json` 的 `version`，经 ldflags 注入后
显示在设置弹窗底部，也是 `mygo build` 产出文件名的一部分（`有听有记Voxi 0.2.0.dmg`）。

**唯一真源是 `mygo.json` 的 `version`。** 改它就必须在同一个提交里改 `CHANGELOG.md`，
两者对不上就是 bug。

### 每次提交都动版本号

用户能感知的改动，**每次提交都要 bump `MAJOR.MINOR.PATCH`**：

| 改动 | 动哪一位 | 例子 |
| --- | --- | --- |
| 大的调整 | MAJOR | 界面整体重构、换掉底层架构 |
| 增加功能 | MINOR | 支持导入音频文件 |
| 问题修复 / 性能优化 | PATCH | 修复点时间戳不跳转 |

**纯文档、测试、重构不动版本号**——用户看不见它们，版本号要对应「用户拿到的是哪个东西」。

`0.x` 阶段没有特殊约定：大的调整直接进 `1.x`（`0.2.0` → `1.0.0`），此后按
`1.1.0`（新功能）、`1.1.1`（修复）、`2.0.0`（又一次大调整）这样走。

### 同一个提交里定版 CHANGELOG

bump 版本号的那次提交，**在同一个提交里**做完两件事：

1. 改 `mygo.json` 的 `version`
2. 把 `## [未发布]` 改成 `## [X.Y.Z] - YYYY-MM-DD`，并在上面新起一个空的 `## [未发布]`

不改版本号的提交（文档、测试、重构）只往未发布节加「内部」条目，攒到下次代码改动时
一起定版。

写具体做了什么、为什么改。不要写「优化了用户体验」这类空话。

### tag 只跟 MAJOR / MINOR 走

**只有 MAJOR 或 MINOR 变动才打 tag，PATCH 不打。**

```sh
git tag -a vX.Y.Z -m "有听有记 Voxi X.Y.Z" -m "<一句话摘要>"
```

tag 是本地操作，不 push。必须打在包含版本号变更的提交上——打在别处，
`git checkout vX.Y.Z` 拿到的代码里版本号还是旧的。

PATCH 版本没有 tag，所以某个 PATCH 构建出的 dmg 若给了别人，那个版本号在 git 里
查不到对应提交——要追溯就靠 CHANGELOG 的日期和提交信息。

小节分类的判定细则、格式模板与示例见 `docs/changelog-guide.md`。
