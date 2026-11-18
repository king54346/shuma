# 數碼精靈 Go 本地服务端

运行和测试

```powershell
cd C:\Users\31483\Desktop\shuma\server
go test ./...
go run . -bind 127.0.0.1
```

雷电模拟器中的 APK 使用 `127.0.0.1`，通过 ADB reverse 连接主机上的 Go 进程。

```powershell
adb reverse tcp:18080 tcp:18080
adb reverse tcp:1104 tcp:1104
adb reverse tcp:1144 tcp:1144
adb reverse tcp:16666 tcp:16666
adb reverse tcp:10888 tcp:10888
```

| 端口 | 服务 |
| --- | --- |
| `18080` | HTTP 版本、区服、公告和 SDK 兼容响应 |
| `1104` | HTTP 上报和反馈 |
| `1144` | HTTP 敏感词检查 |
| `16666` | TCP 登录服 |
| `10888` | TCP 游戏服 |

并行联调时可用 `-port-offset 20000` 让候选进程监听另一组主机端口。ADB reverse 的设备侧端口保持为上表的值，主机侧端口加上偏移。抓包时可将登录和游戏的 ADB reverse 分别指向本机 TCP 转发端口，由转发进程按连接保存双向字节流。

模块和验收顺序见 [复刻计划](PLAN.md)。
