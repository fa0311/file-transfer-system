# ファイル転送システム

Go と gRPC を使用したサーバー間のファイル転送システムです。

## 概要

このシステムは以下の機能を提供します：

- サーバー A/B が相互にファイルを転送
- クライアント C が HTTP 経由で転送を指示
- ワイルドカード対応で複数ファイルの転送が可能
- 大容量ファイル（30GB 以上）のストリーミング転送
- JSONL 形式でのリアルタイム進捗報告
- SHA256 チェックサムによるデータ整合性検証

## アーキテクチャ

```
クライアントC --[HTTP POST]--> サーバーA --[gRPC Stream]--> サーバーB
クライアントC --[HTTP POST]--> サーバーB --[gRPC Stream]--> サーバーA
```

## 必要な環境変数

### サーバー A

```bash
GRPC_LISTEN_ADDR=0.0.0.0:50051
HTTP_LISTEN_ADDR=0.0.0.0:8080
TARGET_SERVER=server-b:50051
ALLOWED_DIR=/data
```

### サーバー B

```bash
GRPC_LISTEN_ADDR=0.0.0.0:50051
HTTP_LISTEN_ADDR=0.0.0.0:8080
TARGET_SERVER=server-a:50051
ALLOWED_DIR=/data
```

## ビルド

### ローカルビルド

```bash
# 依存関係のインストール
go mod tidy

# Protocol Buffersの生成
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/transfer.proto

# ビルド
go build -o server ./cmd/server
```

### Docker ビルド

```bash
docker build -t file-transfer-server .
```

## 実行方法

### ローカル実行

#### サーバー A の起動

```bash
export GRPC_LISTEN_ADDR=0.0.0.0:50051
export HTTP_LISTEN_ADDR=0.0.0.0:8080
export TARGET_SERVER=localhost:50052
export ALLOWED_DIR=/data

./server
```

#### サーバー B の起動

```bash
export GRPC_LISTEN_ADDR=0.0.0.0:50052
export HTTP_LISTEN_ADDR=0.0.0.0:8081
export TARGET_SERVER=localhost:50051
export ALLOWED_DIR=/data

./server
```

### Docker Compose での実行

`docker-compose.yml`を作成：

```yaml
version: "3.8"

services:
  server-a:
    image: ghcr.io/your-org/file-transfer-server:latest
    container_name: file-transfer-server-a
    environment:
      - GRPC_LISTEN_ADDR=0.0.0.0:50051
      - HTTP_LISTEN_ADDR=0.0.0.0:8080
      - TARGET_SERVER=server-b:50051
      - ALLOWED_DIR=/data
    volumes:
      - ./data-a:/data
    ports:
      - "8080:8080"
      - "50051:50051"
    networks:
      - transfer-network
    restart: unless-stopped

  server-b:
    image: ghcr.io/your-org/file-transfer-server:latest
    container_name: file-transfer-server-b
    environment:
      - GRPC_LISTEN_ADDR=0.0.0.0:50051
      - HTTP_LISTEN_ADDR=0.0.0.0:8080
      - TARGET_SERVER=server-a:50051
      - ALLOWED_DIR=/data
    volumes:
      - ./data-b:/data
    ports:
      - "8081:8080"
      - "50052:50051"
    networks:
      - transfer-network
    restart: unless-stopped

networks:
  transfer-network:
```

起動：

```bash
# サービスの起動
docker-compose up -d

# ログの確認
docker-compose logs -f

# サービスの停止
docker-compose down
```

## API 使用方法

### ファイル転送のリクエスト

**エンドポイント:** `POST /transfer`

**リクエスト例:**

```bash
# 単一ファイルの転送
curl -X POST http://server-a:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "/data/video.mp4",
    "dest_path": "/data/received/"
  }'

# ワイルドカードを使用した複数ファイルの転送
curl -X POST http://server-a:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "/data/videos/*.mp4",
    "dest_path": "/data/received/"
  }'
```

**レスポンス:**

JSONL (JSON Lines) 形式でリアルタイム進捗を返します：

```
{"type":"info","message":"Transfer started","time":"2024-11-16T18:30:00Z"}
{"type":"progress","message":"Transferring...","time":"2024-11-16T18:30:01Z"}
{"type":"completed","message":"Transfer completed successfully","time":"2024-11-16T18:30:10Z"}
```

### ヘルスチェック

**エンドポイント:** `GET /health`

```bash
curl http://server-a:8080/health
```

**レスポンス:**

```json
{
  "status": "healthy",
  "timestamp": "2024-11-16T18:30:00Z"
}
```

## セキュリティ機能

1. **パストラバーサル攻撃防止**

   - すべてのファイルパスは`ALLOWED_DIR`内に制限
   - `..`などの不正なパスは拒否

2. **チェックサム検証**
   - 各チャンク転送時に SHA256 チェックサムを検証
   - データの整合性を保証

## 技術仕様

- **チャンクサイズ:** 1MB
- **転送プロトコル:** gRPC 双方向ストリーミング
- **進捗報告:** JSONL (JSON Lines)
- **リトライ:** 最大 3 回（自動リトライ）

## トラブルシューティング

### ピア接続エラー

```
Failed to connect to peer after 10 attempts
```

**解決方法:**

1. `TARGET_SERVER`の設定を確認
2. ネットワーク接続を確認
3. 相手サーバーが起動しているか確認

### パス検証エラー

```
path is outside allowed directory
```

**解決方法:**

1. `ALLOWED_DIR`が正しく設定されているか確認
2. 指定したパスが`ALLOWED_DIR`内にあるか確認

### ディレクトリ権限エラー

```
ALLOWED_DIR is not writable
```

**解決方法:**

1. `ALLOWED_DIR`の権限を確認: `ls -la /data`
2. 必要に応じて権限を変更: `chmod 755 /data`

## ディレクトリ構造

```
file-transfer-system/
├── cmd/
│   └── server/
│       └── main.go              # エントリーポイント
├── internal/
│   ├── config/
│   │   └── config.go            # 設定管理
│   ├── grpc/
│   │   ├── server.go            # gRPCサーバー
│   │   └── client.go            # gRPCクライアント
│   ├── http/
│   │   └── handler.go           # HTTPハンドラー
│   ├── transfer/
│   │   ├── sender.go            # ファイル送信
│   │   ├── receiver.go          # ファイル受信
│   │   └── validator.go         # パスバリデーション
│   └── progress/
│       └── tracker.go           # 進捗追跡
├── api/
│   └── proto/
│       ├── transfer.proto       # Protocol Buffers定義
│       ├── transfer.pb.go       # 生成されたGoコード
│       └── transfer_grpc.pb.go  # 生成されたgRPCコード
├── go.mod
├── go.sum
└── README.md
```

## ライセンス

MIT License
