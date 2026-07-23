# 0003. デフォルトバケット名の account-id/region 解決はCloudFormation疑似パラメータで行う（STS依存を追加しない）

- 日付: 2026-07-23
- ステータス: 採用

## コンテキスト

`tfstore <name>` のデフォルトバケット名は `tfstate-{name}-{account-id}-{region}` である。
`account-id` を得る一般的な手段は AWS STS の `GetCallerIdentity` だが、tfstore の依存方針
（CLAUDE.md「新規の直接依存を追加しない」、intent.md「依存追加は停止条件」）により、AWS SDK
に STS クライアントを追加することは避けたい。一方でスタック名 `tfstore-{name}` は
`CreateStack` API 呼び出しの引数であり CloudFormation テンプレートの外側にあるため、Go側で
文字列結合するだけで済む。

## 決定

`account-id`/`region` を要するバケット名部分だけ CloudFormation 側の疑似パラメータ
（`${AWS::AccountId}` / `${AWS::Region}`）で解決する。テンプレートに `Name`（Default なし、
常に指定必須）と `BucketName`（Default `''`）の2つの String Parameter を追加し、
`BucketNameProvided` Condition（`!Not [!Equals [!Ref BucketName, '']]`）で分岐する
`Fn::If` を `Bucket.Properties.BucketName` に設定する:
`Fn::If[BucketNameProvided, !Ref BucketName, !Sub 'tfstate-${Name}-${AWS::AccountId}-${AWS::Region}']`。
スタック名側は `tfstore-{name}` のまま cmd 側で単純な文字列結合として組み立てる。

## 検討した代替案

- **AWS STS (`GetCallerIdentity`) を追加してGo側でaccount-idを取得する**: バケット名の組み立てを
  Go側に統一できるが、新規の直接依存追加（少なくとも `service/sts` クライアントの利用）となり、
  intent.md が明示する「依存追加は停止条件」に抵触する。CloudFormation 疑似パラメータで同じ値を
  取得できる以上、追加のAWS呼び出し・依存を持ち込む理由がないため不採用。
- **`BucketName` パラメータの `Default` に `Fn::Sub` を書く**: CloudFormationの `Parameters.Default`
  は静的リテラルのみを許容し `Fn::Sub` などの組み込み関数を書けないため、この案自体が技術的に
  実現不可能。デフォルト解決を `Resources` 側の `Fn::If` に持たせる必要があった。
- **`Name` パラメータにも `Default` を設定する**: `Name` はスタック名 `tfstore-{name}` の元にもなる
  値であり、常にGo側（cmd）から明示的に渡される想定である。テンプレート側にデフォルトを持たせると
  「Goが必ず渡す」という呼び出し契約が暗黙になり、CreateStackを直接叩く別クライアントが `Name` を
  省略した場合に意図しないバケット名（`tfstate--{account-id}-{region}` のような欠損値）が生成され得る。
  そのため `Name` に `Default` を設定せず必須パラメータとして扱う。

## 影響

- `internal/cfn/cloudformation.yaml` に `Name`/`BucketName` の2パラメータと `BucketNameProvided`
  Condition が追加される。`internal/cfn.Template` のシンボル名・型・埋め込み方式（`//go:embed`）は
  変更していない。
- `internal/backend.CreateInput{StackName, Name, BucketName}` を介して、Goは `Name`/`BucketName`
  パラメータ値をそのまま CloudFormation の `Parameters` に渡すだけで、account-id/region の解決には
  一切関与しない。将来 account-id を Go側で必要とする別要件が出た場合は、本ADRの前提
  （STS非依存）を再検討する必要がある。
- go.mod/go.sum への変更は発生しない（STS依存を追加していないことは `go mod tidy` の無diffで
  確認済み）。
