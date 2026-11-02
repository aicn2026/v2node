# v2node
A V2board backend based on embedded sing-box (panel API unchanged).
一个基于嵌入式 sing-box 数据面的 V2board 节点服务端（面板对接不变）。
支持协议：VLESS Reality、TUIC、Hysteria2、Shadowsocks。

**注意： 本项目需要搭配[修改版V2board](https://github.com/wyx2685/v2board)**

## 软件安装

### 一键安装

```
wget -N https://raw.githubusercontent.com/mouren1988/v2node/feat/singbox-dataplane/script/install.sh && bash install.sh
```

## 构建
``` bash
# Linux 节点二进制（Reality 需要 with_utls，TUIC 需要 with_quic）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "with_quic,with_utls" -trimpath -ldflags "-s -w -buildid=" -o v2node-linux-amd64 .
```
