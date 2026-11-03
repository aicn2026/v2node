#!/usr/bin/env bash
# 离线安装：从 /tmp/v2node-linux-64.zip 安装 v2node（不依赖 GitHub）
# 用法:
#   bash install-local.sh
#   bash install-local.sh --api-host https://example.com --api-key KEY --node-id 1,2,3
#   bash install-local.sh --api-host https://example.com --api-key KEY --node-id 1 --node-id 2
#   ZIP=/tmp/v2node-linux-64.zip bash install-local.sh ...

set -euo pipefail

ZIP="${ZIP:-/tmp/v2node-linux-64.zip}"
API_HOST="https://example.com"
API_KEY="YOUR_KEY"
NODE_IDS=()

append_node_ids() {
  local raw="$1"
  raw="${raw//,/ }"
  local id
  for id in $raw; do
    [[ -z "$id" ]] && continue
    if [[ ! "$id" =~ ^[0-9]+$ ]]; then
      echo "无效节点ID: $id"
      exit 1
    fi
    NODE_IDS+=("$id")
  done
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-host) API_HOST="$2"; shift 2 ;;
    --node-id)  append_node_ids "$2"; shift 2 ;;
    --api-key)  API_KEY="$2"; shift 2 ;;
    --zip)      ZIP="$2"; shift 2 ;;
    -h|--help)
      echo "用法: bash $0 [--zip PATH] [--api-host URL] [--api-key KEY] [--node-id ID[,ID...]]"
      exit 0
      ;;
    *)
      echo "未知参数: $1"
      exit 1
      ;;
  esac
done

if [[ ${#NODE_IDS[@]} -eq 0 ]]; then
  NODE_IDS=(1)
fi

if [[ ! -f "$ZIP" ]]; then
  echo "找不到安装包: $ZIP"
  exit 1
fi

echo "==> 解压 $ZIP"
WORKDIR="$(mktemp -d /tmp/v2node-install.XXXXXX)"
trap 'rm -rf "$WORKDIR"' EXIT
unzip -o "$ZIP" -d "$WORKDIR"

BIN=""
if [[ -f "$WORKDIR/v2node" ]]; then
  BIN="$WORKDIR/v2node"
else
  BIN="$(find "$WORKDIR" -type f -name v2node | head -n1 || true)"
fi
if [[ -z "$BIN" || ! -f "$BIN" ]]; then
  echo "压缩包内未找到 v2node 二进制"
  exit 1
fi

echo "==> 安装二进制到 /usr/local/v2node/"
mkdir -p /usr/local/v2node /etc/v2node
install -m 755 "$BIN" /usr/local/v2node/v2node

echo "==> 写入 /etc/v2node/config.json (${#NODE_IDS[@]} 个节点)"
nodes_json=""
i=0
for id in "${NODE_IDS[@]}"; do
  if [[ $i -gt 0 ]]; then
    nodes_json+=",
"
  fi
  nodes_json+="        {
            \"ApiHost\": \"${API_HOST}\",
            \"NodeID\": ${id},
            \"ApiKey\": \"${API_KEY}\",
            \"Timeout\": 15
        }"
  i=$((i + 1))
done

cat > /etc/v2node/config.json <<EOF
{
    "Log": {
        "Level": "warning",
        "Output": "",
        "Access": "none"
    },
    "Nodes": [
${nodes_json}
    ]
}
EOF

echo "==> 写入 systemd 服务"
cat > /etc/systemd/system/v2node.service <<'EOF'
[Unit]
Description=v2node Service
After=network.target nss-lookup.target
Wants=network.target

[Service]
User=root
Group=root
Type=simple
LimitAS=infinity
LimitRSS=infinity
LimitCORE=infinity
LimitNOFILE=999999
WorkingDirectory=/usr/local/v2node/
ExecStart=/usr/local/v2node/v2node server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable v2node
systemctl restart v2node
sleep 1

echo "==> 版本"
/usr/local/v2node/v2node version || true
echo "==> 状态"
systemctl status v2node --no-pager -l || true
echo
echo "安装完成。日志: journalctl -u v2node -f"
