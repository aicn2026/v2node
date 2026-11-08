# v2node
A V2board backend based on embedded sing-box (panel API unchanged).
һ������Ƕ��ʽ sing-box ������� V2board �ڵ����ˣ����ԽӲ��䣩��
֧��Э�飺VLESS Reality��TUIC��Hysteria2��Shadowsocks��

**ע�⣺ ����Ŀ��Ҫ����[�޸İ�V2board](https://github.com/wyx2685/v2board)**

## �����װ

### һ����װ

```
# ���ڵ�
wget -N https://raw.githubusercontent.com/aicn2026/v2node/feat/singbox-dataplane/script/install.sh && bash install.sh --api-host https://example.com --api-key YOUR_KEY --node-id 1

# ��ڵ㣨ͬһ���/��Կ����� NodeID��
wget -N https://raw.githubusercontent.com/aicn2026/v2node/feat/singbox-dataplane/script/install.sh && bash install.sh --api-host https://example.com --api-key YOUR_KEY --node-id 1,2,3
```

## ����
``` bash
# Linux �ڵ�����ƣ�Reality ��Ҫ with_utls��TUIC ��Ҫ with_quic��
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "with_quic,with_utls" -trimpath -ldflags "-s -w -buildid=" -o v2node-linux-amd64 .
```
