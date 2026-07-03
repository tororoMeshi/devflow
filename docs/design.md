# devflow MVP 設計書

## 概要

この文書は、`devflow MVP 要件定義書` をもとに、MVP実装に必要な設計を整理するものです。

devflow は、人間が毎回チャットや設定ファイルで伝えていた進行指示を、機械的に実行できる Flow として扱うためのCLIツールです。

MVPでは、次の責務に集中します。

* Flow定義を読み込む
* 利用可能なFlowを一覧表示する
* 現在のFlowと工程を状態ファイルに保存する
* AIに現在の工程の指示を表示する
* 成果物の存在を確認する
* 承認を記録する
* 工程を完了する
* 前の工程へ戻る
* 工程をスキップする
* Flowを終了する

MVPでは、自動コマンド実行、禁止コマンド制御、AI API呼び出し、MCPオーケストレーション、複雑な条件式エンジンは扱いません。

## 設計方針

### 固定部分をdevflowが扱う

devflow は、作業の現在地、工程順序、成果物、承認、終了理由など、固定すると安定する進行管理を扱います。

AIは、各工程の中で調査、設計、実装、レビュー、分析、提案を行います。

### Flow定義は共有し、状態はローカルに置く

Flow定義はプロジェクトの進行ルールとして扱うため、Git管理対象にします。

一方で、`state.json` は作業者、ブランチ、AIセッション、現在の依頼に依存するため、Git管理対象外にします。

### MVPでは同時に1つのFlowだけを実行する

MVPでは、同時に実行できるFlowは1つだけとします。

状態管理を単純に保つため、複数Flowの同時実行やFlowの合成は扱いません。

### gateは成果物と承認に限定する

MVPの通過条件（gate）は、次の2つだけです。

* 必須成果物が存在すること
* 承認が必要な工程で承認が記録されていること

条件式、コマンド結果、成果物の中身、外部サービスの状態は判定しません。

### 状態遷移はstate更新に限定する

MVPでは、`state.json` を更新するコマンド処理を状態遷移として設計します。

対象は主に次のコマンドです。

* `devflow start <flow>`
* `devflow done`
* `devflow approve`
* `devflow back`
* `devflow skip`
* `devflow finish`

各コマンドは、現在のstate、Flow定義、コマンド引数を入力として検証し、新しいstate、表示メッセージ、終了コードを返す処理として扱います。

状態遷移の中核は、できるだけ副作用を持たない関数に寄せます。

具体的には、Flow定義、現在のstate、コマンド引数、gate判定結果を入力として受け取り、新しいstateと診断結果を返す構造にします。

ファイル読み書き、artifact存在確認、標準出力、終了コードの返却は、外側のコマンド層で扱います。

一方で、Flow定義そのものは状態遷移言語にはしません。

MVPのFlow定義は、`steps` 配列の順序を使う単純な定義に留めます。
`from`、`to`、`condition`、`on_error` のような遷移定義は扱いません。

## ディレクトリ構成

### `devflow init` 後

`devflow init` 実行後は、次の構成になります。

```text
.devflow/
  flows/
    post-task-review.cue
  .gitignore
```

`.devflow/.gitignore` には次の内容を入れます。

```gitignore
state.json
```

### `devflow start <flow>` 後

`devflow start <flow>` 実行後は、ローカル状態として `state.json` が作成されます。

```text
.devflow/
  flows/
    post-task-review.cue
  .gitignore
  state.json
```

## ファイルの役割

### `.devflow/flows/`

Flow定義を置くディレクトリです。

このディレクトリは原則としてGit管理対象です。

MVPでは、`devflow init` によって次のFlowを作成します。

```text
.devflow/flows/post-task-review.cue
```

### `.devflow/state.json`

現在進行中または直近のFlow状態を保存するファイルです。

このファイルはローカル状態として扱い、Git管理対象外にします。

`state.json` は `devflow init` では作成しません。
`devflow start <flow>` 実行時に作成します。

## Flow定義形式

MVPでは、Flow定義をCUEで記述します。

### 基本構造

```cue
flow: {
	id: string
	title: string
	description?: string
	steps: [...#Step]
}

#Step: {
	id: string
	title: string
	instruction: string
	artifacts?: [...#Artifact]
	approval?: #Approval
}

#Artifact: {
	path: string
	required?: bool | *true
}

#Approval: {
	required?: bool | *false
}
```

### Flow

Flowは、開発作業の流れ全体を表します。

必須項目は次の通りです。

* `id`
* `title`
* `steps`

任意項目は次の通りです。

* `description`

例:

```cue
flow: {
	id: "post-task-review"
	title: "タスク後レビュー"
	description: "実装や修正の完了後に、変更内容、テスト、レビュー、人間承認を確認するFlowです。"

	steps: [
		{
			id: "check_changes"
			title: "変更ファイル確認"
			instruction: "git status と diff を確認し、変更されたファイルを整理してください。"
		},
	]
}
```

### 工程（step）

工程は、Flowの中の1つの区切りです。

必須項目は次の通りです。

* `id`
* `title`
* `instruction`

任意項目は次の通りです。

* `artifacts`
* `approval`

工程順序は、`steps` 配列の並び順で扱います。

MVPでは、Flow定義に独立した `gate` フィールドは持ちません。
gateは、`step.artifacts` と `step.approval` から devflow が導出します。

### 成果物（artifact）

成果物は、工程を完了するために存在確認するファイルです。

MVPでは、成果物はプロジェクトルートからの相対ファイルパスとして扱います。

例:

```cue
artifacts: [
	{
		path: "docs/code-review.md"
		required: true
	},
]
```

`required` を省略した場合は `true` として扱います。
Flow読み込みまたは正規化の段階で、省略された `required` は `true` にそろえます。
gate判定は、正規化済みの `Artifact` を受け取る前提にします。

MVPでは、`required: false` は読み込めますが、完了判定には使いません。
任意成果物は表示上の補足として扱います。

### 承認（approval）

承認は、人間の承認が必要な工程を表します。

例:

```cue
approval: {
	required: true
}
```

`approval.required` が `true` の工程では、対象工程に承認が記録されるまで `devflow done` で次へ進めません。

MVPでは、承認IDは独立して持たず、工程IDに対して承認を記録します。

## Flow定義の検証

Flow定義は、主に次のタイミングで検証します。

* `devflow list`
* `devflow start <flow>`

MVPでは、単独の `devflow validate` コマンドは持ちません。

### 検証項目

Flow定義では、最低限次を確認します。

* `flow.id` が存在する
* `flow.title` が存在する
* `flow.steps` が1件以上存在する
* 各工程に `id` が存在する
* 各工程に `title` が存在する
* 各工程に `instruction` が存在する
* 工程IDがFlow内で重複していない
* artifact path が有効である

必須文字列は、空文字または空白のみの場合も不正とします。

対象は次の通りです。

* `flow.id`
* `flow.title`
* `step.id`
* `step.title`
* `step.instruction`
* `artifact.path`

`description` は任意説明なので、空でも致命的なエラーにはしません。

`flow.id` と `step.id` は、機械向けの識別子として扱います。
MVPでは、英数字、ハイフン、アンダースコアのみ許可します。

```text
OK:
- post-task-review
- check_changes
- step1

NG:
- check changes
- レビュー工程
- step/1
```

`title` は人間向けの表示名なので、日本語を許可します。

MVPでは、Flowファイル名と `flow.id` を一致させます。

例:

```text
.devflow/flows/post-task-review.cue
flow.id: "post-task-review"
```

Flowファイル名と `flow.id` が一致しないFlowは、Flow定義として不正とします。

`devflow list` では、そのFlowを `invalid` として表示します。

`devflow start <flow>` では、`.devflow/flows/<flow>.cue` を読み、読み込んだ `flow.id` が `<flow>` と一致することを確認します。

Flow読み込みまたは正規化のテストでは、artifactの `required` について次を確認します。

* `required` 省略時は `true` になる
* `required: false` は `false` のまま残る

approvalの `required` についても、次を確認します。

* `approval.required` 省略時は `false` として扱う
* `approval` がない場合はapproval不要として扱う

内部モデルでは、`approval` がない場合は `Step.Approval = nil` として保持してよいものとします。
gateやpromptでは、nilをapproval不要として扱います。

`step.artifacts` がない場合は、空sliceとして扱います。

### artifact path の検証

artifact path は、次の条件を満たす必要があります。

* プロジェクトルートからの相対パスである
* 絶対パスではない
* `..` を含まない
* URLではない
* globではない
* ディレクトリではなくファイルパスとして扱える
* 空文字または空白のみではない
* 末尾スラッシュのディレクトリ風pathではない

artifact path は、内部的に `/` 区切りのプロジェクト相対パスとして扱います。
MVPでは、Windows形式の絶対パスや親ディレクトリ参照も不正とします。

例:

```text
Invalid:
- C:\tmp\result.md
- ..\result.md
```

MVPでは、ファイルが空でも存在すれば成果物として扱います。

## Flow読み込み・検証・正規化テスト

Flow読み込み・検証・正規化テストでは、`.devflow/flows/*.cue` を読み込み、devflow内部で扱う `Flow` 構造体へ変換できることを確認します。

Flow層は、ファイルに書かれたFlow定義を、command、transition、gate が使える内部モデルに変換する層です。

### テスト対象

Flowまわりのテストは、次の3つに分けます。

* 読み込みテスト
* 検証テスト
* 正規化テスト

実装上も、次の責務に分けます。

```text
loader:
  CUEファイルを読む
  CUE評価を行う
  Flow構造体へ変換する

validate:
  Flow構造体の妥当性を確認する

normalize:
  required省略値などを内部モデル向けに整える
```

Flow層のテストでは、次を扱います。

* `.cue` ファイルの読み込み
* `flow.id`
* `flow.title`
* `flow.description`
* `flow.steps`
* `step.id`
* `step.title`
* `step.instruction`
* `step.artifacts`
* `artifact.path`
* `artifact.required`
* `step.approval.required`
* step ID重複
* artifact path の静的検証

Flow層のテストでは、次を扱いません。

* `state.json` の読み書き
* active Flow 判定
* `current_step_id` の状態遷移
* artifactファイルが実際に存在するか
* approvalが記録済みか
* `done`、`back`、`skip`、`finish` の状態更新
* 標準出力
* `os.Exit`

Flow層が見るartifactは、pathの静的な妥当性だけです。
実ファイルの存在確認はgateの責務です。

### 読み込みテスト

読み込みテストでは、最低限次を確認します。

* `id`、`title`、`steps` を持つ最小Flowを読み込める
* `description` があるFlowを読み込める
* `description` がないFlowも読み込める
* artifactがあるstepを読み込める
* approvalがあるstepを読み込める
* 複数stepを定義順に読み込める

工程順序は `steps` 配列の並び順で扱うため、読み込んだ `Steps` の順序が維持されることを確認します。

### 検証テスト

検証テストでは、最低限次をinvalidとして扱うことを確認します。

* `flow.id` がない
* `flow.title` がない
* `flow.steps` がない、または空配列
* `step.id` がない
* `step.title` がない
* `step.instruction` がない
* step ID が重複している
* `artifact.path` がない
* `artifact.path` が不正

必須文字列が空文字または空白のみの場合もinvalidとします。

検証エラーは、文字列完全一致ではなくエラーコードで確認します。

MVPでは、最低限次のようなエラーコードを扱います。

```text
error_missing_flow_id
error_missing_flow_title
error_flow_has_no_steps
error_missing_step_id
error_missing_step_title
error_missing_step_instruction
error_duplicate_step_id
error_missing_artifact_path
error_invalid_artifact_path
error_flow_id_filename_mismatch
```

### 正規化テスト

正規化テストでは、最低限次を確認します。

* `artifact.required` 省略時は `true` になる
* `artifact.required = false` は `false` のまま残る
* `approval.required` 省略時は `false` として扱う
* `approval` がない場合はapproval不要として扱う
* `artifacts` がない場合は空sliceとして扱う

### pathcheck のテスト

artifact path の検証を `pathcheck` パッケージに分ける場合は、個別テストを用意します。

Valid:

```text
docs/code-review.md
docs/review/result.md
README.md
```

Invalid:

```text
/tmp/result.md
../result.md
docs/../secret.md
https://example.com/result.md
http://example.com/result.md
docs/*.md
docs/
""
"   "
C:\tmp\result.md
..\result.md
```

### `devflow list` との関係

`devflow list` は、壊れたFlowも隠さず `invalid` として表示します。

そのため、複数Flowの読み込みでは、1つのFlowが壊れていても処理全体を止めず、他のFlowも読み続けます。

概念的には、次のような結果を返します。

```go
type FlowFileResult struct {
	Flow *Flow
	FilePath string
	Status string
	Err error
}
```

複数Flow読み込みのテストでは、validなFlowとinvalidなFlowが混在している場合に、validなFlowは `Flow` として返り、invalidなFlowはErr付き結果として返ることを確認します。

### `devflow start <flow>` との関係

`devflow start <flow>` は、指定されたFlow IDのFlowを開始します。

MVPでは、Flowファイル名と `flow.id` を一致させます。

`devflow start <flow>` は `.devflow/flows/<flow>.cue` を読みます。
読み込んだ `flow.id` が `<flow>` と一致しない場合はエラーにします。

### テストケースの書き方

Flow読み込みは、`t.TempDir()` に `.cue` ファイルを作って確認します。

```go
func TestLoadFlow(t *testing.T) {
	tests := []struct {
		name string
		cue string
		wantFlow flow.Flow
		wantErr bool
	}{
		{
			name: "loads minimal flow",
		},
		{
			name: "defaults artifact required to true",
		},
		{
			name: "keeps artifact required false",
		},
		{
			name: "returns error when step id is duplicated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".devflow", "flows", "test.cue")
			writeFile(t, path, tt.cue)

			got, err := flow.LoadFile(path)
			if tt.wantErr {
				assertError(t, err)
				return
			}

			assertNoError(t, err)
			assertFlowEqual(t, got, tt.wantFlow)
		})
	}
}
```

検証だけなら、CUEファイルを使わず `Flow` 構造体を直接渡すテストでもよいです。

テストヘルパーは、次のようなものを用意します。

* `writeFlowFile`
* `loadFlowFromString`
* `assertFlowEqual`
* `assertValidationErrorCode`
* `assertStepIDs`
* `assertArtifact`
* `assertApprovalRequired`

## 状態ファイル形式

MVPでは、`state.json` を次の構造で保存します。

```json
{
  "flow_id": "post-task-review",
  "status": "running",
  "current_step_id": "check_changes",
  "completed_steps": [],
  "skipped_steps": {},
  "approvals": {},
  "back_history": [],
  "finish": null
}
```

## 状態フィールド

### `flow_id`

現在または直近で実行したFlow IDです。

### `status`

Flowの状態です。

`state.json` に保存する値として、MVPでは次の値を扱います。

```text
running
completed
finished
```

`no_state` と `invalid_state` は `state.json` に保存するstatusではなく、読み込み時に判定する状態として扱います。

#### `running`

Flowが実行中であることを表します。

この状態では、active Flow が存在します。

#### `completed`

最後の工程で `devflow done` が成功し、Flowが自然完了したことを表します。

この状態では、active Flow は存在しません。

#### `finished`

`devflow finish --reason <reason>` によって、Flowが理由付きで終了されたことを表します。

この状態では、active Flow は存在しません。

#### `no_state`

`state.json` が存在しない状態です。

まだFlowを開始していない状態として扱います。

`devflow start <flow>` によって `running` に遷移できます。

#### `invalid_state`

`state.json` 自体が壊れている、またはstateとFlow定義の整合性確認に失敗した状態です。

MVPでは、`invalid_state` を概念上は次の2つに分けて扱います。

* `state.json` 単体の不正
* stateとFlow定義の不整合

`Store.Load()` が返す `LoadInvalid` は、JSON破損、未知のstatus、必須フィールド不足、型不正など、`state.json` 単体の不正だけを表します。

`state.flow_id` が存在しないFlowを参照している場合や、`current_step_id` がFlow内に存在しない場合は、command層またはactive Flow読み込み時の整合性確認で検出します。

MVPでは自動修復しません。
人間とAIが原因を判断できるエラーを表示し、非0で終了します。

### `current_step_id`

現在の工程IDです。

MVPでは、`current_step_id` は `running`、`completed`、`finished` のすべてで必須です。

`completed` の場合は、最後に `done` した工程IDを保持します。

`finished` の場合は、`finish` 実行時にいた工程IDを保持します。

### `completed_steps`

完了済み工程IDの一覧です。

`devflow done` が成功した工程を追加します。

### `skipped_steps`

スキップ済み工程の記録です。

キーは工程IDです。

```json
{
  "skipped_steps": {
    "check_docs": {
      "reason": "今回はREADME更新が不要なため"
    }
  }
}
```

### `approvals`

承認記録です。

キーは工程IDです。

```json
{
  "approvals": {
    "human_approval": {
      "approved": true,
      "note": "確認済み。次へ進めてよい。"
    }
  }
}
```

### `back_history`

戻り操作の履歴です。

```json
{
  "back_history": [
    {
      "from_step_id": "human_approval",
      "to_step_id": "write_review",
      "reason": "レビュー結果に追記が必要になったため"
    }
  ]
}
```

MVPでは、履歴の時刻は必須にしません。

### `finish`

Flow終了情報です。

`status` が `finished` の場合に使います。

```json
{
  "finish": {
    "reason": "対象外の変更だったため"
  }
}
```

`status` が `running` または `completed` の場合は `null` とします。

## active Flow の定義

MVPでは、`state.json` が存在し、かつ `status` が `running` の場合に active Flow が存在すると判断します。

次の場合、active Flow は存在しません。

* `state.json` が存在しない
* `state.json` の `status` が `completed`
* `state.json` の `status` が `finished`

active Flow が存在しない場合でも、次のコマンドは実行できます。

* `devflow init`
* `devflow list`
* `devflow start <flow>`

active Flow を必要とする主なコマンドは次の通りです。

* `devflow status`
* `devflow prompt`
* `devflow done`
* `devflow approve`
* `devflow back`
* `devflow skip`
* `devflow finish`

## 状態遷移

MVPでは、Flow全体の状態を大きく次のように整理します。

```text
no_state
running
completed
finished
invalid_state
```

このうち `state.json` に保存する `status` は、`running`、`completed`、`finished` の3つです。
`no_state` は `state.json` が存在しない状態、`invalid_state` は読み込みまたは整合性確認に失敗した状態として判定します。

### コマンドごとの状態遷移

| 現在状態 | コマンド | 条件 | 次状態 | 備考 |
| --- | --- | --- | --- | --- |
| no_state | `start` | Flow有効 | running | `state.json` 作成 |
| running | `done` | gate OK / 次stepあり | running | `current_step_id` 更新 |
| running | `done` | gate OK / 次stepなし | completed | 自然完了 |
| running | `approve` | 対象工程が存在 | running | 承認を記録 |
| running | `skip` | reasonあり / 次stepあり | running | skippedを記録。必要に応じてwarningを返す |
| running | `skip` | reasonあり / 次stepなし | completed | 最終工程skip。warningを返す |
| running | `back` | reasonあり / 前stepあり | running | `current_step_id` を前へ戻す |
| running | `finish` | reasonあり | finished | 理由付き終了 |
| completed | `start` | Flow有効 | running | 新しいFlow開始 |
| finished | `start` | Flow有効 | running | 新しいFlow開始 |
| invalid_state | active Flowを読むコマンド | - | invalid_state | 非0で終了 |

`devflow init` と `devflow list` は、active Flow の有無に関係なく実行できます。

### `devflow start <flow>`

指定されたFlowを開始します。

#### 前提

* `.devflow/flows/<flow>.cue` が存在する
* Flow定義が有効である
* 読み込んだ `flow.id` が `<flow>` と一致する
* active Flow が存在しない

#### 処理

* `.devflow/flows/<flow>.cue` を読み込む
* 最初の工程を `current_step_id` にする
* `status` を `running` にする
* `state.json` を作成または上書きする

#### 成功後の状態

```json
{
  "flow_id": "post-task-review",
  "status": "running",
  "current_step_id": "check_changes",
  "completed_steps": [],
  "skipped_steps": {},
  "approvals": {},
  "back_history": [],
  "finish": null
}
```

### `devflow done`

現在の工程を完了扱いにします。

#### 前提

* active Flow が存在する
* 現在の工程がFlow定義内に存在する
* 必須成果物が存在する
* 必須承認が記録されている

#### 処理

* 現在の工程IDを `completed_steps` に追加する
* 現在の工程が `skipped_steps` に存在する場合は削除する
* 次の工程が存在する場合、`current_step_id` を次の工程にする
* 次の工程が存在しない場合、`status` を `completed` にする

#### 成功後の状態

次の工程が存在する場合:

```json
{
  "status": "running",
  "current_step_id": "write_review"
}
```

最後の工程だった場合:

```json
{
  "status": "completed",
  "current_step_id": "human_approval"
}
```

### `devflow approve`

承認が必要な工程に対して承認を記録します。

#### 前提

* active Flow が存在する
* 対象工程がFlow定義内に存在する
* 対象工程に `approval.required: true` が設定されている

#### 対象工程

`--step <step>` が指定された場合は、その工程を対象にします。

`--step` が省略された場合は、現在の工程を対象にします。

#### 処理

* 対象工程IDに対して `approved: true` を記録する
* `--note <note>` が指定されている場合は、noteを記録する

承認不要工程への `approve` はエラーにします。
`approve` は、承認が必要な工程に対する操作として扱います。

#### 保存例

```json
{
  "approvals": {
    "human_approval": {
      "approved": true,
      "note": "レビュー結果を確認済み。"
    }
  }
}
```

### `devflow back --reason <reason>`

前の工程へ戻ります。

MVPの `back` は、AIまたは人間が「前工程へ戻った方が安全」と判断した場合に使います。

`back` は先へ進める操作ではなく、確認を増やすための操作です。
そのため、AIが問題を見つけた場合は、reason付きで実行してよい操作として扱います。

ただし、人間承認済みの判断を大きく覆す可能性がある場合は、AIが勝手に進めず、人間に確認する運用とします。

AIやユーザーは `completed_steps` を直接編集しません。
AIやユーザーは、前工程へ戻る意図と理由を `devflow back --reason <reason>` で表し、devflow が内部的に `current_step_id`、`completed_steps`、`back_history` を更新します。

#### 前提

* active Flow が存在する
* `--reason <reason>` が指定されている
* 戻れる前工程が存在する

#### 戻り先

MVPでは、直前の工程へ戻ります。

戻り先は、Flow定義の `steps` 配列における現在工程の1つ前の工程です。

#### 処理

* `back_history` に戻り操作を記録する
* `current_step_id` を前の工程IDにする
* 戻り先の工程IDが `completed_steps` に存在する場合は削除する
* artifact ファイルは削除しない
* approvals は削除しない
* skipped_steps は削除しない

#### 理由

artifactやapprovalは、人間やAIの作業結果として残す価値があります。
戻り操作は状態だけを戻し、作成済みファイルや承認記録は削除しません。

戻り先の工程IDを `completed_steps` から削除するのは、その工程を再作業対象として扱うためです。

MVPの `back` は直前の工程へ戻る操作に限定するため、戻り先より後の `completed_steps` を一括整理する処理は行いません。

### `devflow skip --reason <reason>`

現在の工程をスキップします。

`skip` は、通常のgateを満たして進む `done` とは異なる例外的な操作です。
確認を減らして先へ進む操作であるため、AIが自律的に不要判断して実行する操作とはしません。

AIは `skip` が必要だと判断した場合、原則として人間に確認します。
人間が明示した場合に、reason付きで `devflow skip` を実行します。

#### 前提

* active Flow が存在する
* `--reason <reason>` が指定されている
* `--reason <reason>` が空文字または空白のみではない

#### 処理

* 現在の工程IDを `skipped_steps` に記録する
* 次の工程が存在する場合、`current_step_id` を次の工程にする
* 次の工程が存在しない場合、`status` を `completed` にする

#### 補足

skip は、人間が明示した場合だけ実行する運用とします。

ただし、devflow本体では人間判定を強制しません。

MVPでは、承認が必要な工程や必須成果物がある工程もskip可能です。
その代わり、reasonを必須とし、状態ファイルに記録します。

skipした工程は `completed_steps` には入れず、`skipped_steps` に記録します。
これにより、完了した工程と飛ばした工程を `status` で区別できるようにします。

承認が必要な工程をskipした場合は、warningを診断情報として返します。

```text
Warning: this step requires approval, but it was skipped.
Reason:
- 今回は人間確認により承認工程を省略するため
```

必須成果物がある工程をskipした場合も、warningを診断情報として返します。

```text
Warning: this step has required artifacts, but it was skipped.
Required artifacts:
- docs/code-review.md
```

最終工程をskipした場合、Flowは `completed` になります。
その場合も、Flowが完了することをwarningとして返します。

```text
Warning: this is the final step. Skipping it will complete the flow.
```

最終工程が承認必須工程である場合は、承認なしでFlowが完了することをwarningとして返します。

```text
Warning: this is the final approval step. Skipping it will complete the flow without approval.
```

### `devflow finish --reason <reason>`

現在のFlowを理由付きで終了します。

#### 前提

* active Flow が存在する
* `--reason <reason>` が指定されている

#### 処理

* `status` を `finished` にする
* `finish.reason` に理由を記録する
* `current_step_id` は最後にいた工程IDとして残す

### `devflow status`

現在のFlow状態を表示します。

#### active Flow が存在する場合

次の情報を表示します。

* Flow ID
* Flow title
* 現在の工程ID
* 現在の工程title
* 完了済み工程
* スキップ済み工程
* 承認状態

#### active Flow が存在しない場合

MVPでは、`status` は active Flow を必要とするコマンドとして扱います。

active Flow が存在しない場合は、非0の終了コードを返し、active Flow が存在しないことを表示します。

### `devflow prompt`

現在の工程でAIが行う作業指示を表示します。

#### 表示内容

最低限、次を表示します。

```text
Flow: post-task-review
Step: check_changes - 変更ファイル確認

Instruction:
git status と diff を確認し、変更されたファイルを整理してください。

Required artifacts:
- none

Required approval:
- none

After completing:
- devflow done
```

成果物がある場合:

```text
Required artifacts:
- docs/code-review.md
```

`required: true` のartifact、または `required` 省略により `true` に正規化されたartifactは、`Required artifacts` に表示します。

`required: false` のartifactは、`Optional artifacts` に表示してよいものとします。
`Optional artifacts` は表示上の補足であり、`devflow done` のgate判定には使いません。

`Required artifacts` は常に表示し、対象がない場合は `none` と表示します。
`Optional artifacts` は、optional artifact が存在する場合だけ表示します。

```text
Required artifacts:
- docs/code-review.md

Optional artifacts:
- docs/notes.md
```

承認が必要な場合:

```text
Required approval:
- current step
```

## `devflow list` の設計

`devflow list` は、`.devflow/flows/` 配下のFlow定義を読み込み、利用可能なFlowを一覧表示します。

### 有効なFlow

有効なFlowは、次の情報を表示します。

* Flow ID
* Flow title
* Flow description（存在する場合）
* 工程数

例:

```text
Available flows:

- post-task-review
  Title: タスク後レビュー
  Description: 実装や修正の完了後に確認を行うFlowです。
  Steps: 5
  Status: valid
```

### 壊れたFlow

Flow定義が壊れている場合、そのFlowは隠さず `invalid` として表示します。
Flowファイル名と `flow.id` が一致しない場合も `invalid` として扱います。

例:

```text
- broken-flow
  File: .devflow/flows/broken-flow.cue
  Status: invalid
  Error: missing required field: flow.id
```

壊れたFlowが1つでも存在する場合、`devflow list` は非0の終了コードを返します。

有効なFlowは、壊れたFlowが存在しても一覧表示します。

## `post-task-review` サンプルFlow

MVPでは、`devflow init` によって `post-task-review.cue` を作成します。

初期サンプルは、次の工程を持ちます。

```cue
flow: {
	id: "post-task-review"
	title: "タスク後レビュー"
	description: "AIによる実装や修正が完了した後に、変更内容、テスト、レビュー、人間承認を確認するFlowです。"

	steps: [
		{
			id: "check_changes"
			title: "変更ファイル確認"
			instruction: "git status と diff を確認し、変更されたファイルを整理してください。"
		},
		{
			id: "summarize_changes"
			title: "変更内容の要約"
			instruction: "変更内容を確認し、依頼内容に対して何を変更したかを要約してください。"
		},
		{
			id: "check_quality"
			title: "品質確認"
			instruction: "テスト、lint、型チェックなど、今回の変更に必要な確認を行い、結果を整理してください。"
		},
		{
			id: "write_review"
			title: "レビュー結果作成"
			instruction: "変更内容、確認結果、懸念点、必要な修正を docs/code-review.md にまとめてください。"
			artifacts: [
				{
					path: "docs/code-review.md"
					required: true
				},
			]
		},
		{
			id: "human_approval"
			title: "人間承認"
			instruction: "レビュー結果を人間に提示し、次へ進んでよいか確認してください。"
			approval: {
				required: true
			}
		},
	]
}
```

## dogfooding用Flow

`purpose-driven-development` は、`devflow init` では作成しません。

公開サンプルFlowは `post-task-review` とし、`purpose-driven-development` は devflow 自体の開発・検証に使う dogfooding 用Flowとして扱います。

`purpose-driven-development` は、開発者自身の検証環境で次の場所に手動配置します。

```text
.devflow/flows/purpose-driven-development.cue
```

このFlowは公開サンプルではありません。

## エラー方針

各コマンドは、成功時に終了コード `0` を返します。

失敗時は非0の終了コードを返します。

MVPでは、細かい終了コード番号は定義しません。
非0であることをもって失敗と判定します。

### 主なエラー

| 状況                           | 対象コマンド                                                          | 挙動    |
| ---------------------------- | --------------------------------------------------------------- | ----- |
| active Flow が存在しない           | `status`, `prompt`, `done`, `approve`, `back`, `skip`, `finish` | 非0で終了 |
| active Flow が存在する状態で別Flowを開始 | `start`                                                         | 非0で終了 |
| Flow定義が存在しない                 | `start`                                                         | 非0で終了 |
| Flow定義が不正                    | `list`, `start`                                                 | 非0で終了 |
| stateが壊れている                  | active Flowを読むコマンド                                              | 非0で終了 |
| stateが存在しない工程を参照している         | active Flowを読むコマンド                                              | 非0で終了 |
| 必須成果物が不足している                 | `done`                                                          | 非0で終了 |
| 必須承認が不足している                  | `done`                                                          | 非0で終了 |
| 承認不要工程を承認しようとした              | `approve`                                                       | 非0で終了 |
| reason が不足している、または空白のみ      | `back`, `skip`, `finish`                                        | 非0で終了 |

### エラーメッセージ方針

エラーメッセージは、人間とAIが次の行動を判断できる内容にします。

例:

```text
Error: required artifact is missing.
Missing:
- docs/code-review.md

Create the missing artifact, then run:
  devflow done
```

承認不足の例:

```text
Error: approval is required for the current step.
Step:
- human_approval

After human approval, run:
  devflow approve --note "<note>"
  devflow done
```

active Flow がない場合:

```text
Error: no active flow.

Start a flow first:
  devflow list
  devflow start <flow>
```

## state書き込み

MVPでは、`state.json` の書き込みは可能な範囲でatomicに行います。

基本方針は次の通りです。

1. 一時ファイルに新しいstateを書く
2. 書き込みが成功したら `state.json` にリネームする

例:

```text
.devflow/state.json.tmp
.devflow/state.json
```

これにより、途中で処理が失敗した場合に `state.json` が壊れる可能性を下げます。

## 関数構造の方針

MVPでは、Functional Core, Imperative Shell に近い構造を採用します。

状態遷移や判定結果の組み立ては、入力と出力が明確な関数に寄せます。
ファイルシステムや標準出力などの副作用は、CLIコマンド層に閉じ込めます。

### 副作用を持つ層

次の処理は副作用を持つため、コマンド層または専用の読み書き層で扱います。

* CUEファイルを読む
* `state.json` を読む
* artifactファイルの存在を確認する
* `state.json` をatomic writeする
* 標準出力へ表示する
* 終了コードを返す

### 副作用を持たない層

次の処理は、できるだけ副作用を持たない関数として扱います。

* 次工程の計算
* `done`、`approve`、`back`、`skip`、`finish` のstate更新
* `completed_steps`、`skipped_steps`、`approvals` の更新
* エラー種別や診断情報の決定

概念的には、次のような関数として扱います。

```go
ApplyStart(flow Flow, current *State) TransitionResult
ApplyDone(flow Flow, state State, gate GateResult) TransitionResult
ApplyApprove(flow Flow, state State, targetStepID string, note string) TransitionResult
ApplyBack(flow Flow, state State, reason string) TransitionResult
ApplySkip(flow Flow, state State, reason string) TransitionResult
ApplyFinish(state State, reason string) TransitionResult
```

これらの関数は、ファイルを読まず、標準出力へ書かず、`os.Exit` せず、`state.json` も保存しません。

`ApplyStart(flow Flow, current *State)` には、command層で `Store.Load()` の結果を確認した後のstateだけを渡します。
`Store.Load()` が `LoadInvalid` を返した場合、command層で非0終了し、`ApplyStart` は呼び出しません。

`no_state` の場合は `current = nil` として `ApplyStart` に渡します。
`LoadOK` の場合は、読み込んだ既存 `State` を `current` に渡します。

`ApplyStart` は、`current == nil` を `state.json` が存在しない状態として扱います。
`current.Status == running` の場合は、active Flow が存在するためエラーにします。
`current.Status == completed` または `current.Status == finished` の場合は、新しいFlow開始を許可します。

結果は、次のような情報として返します。

```go
type TransitionResult struct {
	State *State
	Diagnostics []Diagnostic
	ExitCode int
}
```

`Diagnostics` には、errorだけでなくwarningも含めます。

warningは失敗ではありません。
たとえば `skip` で承認必須工程や必須成果物がある工程を飛ばした場合、state更新は成功し、`ExitCode` は `0` のままwarningを返します。

```text
ExitCode: 0
Diagnostics:
- Level: warning
  Code: skipped_required_artifact
```

`ExitCode` は、MVPではCLI実装を単純にするため `TransitionResult` に含めます。

将来 `--json` 出力や詳細なエラー分類を追加する段階では、transition層は成功・失敗と診断情報だけを返し、終了コードへの変換をcommand層へ寄せるか再検討します。

表示文言そのものをこの層で確定しすぎず、診断コード、メッセージ、ヒントなどの材料を返す形にします。
これにより、将来 `--json` 出力を追加する場合も拡張しやすくなります。

### stateのコピー

Goでは、sliceやmapを含む構造体を値渡ししても、sliceやmapの中身は共有されます。

そのため、状態遷移関数では入力stateを直接変更せず、最初にコピーしてから新しいstateを作ります。

```go
next := state.Clone()
```

これにより、`ApplyDone` などが呼び出し元のstateを暗黙に変更することを避けます。

`State.Clone()` は、`state` パッケージに定義します。

```text
internal/state/model.go
```

または、必要に応じて次のように分けます。

```text
internal/state/clone.go
```

`ApplyDone`、`ApplyApprove`、`ApplyBack`、`ApplySkip`、`ApplyFinish` は、関数の先頭で入力stateを `Clone()` してから更新します。
呼び出し側に Clone 済みstateを渡すことは要求しません。

`ApplyStart` は既存stateを更新するのではなく新しい `State` を作る操作であるため、必ずしも `Clone()` を使いません。
ただし、既存stateを変更してはなりません。

`Clone()` は shallow copy ではなく、`State` が持つ slice、map、pointer を共有しない deep copy とします。

MVPでは、少なくとも次のフィールドをコピー対象にします。

* `completed_steps`
* `skipped_steps`
* `approvals`
* `back_history`
* `finish`

コピー方針は次の通りです。

* string はそのままコピーする
* `[]string` は新しいsliceを作って中身をコピーする
* `map[string]SkippedStep` は新しいmapを作って各要素をコピーする
* `map[string]Approval` は新しいmapを作って各要素をコピーする
* `[]BackHistory` は新しいsliceを作って中身をコピーする
* `*Finish` は、nilならnil、nilでなければ新しい `Finish` を作って中身をコピーする

MVP時点で `SkippedStep`、`Approval`、`BackHistory`、`Finish` が文字列やboolだけを持つ場合は、各要素は値コピーで十分です。
将来これらの構造体にslice、map、pointerを追加した場合は、その構造体側にも `Clone()` を追加するか検討します。

`Clone()` 後の `State` では、空のslice/mapはnilではなく空のslice/mapとして扱います。
これにより、`state.json` 保存時に次のような安定したJSONを出力できます。

```json
{
  "completed_steps": [],
  "skipped_steps": {},
  "approvals": {},
  "back_history": []
}
```

次のように、空のコレクションが `null` になる出力は避けます。

```json
{
  "completed_steps": null,
  "skipped_steps": null,
  "approvals": null,
  "back_history": null
}
```

### stateコピーのテスト

`State.Clone()` は単体テストを用意します。

最低限、次を確認します。

* Clone後に `CompletedSteps` を変更しても元の `State` が変わらない
* Clone後に `SkippedSteps` を変更しても元の `State` が変わらない
* Clone後に `Approvals` を変更しても元の `State` が変わらない
* Clone後に `BackHistory` を変更しても元の `State` が変わらない
* Clone後に `Finish` を変更しても元の `State` が変わらない
* nilのslice/mapが、Clone後に空のslice/mapとして扱われる

transition関数側でも、次を確認します。

* `ApplyDone` は入力 `State` を変更しない
* `ApplyApprove` は入力 `State` を変更しない
* `ApplyBack` は入力 `State` を変更しない
* `ApplySkip` は入力 `State` を変更しない
* `ApplyFinish` は入力 `State` を変更しない

## transition関数の単体テスト

transition関数の単体テストでは、ファイルI/Oを扱いません。

次の処理は、transition関数の単体テストには含めません。

* `state.json` の読み書き
* CUEファイルの読み込み
* artifactファイルの存在確認
* 標準出力
* `os.Exit`

必要な `Flow`、`State`、`GateResult` は、テスト内で構造体として組み立てます。

主なテスト対象は次の通りです。

* `ApplyStart`
* `ApplyDone`
* `ApplyApprove`
* `ApplyBack`
* `ApplySkip`
* `ApplyFinish`

### 共通方針

`ApplyDone`、`ApplyApprove`、`ApplyBack`、`ApplySkip`、`ApplyFinish` は、入力 `State` を変更しないことを確認します。

テストでは、関数実行前の入力stateを保存し、関数実行後に入力stateが変わっていないことを確認します。
変更されるのは `result.State` だけです。

成功時は、次を確認します。

* `ExitCode` が `0`
* `State` が期待通り更新されている
* `Diagnostics` が期待通りである

warningを返すケースでは、`ExitCode` は `0` のまま、`Diagnostics` にwarningが含まれることを確認します。

失敗時は、`State` を保存しないために `result.State` を `nil` とします。
`Diagnostics` にはerrorを含めます。

command層は、次のように `result.State` が存在する場合だけ保存します。

```go
if result.State != nil {
	store.Save(*result.State)
}
```

### 関数別の主な観点

`ApplyStart` では、有効なFlowとactive Flowなしの状態から、最初の工程を `current_step_id` にした新しい `State` が作られることを確認します。
既に `running` のstateがある場合や、Flowにstepが存在しない場合はエラーにします。
`current == nil` の場合は `no_state` として開始でき、`completed` または `finished` のstateが渡された場合も新しいFlowを開始できることを確認します。
`LoadInvalid` はcommand層で扱うため、`ApplyStart` の入力にはしません。

`ApplyDone` では、gate OKの場合に現在工程が `completed_steps` に追加され、次工程があれば `running` のまま `current_step_id` が進むことを確認します。
最終工程の場合は `status` が `completed` になることを確認します。
現在工程が `skipped_steps` に存在する場合は、`completed_steps` に追加したうえで `skipped_steps` から削除されることを確認します。
gateがartifact不足またはapproval不足の場合はエラーにします。

`ApplyApprove` では、対象工程に `approved: true` とnoteが記録されることを確認します。
`--step` 省略時は現在工程を対象にし、`--step` 指定時は指定工程を対象にします。
対象工程がFlow内に存在しない場合、または承認不要工程の場合はエラーにします。
noteなしの場合は、MVPでは空文字として保存してよいものとします。

`ApplyBack` では、直前工程へ戻り、`back_history` に `from`、`to`、`reason` が記録されることを確認します。
戻り先工程が `completed_steps` に存在する場合は、その工程だけ削除します。
戻り先より後の `completed_steps` を一括整理しないことも確認します。
reasonが空、空白のみ、現在工程が最初の工程、現在工程がFlow内に存在しない、`status` が `running` ではない場合はエラーにします。

`ApplySkip` では、現在工程が `skipped_steps` に記録され、`completed_steps` には入らないことを確認します。
次工程があれば `running` のまま `current_step_id` が進み、最終工程の場合は `status` が `completed` になります。
承認必須工程、必須成果物がある工程、最終工程、最終承認工程をskipした場合は、`ExitCode` は `0` のままwarningを返すことを確認します。
reasonが空、空白のみ、現在工程がFlow内に存在しない、`status` が `running` ではない場合はエラーにします。

`ApplyFinish` では、`status` が `finished` になり、`finish.reason` が保存され、`current_step_id` や既存の `completed_steps`、`skipped_steps`、`approvals` が維持されることを確認します。
reasonが空、空白のみ、`status` が `running` ではない場合はエラーにします。

### Diagnosticのテスト方針

`Diagnostics` のテストでは、表示文言そのものではなく、構造化された情報を確認します。

主に次を確認します。

* `Level`
* `Code`
* `StepID`
* 必要な補足情報

例:

```go
Diagnostic{
	Level: "warning",
	Code: "warning_skipped_required_artifact",
	StepID: "write_review",
}
```

表示文言そのものは、formatter側のテストで確認します。

### Diagnostic Code

MVPでは、最低限次のようなDiagnostic Codeを扱います。

```text
error_no_active_flow
error_invalid_current_step
error_missing_required_artifact
error_missing_required_approval
error_empty_reason
error_no_previous_step
error_flow_already_running
error_flow_has_no_steps
error_approval_not_required
error_invalid_flow_id
error_invalid_step_id
error_invalid_gate_result

warning_skipped_required_approval
warning_skipped_required_artifact
warning_skipped_final_step
warning_skipped_final_approval_step
```

### テストケースの書き方

Goでは table-driven test を基本にします。

```go
func TestApplyDone(t *testing.T) {
	tests := []struct {
		name string
		flow flow.Flow
		state state.State
		gate gate.Result
		wantState state.State
		wantExitCode int
		wantDiagnostics []string
	}{
		{
			name: "moves to next step when gate is ok",
		},
		{
			name: "completes flow when current step is final",
		},
		{
			name: "returns error when required artifact is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.state.Clone()

			got := transition.ApplyDone(tt.flow, tt.state, tt.gate)

			assertStateNotMutated(t, before, tt.state)
			assertTransitionResult(t, got, tt.wantState, tt.wantExitCode, tt.wantDiagnostics)
		})
	}
}
```

テストヘルパーは、次のようなものを用意します。

* `assertStateNotMutated`
* `assertDiagnosticCodes`
* `assertCompletedSteps`
* `assertSkippedSteps`

## gate判定の単体テスト

gate判定の単体テストでは、`devflow done` の通過条件を確認します。

MVPのgateは、次の2つに限定します。

* 必須artifactが存在すること
* 承認必須工程のapprovalが記録されていること

gateテストでは、次を扱いません。

* `state.json` の読み書き
* CUEファイルの読み込み
* Flow定義の構文検証
* artifact path の静的検証
* current step の遷移
* `completed_steps` への追加
* `skipped_steps` の更新
* 標準出力
* `os.Exit`

FlowやStateは、テスト内で構造体として組み立てます。

artifact path の静的検証は、`pathcheck` またはFlow定義検証の責務とします。
gateは、有効なFlow定義を受け取る前提で、実行時に必要なartifactが実際に存在するかを確認します。
artifactの `required` 省略値の解決も、Flow読み込みまたは正規化の責務とします。
gateは、`required` が正規化済みの `Step` を受け取る前提にします。

### gateの関数形

gateの責務を単純にするため、current step は解決済みの `Step` として受け取ります。

```go
func CheckDoneGate(step flow.Step, state state.State, projectRoot string) gate.Result
```

gateは、Flow全体から `current_step_id` を探す処理を担当しません。
current step の解決は、command層またはtransition呼び出し前の処理で行います。

gateの責務は次の通りです。

* `step.artifacts` を確認する
* projectRoot配下にrequired artifactが存在するか確認する
* `step.approval.required` を確認する
* `state.approvals[step.ID].approved` が `true` か確認する

### GateResult

MVPの `GateResult` は、次のような構造を基本とします。

```go
type Result struct {
	OK bool
	MissingArtifacts []string
	MissingApprovals []string
}
```

gateは `MissingArtifacts` と `MissingApprovals` を返します。
Diagnosticへの変換は、transition側に寄せます。

### artifact判定

artifact存在確認は、`t.TempDir()` を使ってテストします。

```go
root := t.TempDir()
writeFile(t, root, "docs/code-review.md", "review")
```

required artifact は、projectRoot配下に通常ファイルとして存在する場合のみ満たされたものとします。
pathが存在してもディレクトリである場合は、不足として扱います。

`required: false` のartifactは完了判定には使いません。
不足していてもgate OKとします。
`required` 省略時に `true` へそろえる処理はgateでは扱わず、Flow読み込みまたは正規化側のテストで確認します。

artifactのテストでは、最低限次を確認します。

* required artifactが存在する場合はOKになる
* required artifactが存在しない場合は `MissingArtifacts` に入る
* required artifactが複数あり一部不足する場合、不足分だけ `MissingArtifacts` に入る
* required artifactが複数ありすべて不足する場合、すべて `MissingArtifacts` に入る
* `required: false` のartifactは不足していてもOKになる
* artifact pathがディレクトリの場合は不足として扱う

### approval判定

`approval.required = true` の工程では、`state.approvals[step.ID].approved = true` の場合だけ承認済みとします。

次の場合は不足として扱い、`MissingApprovals` に `step.ID` を入れます。

* approval記録が存在しない
* approval記録はあるが `approved = false`
* 別工程のapprovalしか存在しない

approvalのテストでは、最低限次を確認します。

* approval不要工程はOKになる
* approval必須工程で `approved = true` の場合はOKになる
* approval必須工程でapproval記録がない場合は不足になる
* approval必須工程で `approved = false` の場合は不足になる
* 別工程のapprovalは現在工程の承認として扱わない

### 複合判定

artifactとapprovalの両方が不足している場合は、最初の不足で止めず、両方を `GateResult` に含めます。

```text
MissingArtifacts:
- docs/code-review.md

MissingApprovals:
- human_approval
```

これにより、AIや人間が一度に不足を把握できます。

### テストケースの書き方

gateも table-driven test を基本にします。

```go
func TestCheckDoneGate(t *testing.T) {
	tests := []struct {
		name string
		step flow.Step
		state state.State
		files []string
		dirs []string
		wantOK bool
		wantMissingArtifacts []string
		wantMissingApprovals []string
	}{
		{
			name: "passes when no artifacts and no approval are required",
		},
		{
			name: "passes when required artifact exists",
		},
		{
			name: "fails when required artifact is missing",
		},
		{
			name: "fails when approval is required but missing",
		},
		{
			name: "reports both missing artifact and missing approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			createFiles(t, root, tt.files)
			createDirs(t, root, tt.dirs)

			got := gate.CheckDoneGate(tt.step, tt.state, root)

			assertGateResult(t, got, tt.wantOK, tt.wantMissingArtifacts, tt.wantMissingApprovals)
		})
	}
}
```

テストヘルパーは、次のようなものを用意します。

* `createFiles`
* `createDirs`
* `assertGateResult`

## state store の読み書きテスト

state store の読み書きテストでは、`.devflow/state.json` の読み込みと保存を確認します。

state store は、`State` 構造体と `state.json` の永続化を担当します。
状態遷移そのものは扱いません。

### state store の責務

state store が扱うことは次の通りです。

* `state.json` の存在確認
* `state.json` の読み込み
* JSON decode
* `State` の最低限の構造確認
* `State` の保存
* atomic write
* 保存後のJSON形式の安定化

state store のテストでは、次を扱いません。

* Flow定義の読み込み
* CUE評価
* Flow内に `current_step_id` が存在するかの確認
* artifactファイルの存在確認
* approvalの意味判定
* `done`、`back`、`skip`、`finish` の状態遷移
* 標準出力
* `os.Exit`

`state.flow_id` が実在するFlowか、`current_step_id` がFlow内に存在するかは、state store ではなく、command層またはactive Flow読み込み時の整合性確認で扱います。

`Store.Load()` は、`state.json` 単体の読み込みと構造検証だけを行います。
Flow定義との突き合わせは行いません。

### 関数形

MVPでは、次のような関数構成を基本とします。

```go
type Store struct {
	Path string
}

func (s Store) Load() LoadResult
func (s Store) Save(state State) error
```

`Load` は、`no_state` と `invalid_state` を区別するため、`LoadResult` を返します。

```go
type LoadStatus string

const (
	LoadNoState LoadStatus = "no_state"
	LoadOK LoadStatus = "ok"
	LoadInvalid LoadStatus = "invalid_state"
)

type LoadResult struct {
	Status LoadStatus
	State *State
	Err error
}
```

期待する扱いは次の通りです。

* `state.json` が存在しない場合は、`Status = no_state`、`State = nil`、`Err = nil` とする
* 正常に読める場合は、`Status = ok`、`State != nil`、`Err = nil` とする
* 壊れている場合は、`Status = invalid_state`、`State = nil`、`Err != nil` とする

ここでいう `invalid_state` は、JSON破損、未知のstatus、必須フィールド不足、型不正など、`state.json` 単体の不正を指します。

### 読み込みテスト

読み込みテストでは、最低限次を確認します。

* `state.json` が存在しない場合は `no_state` になる
* validな `running` state を読める
* validな `completed` state を読める
* validな `finished` state を読める
* 壊れたJSONは `invalid_state` になる
* unknown status は `invalid_state` になる
* 必須フィールド不足は `invalid_state` になる

MVPで保存する `status` は、次の3つだけです。

```text
running
completed
finished
```

最低限、次のフィールドが不足している場合は `invalid_state` とします。

* `flow_id`
* `status`
* `current_step_id`

`completed` または `finished` の場合でも、MVPでは単純さを優先して `current_step_id` を必須とします。

collectionが `null` の場合は、読み込み時に空のslice/mapへ正規化します。

* `completed_steps: null` は `[]` として扱う
* `skipped_steps: null` は `{}` として扱う
* `approvals: null` は `{}` として扱う
* `back_history: null` は `[]` として扱う

ただし、保存時には必ず `[]` または `{}` として出力します。

### 保存テスト

保存テストでは、最低限次を確認します。

* `running` state を保存できる
* `completed` state を保存できる
* `finished` state と `finish.reason` を保存できる
* 保存後に読み直すと同じStateになる
* 既存の `state.json` を新しいStateで置き換えられる
* 空slice/mapが `null` ではなく `[]` / `{}` として保存される
* 親ディレクトリが存在しない場合でも作成して保存できる
* Save成功後に一時ファイルが残らない

空collectionの期待するJSONは次の通りです。

```json
{
  "completed_steps": [],
  "skipped_steps": {},
  "approvals": {},
  "back_history": []
}
```

次のような出力は避けます。

```json
{
  "completed_steps": null,
  "skipped_steps": null,
  "approvals": null,
  "back_history": null
}
```

### atomic write のテスト方針

`Store.Save` は一時ファイルに書き込んでから `state.json` へリネームします。
保存前に `filepath.Dir(path)` を `MkdirAll` し、親ディレクトリが存在しない場合は作成します。

MVPでは、atomic write の失敗系はOSやファイル権限に依存しやすいため、成功系を中心に確認します。

最低限、次を確認します。

* 親ディレクトリが存在しない場合でもSaveできる
* Save成功後、`state.json` が存在する
* Save成功後、一時ファイルが残らない
* 既存 `state.json` が新しい内容に置き換わる

失敗時に既存 `state.json` が保持されることまでテストする場合は、ファイル書き込み処理を小さなインターフェースに分けることを検討します。
MVPでは、そこまでの分離は必須にしません。

### Normalize

state store では、読み込み後と保存前に `Normalize()` 相当の処理を行います。

MVPでは、次の方針とします。

* `Store.Load()` は、Decode後に `Normalize()` した `State` を返す
* `Store.Save()` は、保存前に `Clone()` または `Normalize()` で空slice/mapを整える
* `State.Clone()` は deep copy したうえでNormalize済みの `State` を返す

これにより、読み込み、保存、transitionのどこでも `null` のcollectionが混ざりにくくなります。

### テストケースの書き方

state store も `t.TempDir()` を使った table-driven test を基本にします。

```go
func TestStoreLoad(t *testing.T) {
	tests := []struct {
		name string
		json string
		wantStatus LoadStatus
		wantState *state.State
		wantErr bool
	}{
		{
			name: "returns no_state when state file does not exist",
		},
		{
			name: "loads valid running state",
		},
		{
			name: "returns invalid_state for broken json",
		},
		{
			name: "returns invalid_state for unknown status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			store := state.Store{Path: filepath.Join(root, ".devflow", "state.json")}

			if tt.json != "" {
				writeFile(t, store.Path, tt.json)
			}

			got := store.Load()

			assertLoadResult(t, got, tt.wantStatus, tt.wantState, tt.wantErr)
		})
	}
}
```

保存側は別テストにします。

```go
func TestStoreSave(t *testing.T) {
	tests := []struct {
		name string
		state state.State
		wantJSONContains []string
	}{
		{
			name: "saves empty collections as arrays and objects",
		},
		{
			name: "saves finished state with reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			store := state.Store{Path: filepath.Join(root, ".devflow", "state.json")}

			err := store.Save(tt.state)
			if err != nil {
				t.Fatal(err)
			}

			gotBytes := readFile(t, store.Path)
			assertJSONContains(t, gotBytes, tt.wantJSONContains)
		})
	}
}
```

テストヘルパーは、次のようなものを用意します。

* `writeFile`
* `readFile`
* `assertLoadResult`
* `assertStateEqual`
* `assertJSONHasArray`
* `assertJSONHasObject`
* `assertNoTmpFile`

JSONの比較は、文字列完全一致よりdecodeして構造比較する方が安定します。
ただし、`[]` と `null`、`{}` と `null` の違いを確認したいケースでは、JSONを `map[string]any` にdecodeして型を見るか、文字列含有で確認します。

## command層の統合テスト

command層の統合テストでは、Flow読み込み、state読み込み、gate判定、transition呼び出し、state保存、表示が正しく接続されていることを確認します。

transitionやgateの全パターンをcommand層で再確認しすぎず、代表的な正常系、異常系、warning付き成功を確認します。

### 基本方針

テストは `t.TempDir()` をプロジェクトルートとして使います。
`.devflow/flows/`、`state.json`、artifactファイルは実ファイルとして作成します。

MVPでは、実CLIプロセスを起動するのではなく、commandパッケージの関数を直接呼びます。
CLI引数パースは薄い層として、別途最小限テストすれば十分です。

command関数は `os.Exit` を直接呼ばず、終了コードを値として返します。
`main.go` だけが最後に `os.Exit(code)` を呼びます。

stdout / stderr は `io.Writer` として注入し、テストでは `bytes.Buffer` で確認します。

```go
type Context struct {
	ProjectRoot string
	Stdout io.Writer
	Stderr io.Writer
}

type CommandResult struct {
	ExitCode int
}
```

成功、失敗、warning付き成功は、次のように区別して確認します。

* 成功時は、`ExitCode = 0`、`state.json` の更新、必要なstdoutを確認する
* 失敗時は、`ExitCode != 0`、`state.json` が更新されないこと、エラー出力を確認する
* warning付き成功では、`ExitCode = 0`、`state.json` が更新されること、warningが出力されることを確認する

### コマンド別の代表ケース

`init` では、空のプロジェクトルートに `.devflow/`、`.devflow/flows/`、`.devflow/.gitignore`、`.devflow/flows/post-task-review.cue` が作成され、`.devflow/state.json` が作成されないことを確認します。
既存ファイルがある場合は上書きしません。
MVPでは、既存ファイルを保ったうえで `ExitCode = 0` とし、noticeを表示する方針とします。

`list` では、有効なFlowのID、title、description、工程数、valid表示を確認します。
壊れたFlowがある場合は、そのFlowを `invalid` として表示し、有効なFlowも表示したうえで非0を返すことを確認します。

`start` では、stateがない場合に有効なFlowを開始し、`state.json` が `running`、指定Flow ID、最初の工程ID、空collection、`finish = null` で作成されることを確認します。
active Flow が既にある場合は非0で終了し、`state.json` を変更しません。
`completed` または `finished` のstateがある場合は、新しいFlowを開始できることを確認します。
存在しないFlowを指定した場合は非0で終了し、`state.json` を作成しません。

`status` では、active Flow のFlow ID、Flow title、current step ID、current step title、completed steps、skipped steps、approval stateが表示されることを確認します。
active Flow がない場合やstateが壊れている場合は非0で終了します。

`prompt` では、現在工程のFlow、Step、Instruction、Required artifacts、Required approval、After completingが表示されることを確認します。
required artifact や approval がある工程では、それらが表示されることを確認します。
optional artifact がある工程では、`Optional artifacts` が表示されることを確認します。
active Flow がない場合は非0で終了します。

`approve` では、現在工程または `--step` で指定した承認必須工程に `approved = true` とnoteが保存されることを確認します。
承認不要工程への `approve` は非0で終了し、`state.json` を変更しません。

`done` では、gate OKの場合に現在工程が `completed_steps` に入り、次工程があれば `running` のまま進み、最終工程なら `completed` になることを確認します。
required artifact不足やrequired approval不足の場合は非0で終了し、`state.json` を変更せず、不足情報を表示します。
required artifactが実ファイルとして存在する場合やapproval済みの場合は進めることを確認します。

`back` では、直前工程へ戻り、戻り先工程が `completed_steps` から削除され、`back_history` に `from`、`to`、`reason` が記録されることを確認します。
reason不足、空白のみ、最初の工程でのbackは非0で終了し、`state.json` を変更しません。

`skip` では、現在工程が `skipped_steps` に記録され、`completed_steps` には入らず、次工程があれば進むことを確認します。
required artifactがある工程、approval必須工程、最終工程のskipでは、`ExitCode = 0`、`state.json` 更新、warning表示を確認します。
最終工程をskipした場合は `completed` になります。
reason不足または空白のみの場合は非0で終了し、`state.json` を変更しません。

`finish` では、active Flow が理由付きで `finished` になり、`finish.reason` が保存され、`current_step_id` が維持されることを確認します。
reason不足または空白のみの場合は非0で終了し、`state.json` を変更しません。

### 共通エラー系

active Flow を必要とする主なコマンドは、active Flow がない場合にエラーになります。

```text
status
prompt
done
approve
back
skip
finish
```

command層では、これらを共通helperまたはtable-driven testで代表的に確認します。

最低限、次を確認します。

* `state.json` が存在しない場合は非0で終了し、`state.json` を作成しない
* `completed` または `finished` のstateでは非0で終了する
* `invalid_state` の場合は非0で終了し、`state.json` を変更しない

Flow定義との整合性エラーもcommand層またはactive Flow読み込み時の責務です。

次の場合は、非0で終了し、`state.json` を変更しません。

* `state.flow_id` が存在しないFlowを参照している
* `current_step_id` がFlow内に存在しない

### 表示テスト

command層では、表示文言の完全一致は避けます。
文言は変わりやすいため、必要な情報が含まれていることを確認します。

主に次を確認します。

* Flow ID
* Step ID
* artifact path
* approval required
* error code または短いエラー見出し
* warning 見出し

`prompt` はAIが読む出力なので、最低限次の構造を確認します。

```text
Flow:
Step:
Instruction:
Required artifacts:
Required approval:
After completing:
```

### テストヘルパー

command統合テストでは、次のようなヘルパーを用意します。

* `setupProject`
* `writeFlow`
* `writeBrokenFlow`
* `readState`
* `writeState`
* `createArtifact`
* `runCommand`
* `assertExitCode`
* `assertState`
* `assertStateUnchanged`
* `assertOutputContains`
* `assertOutputNotContains`

`assertStateUnchanged` は、基本的にはテスト前後で読み込んだ `State` の構造比較を行います。
atomic write やJSON整形差分の影響を避けるため、文字列完全一致だけには依存しません。

失敗時に保存処理が走っていないことまで確認したいケースでは、補助的に `state.json` のバイト列比較や更新時刻の比較を使います。

## 実装構成

MVPの実装では、CLI層、Flow読み込み層、状態保存層、状態遷移層、gate判定層、コマンド実行層を分けます。

```text
cmd/
  devflow/
    main.go

internal/
  flow/
    loader.go
    validate.go
    model.go

  state/
    store.go
    model.go

  transition/
    result.go
    start.go
    done.go
    approve.go
    back.go
    skip.go
    finish.go

  gate/
    check.go
    model.go

  command/
    init.go
    list.go
    start.go
    status.go
    prompt.go
    done.go
    approve.go
    back.go
    skip.go
    finish.go

  pathcheck/
    artifact.go
```

### `flow`

Flow定義の読み込み、CUE評価、検証を担当します。

### `state`

`state.json` の読み書きとState構造体を担当します。

### `transition`

副作用を持たない状態遷移ロジックを担当します。

Flow定義、現在のstate、コマンド引数、gate判定結果を受け取り、新しいstateと診断結果を返します。

### `gate`

`devflow done` の通過条件を確認します。

artifactファイルの存在確認と、承認記録の確認を行います。

### `command`

各CLIコマンドの組み立てを担当します。

Flow読み込み、state読み込み、gate判定、transition呼び出し、state保存、表示をつなぎます。

### `pathcheck`

artifact path の検証を担当します。

## コマンド別処理概要

### `init`

* `.devflow/` を作成する
* `.devflow/flows/` を作成する
* `.devflow/.gitignore` を作成する
* `.devflow/flows/post-task-review.cue` を作成する
* 既存ファイルは原則として上書きしない
* `state.json` は作成しない

### `list`

* `.devflow/flows/` 配下のFlow定義を読む
* 有効なFlowを表示する
* 壊れたFlowを `invalid` として表示する
* 壊れたFlowが1つでもあれば非0で終了する

### `start`

* `.devflow/flows/<flow>.cue` を読み込む
* Flow定義を検証する
* 読み込んだ `flow.id` が `<flow>` と一致することを確認する
* active Flow が存在する場合はエラーにする
* 最初の工程を current step として `state.json` を作成する

### `status`

* active Flow を読み込む
* stateを読み込む
* 現在の状態を表示する

### `prompt`

* active Flow を読み込む
* current step を取得する
* AI向け指示を表示する

### `done`

* `state.json` を読む
* Flow定義を読む
* `state.current_step_id` からcurrent stepを解決する
* 解決済みStepを `CheckDoneGate(step, state, projectRoot)` に渡す
* `gate.Result` を `ApplyDone(flow, state, gateResult)` に渡す
* `ApplyDone` の結果に `State` があれば保存する

`CheckDoneGate` はcurrent step探索を担当しません。

`ApplyDone` はgateの再判定はしませんが、防御的に `state.current_step_id` がFlow内に存在することは確認します。

### `approve`

* active Flow を読み込む
* 対象工程を決める
* 対象工程が承認必須工程であることを確認する
* 承認を記録する
* noteがあれば保存する

### `back`

* active Flow を読み込む
* reasonを確認する
* 前の工程が存在するか確認する
* back履歴を保存する
* current step を前の工程に戻す

### `skip`

* active Flow を読み込む
* reasonを確認する
* reasonが空文字または空白のみの場合はエラーにする
* current step を skipped として記録する
* 承認必須工程、必須成果物がある工程、最終工程をskipする場合はwarningを返す
* 次工程があれば進める
* 次工程がなければ `completed` にする

### `finish`

* active Flow を読み込む
* reasonを確認する
* `status` を `finished` にする
* finish reason を保存する

## 実装タスク分解

MVPでは、下位層をテストで固定してから上位層を組み立てます。

実装順序は次の通りです。

```text
1. プロジェクト土台
2. 共通モデル
3. State.Clone / Normalize
4. state store
5. pathcheck
6. flow loader / validate / normalize
7. gate
8. transition
9. command
10. cmd/devflow
11. initテンプレート
12. 統合確認
```

### 1. プロジェクト土台

Goプロジェクトとして実装を始められる状態を作ります。

対象:

```text
cmd/devflow/main.go
internal/flow/
internal/state/
internal/transition/
internal/gate/
internal/command/
internal/pathcheck/
```

完了条件:

```bash
go test ./...
go build ./cmd/devflow
```

### 2. 共通モデル

各層で使う構造体を先に定義します。

対象:

```text
internal/flow/model.go
internal/state/model.go
internal/gate/model.go
internal/transition/result.go
```

作るもの:

* `Flow` / `Step` / `Artifact` / `Approval`
* `State` / `SkippedStep` / `ApprovalRecord` / `BackHistory` / `Finish`
* `Diagnostic`
* `GateResult`
* `TransitionResult`

完了条件:

* 各モデルがコンパイルできる
* JSON tag が `state.json` の設計と一致している
* CUE読み込み前でもテスト用構造体を組み立てられる

### 3. State.Clone / Normalize

状態遷移関数が入力 `State` を破壊しないための土台を作ります。

対象:

```text
internal/state/model.go
internal/state/clone.go
internal/state/clone_test.go
```

完了条件:

* Clone後に `CompletedSteps`、`SkippedSteps`、`Approvals`、`BackHistory`、`Finish` を変更しても元Stateが変わらない
* nil slice / map が空 slice / map に正規化される
* `Finish` pointer がdeep copyされる

### 4. state store

`.devflow/state.json` の読み書きを実装します。

対象:

```text
internal/state/store.go
internal/state/store_test.go
```

完了条件:

* stateなしを `no_state` として読める
* validな `running` / `completed` / `finished` stateを読める
* broken JSON、unknown status、必須フィールド不足を `invalid_state` にできる
* null collection をLoad時に空へ正規化できる
* Save後に `state.json` が作成される
* Save後にtmpファイルが残らない
* 空 slice / map が `[]` / `{}` として保存される

### 5. pathcheck

artifact path の静的検証を実装します。

対象:

```text
internal/pathcheck/artifact.go
internal/pathcheck/artifact_test.go
```

完了条件:

* `/` 区切りのプロジェクト相対パスをvalidにできる
* 絶対パス、`..`、URL、glob、末尾スラッシュ、空文字、空白のみをinvalidにできる
* Windows形式の絶対パスや親ディレクトリ参照もinvalidにできる

### 6. flow loader / validate / normalize

`.devflow/flows/*.cue` を読み込み、内部 `Flow` 構造体へ変換します。

対象:

```text
internal/flow/loader.go
internal/flow/validate.go
internal/flow/normalize.go
internal/flow/loader_test.go
internal/flow/validate_test.go
internal/flow/normalize_test.go
```

完了条件:

* 最小Flowを読み込める
* descriptionあり / なしを扱える
* artifactありstep、approvalありstepを読み込める
* step順序が維持される
* 必須項目不足、空文字 / 空白のみ、step ID重複をinvalidにできる
* `flow.id` / `step.id` の文字種制限を確認できる
* artifact path不正をinvalidにできる
* ファイル名と `flow.id` の不一致をinvalidにできる
* valid / invalid 混在でも読み続けられる

### 7. gate

`devflow done` の通過条件を判定します。

対象:

```text
internal/gate/check.go
internal/gate/check_test.go
```

完了条件:

* artifact / approval 不要工程はOKになる
* required artifact が通常ファイルとして存在すればOKになる
* required artifact がない、またはディレクトリなら `MissingArtifacts` になる
* approval必須工程で `approved = true` の場合だけOKになる
* approval不足とartifact不足をまとめて返せる

### 8. transition

副作用を持たない状態遷移関数を実装します。

対象:

```text
internal/transition/start.go
internal/transition/done.go
internal/transition/approve.go
internal/transition/back.go
internal/transition/skip.go
internal/transition/finish.go
internal/transition/result.go
internal/transition/*_test.go
```

完了条件:

* 成功時は `result.State != nil`
* 失敗時は `result.State == nil`
* warning付き成功は `ExitCode = 0`
* 入力 `State` を変更しない
* `done` / `approve` / `back` / `skip` / `finish` の代表ケースが通る
* skip warning と `back_history` が期待通り返る

### 9. command

各CLIコマンドの処理を組み立てます。

対象:

```text
internal/command/context.go
internal/command/init.go
internal/command/list.go
internal/command/start.go
internal/command/status.go
internal/command/prompt.go
internal/command/approve.go
internal/command/done.go
internal/command/back.go
internal/command/skip.go
internal/command/finish.go
internal/command/*_test.go
```

完了条件:

* stdout / stderr を `io.Writer` 注入にできる
* `Diagnostic`、prompt、status の表示をcommand内で最小限整形できる
* `init` が `.devflow/` を作成し、`state.json` を作らない
* `list` がvalid / invalid Flowを表示する
* `start` が `.devflow/flows/<flow>.cue` を読み、`state.json` を作成する
* `status` / `prompt` / `approve` / `done` / `back` / `skip` / `finish` の代表ケースが通る
* 失敗時に `state.json` を変更しない
* warning付き成功を `ExitCode = 0` として扱える
* `invalid_state` やstate / Flow不整合を自動修復せずエラーにできる

### 10. cmd/devflow main.go

実CLIとして呼べるようにします。

対象:

```text
cmd/devflow/main.go
```

完了条件:

* `os.Args` を解析できる
* サブコマンドをcommand層へルーティングできる
* `main.go` だけが `os.Exit(code)` を呼ぶ
* 引数なし、unknown command、代表コマンドの最小テストが通る
* `go build ./cmd/devflow` が通る

### 11. initテンプレート

`devflow init` で作られる初期Flowと `.gitignore` を実装します。

対象:

```text
internal/command/init.go
internal/command/templates/
```

Goファイル内の文字列定数として持っても構いません。

完了条件:

* `post-task-review.cue` が作られる
* `post-task-review.cue` の `flow.id` とファイル名が一致する
* `.devflow/.gitignore` に `state.json` が含まれる
* `state.json` は作られない
* 既存ファイルを上書きせずnoticeを出す

### 12. 統合確認

MVPとして一連の流れが動くことを確認します。

確認する流れ:

```text
init
list
start post-task-review
prompt
done
done
done
artifact作成
done
approve
done
```

確認内容:

* 最後に `status = completed` になる
* `docs/code-review.md` がない場合は `done` で止まる
* approval がない場合は `done` で止まる
* `approve` 後に `done` できる
* 手動配置された `purpose-driven-development` を `start` / `prompt` / `done` できる

完了条件:

```bash
go test ./...
go build ./cmd/devflow
```

## MVPで扱わない設計

MVPでは、次の設計は行いません。

* 複数Flowの同時実行
* 条件分岐
* 任意の工程へのgoto
* `completed_steps` を直接編集するコマンド
* subflow呼び出し
* Flow定義のバージョン管理
* Flow変更時のstateマイグレーション
* 成果物の中身の検査
* コマンド実行
* 禁止コマンド制御
* `--json` 出力
* `devflow validate`
* `devflow reset`
* `devflow uncomplete <step>`
* `devflow mark-pending <step>`

## 今後の拡張候補

MVP後に、必要に応じて次を検討します。

* `devflow validate`
* `devflow reset`
* `devflow status --json`
* `devflow prompt --json`
* 任意の工程へ戻る `back --to <step>`
* Flow定義の部品化
* 複数Flowの連携
* fileMatchによるFlow提案
* 表示処理の `internal/format` への分離
* 別ツールによる禁止コマンド制御との連携

## まとめ

MVPのdevflowは、AI支援開発における進行管理をAIの会話文脈の外に置くCLIツールです。

Flow定義は共有し、状態はローカルに保存します。
AIは `devflow prompt` を見て現在の工程を確認し、各工程の中で調査・設計・実装・レビューを行います。

devflow は、工程順序、成果物、承認、戻り、スキップ、終了を管理します。

MVPでは、進行管理に集中し、自動実行、禁止コマンド制御、複雑な条件式、AI API連携は扱いません。
