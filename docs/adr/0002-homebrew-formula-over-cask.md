# 0002. Homebrew 配布は Cask ではなく Formula（GoReleaser `brews`）を使う

- 日付: 2026-07-23
- ステータス: 採用

## コンテキスト

Phase 1 の外部調査（collect-research.md §4）では、GoReleaser の `brews`（Homebrew Formula 生成）が
v2.10 で soft-deprecated、v2.16（2026-05）で完全 deprecated になっており、新規構築の tfstore では
非推奨警告の出ない `homebrew_casks` への移行が妥当と結論づけていた。この調査結果に基づき
Phase 2 のプランは当初 `homebrew_casks`（`directory: Casks`）を採用していた。
その後ユーザーから追加指示（intent.md 追加要件2, 2026-07-23 01:38）があり、
`brew install youyo/tap/tfstore` という Formula 特有の UX を優先したいこと、また Cask は
署名済みバイナリを前提とする設計であり、tfstore は無署名バイナリを配布するため Gatekeeper の
quarantine 問題が Cask 経路でも実利的に解消されないことが指摘された。

## 決定

`.goreleaser.yml` は `homebrew_casks` ではなく `brews`（Formula、`directory: Formula`）を使う。
GoReleaser CLI (`goreleaser check`) が出す `brews is being phased out in favor of
homebrew_casks` という非推奨警告は許容し、抑制や回避は行わない。README のインストール手順は
`brew install youyo/tap/tfstore` を案内し、無署名バイナリに起因する Gatekeeper 隔離の回避手順
（`xattr -dr com.apple.quarantine`）を手動ダウンロード時の注記として維持する。

## 検討した代替案

- **`homebrew_casks` を採用する（Phase 1 調査の推奨案）**: GoReleaser の非推奨警告を避けられるが、
  Cask は署名済みバイナリを前提とする設計であり、無署名の tfstore バイナリでは
  Gatekeeper の "damaged and cannot be opened" 警告など Formula 経路より扱いにくい体験になり得る。
  加えてユーザーが明示的に `brew install <tap>/tfstore` という Formula 特有の UX を希望したため不採用。
- **Formula と Cask の両方を用意する**: 配布導線が二重になり `.goreleaser.yml` と homebrew-tap
  リポジトリ双方の保守コストが増える一方、tfstore の配布規模（個人向け軽量 CLI）に見合わないため
  不採用。

## 影響

- `goreleaser check` は `brews` 非推奨警告を毎回出す。これは既知の許容事項であり CI を失敗させない
  （warning であり check の exit code には影響しない）。
- 将来 GoReleaser が `brews` を完全に削除した場合、本 ADR の前提が崩れるため
  `homebrew_casks`（または後継の配布方式）への再移行を検討する必要がある。その際は署名済み
  バイナリ化（Apple 公証）も合わせて再検討すること。
- ユーザー向けインストール手順は `brew install youyo/tap/tfstore` に統一され、Cask 由来の
  `brew install --cask` 系コマンドは案内しない。
