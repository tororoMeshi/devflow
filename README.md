# devflow

devflow は、AI支援開発で固定すると安定する進行管理を CLI で扱う MVP ツールです。

Flow 定義を読み込み、現在工程、成果物、承認、完了、戻り、スキップ、終了を管理します。
AI API 呼び出しや MCP 連携を行うツールではありません。

## Build

```bash
go build -o /tmp/devflow ./cmd/devflow
```

## Quick Start

```bash
/tmp/devflow init
/tmp/devflow list
/tmp/devflow start post-task-review
/tmp/devflow status
/tmp/devflow prompt
/tmp/devflow done
```

`init` は `.devflow/` と標準の Flow 定義を作成します。
`start` は Flow を開始し、最初の工程を current step として `.devflow/state.json` に保存します。
`prompt` は現在工程で行う作業指示を表示します。
`done` は現在工程を完了し、次の工程へ進めます。

## Artifact が必要な工程

標準 Flow の `write_review` 工程では `docs/code-review.md` が必要です。

このファイルがない状態で `done` すると失敗します。
作成してから `done` を実行します。

```bash
mkdir -p docs
printf 'review ok\n' > docs/code-review.md
/tmp/devflow done
```

## Approval が必要な工程

標準 Flow の `human_approval` 工程では承認が必要です。

`done` の前に `approve` を実行します。

```bash
/tmp/devflow approve --note "確認済み"
/tmp/devflow done
```

## Back / Skip / Finish

前の工程へ戻る場合:

```bash
/tmp/devflow back --reason "確認に戻る"
```

任意の上流工程へ戻る場合:

```bash
/tmp/devflow back --to check_changes --reason "要件確認からやり直す"
```

`back` は戻り先以降の完了、スキップ、承認状態を無効化します。成果物ファイルは削除しません。

現在工程をスキップする場合:

```bash
/tmp/devflow skip --reason "今回は不要と判断"
```

Flow を理由付きで終了する場合:

```bash
/tmp/devflow finish --reason "対象外の作業だったため"
```

## Files

`.devflow/flows/*.cue` は Flow 定義です。

`.devflow/state.json` は現在の Flow と工程を保存するローカル状態です。
`.devflow/state.json` は Git 管理対象外です。

`devflow init` では `state.json` を作りません。
`devflow start <flow>` で `state.json` を作ります。

## MVP Scope

対応済み:

* `init`
* `list`
* `start`
* `status`
* `prompt`
* `approve`
* `done`
* `back`
* `skip`
* `finish`

MVP で扱わないもの:

* AI API 呼び出し
* MCP 連携
* 禁止コマンド制御
* 複雑な条件式エンジン
* `--json`
* 高度な CLI ライブラリ
* 対話式 UI
* 色付き出力
