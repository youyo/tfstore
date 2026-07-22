# 0001. Terraform ステートロックを S3 ネイティブロックに一本化し DynamoDB を廃止する

- 日付: 2026-07-23
- ステータス: 採用

## コンテキスト

旧 tfstore は CloudFormation テンプレートに S3 バケットと DynamoDB テーブル（PAY_PER_REQUEST）を
両方作成し、`dynamodb_table` を指定した `terraform init -backend-config` 例を提示していた。
Terraform 1.10 で S3 の条件付き書き込み（`If-None-Match`）を用いたネイティブロック機構
（`use_lockfile = true`）が実験的導入され、1.11 で GA に昇格した。これに伴い公式ドキュメントは
`dynamodb_table` を deprecated（将来のマイナーリリースで削除予定）と明記している
（`https://developer.hashicorp.com/terraform/language/backend/s3`）。
tfstore は 0 からの再構築であり後方互換の対象ユーザーを持たないため、この機会に
ロック方式を刷新するかどうかの判断が必要だった。

## 決定

CloudFormation テンプレートから `AWS::DynamoDB::Table` を完全に削除し、S3 バケット単体
（バージョニング有効・SSE-S3・PublicAccessBlock 4項目 true・DeletionPolicy/UpdateReplacePolicy:
Retain）のみを作成する構成に変更する。CLI が出力する `terraform init -backend-config` 例も
`encrypt=true` と `use_lockfile=true` のみを含み、`dynamodb_table` は含めない
（`internal/backend.BackendConfigExample`、golden テストで固定）。README には
Terraform >= 1.11 の要件と、`<key>` および `<key>.tflock` オブジェクトへの IAM 権限例を明記する。

## 検討した代替案

- **DynamoDB テーブルを維持し、`use_lockfile` と併用可能にする（ハイブリッド対応）**:
  既存ユーザーの移行パスを考慮する場合には有効だが、tfstore は新規ステート作成専用の
  create-only CLI であり、既存スタックの更新・移行機能自体を持たない設計方針
  （critique-decisions.md H6）と矛盾するため不採用。
- **`dynamodb_table` を後方互換のため任意フラグとして残す**: 新規構築であり移行対象ユーザーが
  存在しないため、複雑さを増すだけと判断し不採用。

## 影響

- CloudFormation スタックが作成する AWS リソースが1種類（S3 バケットのみ）になり、DynamoDB の
  運用・課金・IAM 権限管理が不要になる。
- 生成されるステートバックエンドは Terraform >= 1.11 を要求する（それ未満のバージョンでは
  `use_lockfile` が使えない）。README に明記済み。
- 将来 Terraform が `dynamodb_table` を完全削除しても tfstore 側の対応は不要（最初から使用しない）。
