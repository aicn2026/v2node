# v2node
A v2board backend base on moddified xray-core.
一个基于修改版xray内核的V2board节点服务端。

**注意： 本项目需要搭配[修改版V2board](https://github.com/wyx2685/v2board)**

## 软件安装

### 一键安装

```
wget -N https://raw.githubusercontent.com/wyx2685/v2node/master/script/install.sh && bash install.sh
```

## 构建
``` bash
# Linux 节点二进制（Reality 需要 with_utls，TUIC 需要 with_quic）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "with_quic,with_utls" -trimpath -ldflags "-s -w -buildid=" -o v2node-linux-amd64 .
```

## Stars 增长记录

[![Stargazers over time](https://starchart.cc/wyx2685/v2node.svg?variant=adaptive)](https://starchart.cc/wyx2685/v2node)
