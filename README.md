# shapez-auto

用外部大模型规划产线，自动摆放建筑并加速打通 Steam 版 [shapez.io](https://github.com/tobspr-games/shapez.io) 的主线。

控制面是一个 Windows 程序。它读取游戏模组上报的关卡目标和附近资源，每进入新关卡就清空流水线，再向 OpenAI 兼容接口要一份加工步骤。模型的结果会按游戏规则演算，对不上目标就继续请模型重做，不改用内置求解。然后把切割器、旋转器、染色器、混色器、堆叠器和传送带直接放进当前局，并加快游戏逻辑，直到主线结束。

网页版 shapez.io 没有模组接口，不能用。

## 安装

1. 从仓库的 Releases 下载 `shapez-auto-windows-amd64-setup.exe` 并安装。
2. 打开 shapez-auto。浏览器会进入本机控制面 `http://127.0.0.1:17831/`。
3. 填写外部模型的接口地址和 API Key。地址需要兼容 `POST /v1/chat/completions`。点「探测可用模型」会请求该地址的 `/models`，再从列表里选择模型。每关的产线都由这个模型设计。
4. 点击「安装游戏模组」。模组会写到 `%APPDATA%\shapez.io\mods\shapez-auto.js`。
5. 完全退出并重新打开 Steam 版 shapez.io（需要 1.5 及以上的模组支持）。进入一局游戏。
6. 回到控制面，点击「开始自动通关」。

主线默认打到第 26 关，也就是自由模式解锁。吞吐关会自动把加速倍数提到至少 16。升级会在材料够的时候自动购买。

## 本地构建

发布由 GitHub Actions 在 `windows-latest` 上完成：`go test`、编译 `windows/amd64`，再用 Inno Setup 打安装包。给版本打 `v*` 标签时，安装包会附到 Release。

不要在本机执行 `go build`。
