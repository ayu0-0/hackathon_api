# ベースイメージとして公式のGoイメージを使用
FROM golang:1.22

# ワーキングディレクトリを設定
WORKDIR /app

# モジュールファイルと依存関係をコピー
COPY ../api-db/go.mod .

# 依存関係を取得
RUN go mod download

# アプリケーションのソースコードをコピー
COPY . .

# アプリケーションをビルド
RUN go build -o main .

# コンテナの実行ポートを設定
EXPOSE 8080

# アプリケーションを実行
CMD ["./main"]