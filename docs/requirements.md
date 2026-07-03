# devflow MVP 要件定義書

## 目的

devflow の目的は、人間が毎回チャットや設定ファイルで伝えていた進行指示を、機械的に実行できる Flow として扱うことです。

> 固定すると安定するものは devflow が担い、流動性が価値になるものは AI に任せる。

AI支援開発では、AIに長い設定ファイルや手順書を読ませて、毎回同じ流れで作業してもらうことがあります。

進行順序、確認タイミング、承認、現在地の管理は、固定すると安定しやすい領域です。
devflow は、この領域を AI の会話文脈の外に置きます。

AIは、devflow に現在の工程（step）を確認しながら、各工程の中で調査・設計・実装・レビューを進めます。
devflow は、工程順序、通過条件（gate）、成果物（artifact）、承認（approval）、現在地（state）を管理します。

これにより、AIの判断負荷、設定ファイルの読み込み量、人間が繰り返し入力していた進行指示を削減します。

## 背景

現在、Kiro のグローバルステアリングを共有開発環境に配布し、開発環境に入った利用者全員に共通ルールを適用しています。

共有開発環境では、Kiro IDE から Remote SSH で接続し、`/etc/skel/.kiro/steering/` を正本として配置しています。
その内容を `rsync --delete` によって各ユーザーの `~/.kiro/steering/` に配布し、全員に共通のステアリングを適用しています。

この仕組みにより、AIに対して共通の開発ルールや作業方針を伝えられます。

一方で、ステアリングが長くなるほど、AIは多くの設定ファイルを読み込み、内容を解釈し、次に何をするかを判断する必要があります。

ステアリング内には、次のような順序を持つ進行指示があります。

* 作業前に確認すること
* 要件定義へ進む前に確認すること
* タスク後にレビューすること
* コミット前に確認すること
* ドキュメント同期を確認すること
* 人間の承認を求めること

これらは、AIが毎回判断し直すより、機械的に現在地と次の工程を管理した方が安定します。

devflow は、このうち「順序がある進行指示」を Flow として外に出し、AIが必要なタイミングで次の工程だけを確認できるようにします。

## 解決したい問題

AI支援開発では、次のような問題があります。

* 人間が同じ進行指示を何度もチャットで入力している
* AIが長い設定ファイルを読み続けることで文脈を消費している
* AIが現在どの工程にいるかを会話の文脈に依存している
* 要件定義、設計、レビュー、承認などの順序が崩れることがある
* 人間承認の前に次の工程へ進むことがある
* 必須成果物がないまま完了扱いになることがある
* ステアリングにフローと環境ルールが混ざり、AIの判断負荷が高くなっている

devflow は、これらのうち、固定すると安定する進行管理を担当します。

人間が毎回AIに伝えていた「次はこれをしてほしい」「ここでは止まってほしい」「承認が必要」「この成果物を残してほしい」という指示を、Flowとして扱います。

AIは、Flowの各工程の中で、調査、設計、実装、レビュー、分析、提案を行います。
devflow は、AIが作業を進めるための現在地と次の工程を提示します。

## devflow が担うこと

devflow が担うのは、人間がAIに対して毎回繰り返している進行指示です。

特に、Markdownで書かれたステアリングやチャット指示のうち、順序があり、機械的に現在地を管理した方が安定するものを扱います。

devflow が担うことは、次の通りです。

* Flow定義を読み込む
* 利用可能なFlowを一覧表示する
* 現在の工程（step）を記録する
* AIに次の作業指示を提示する
* 工程の完了を記録する
* 必須成果物（artifact）の存在を確認する
* 承認（approval）が必要な工程を管理する
* 通過条件（gate）を満たした場合に次の工程へ進める
* 必要に応じて前の工程へ戻る
* 必要に応じて工程をスキップする
* 対象外になったFlowや途中で終えたいFlowを終了する
* 現在地（state）をAIの会話文脈ではなく、devflow側で保持する

devflow は、AIが作業を始める前に「今やること」を提示します。

AIは `devflow prompt` の出力を確認し、その工程の中で作業します。
工程が終わったら、`devflow done` によって次の工程へ進めます。

## AI に任せること

AIには、流動性が価値になる作業を任せます。

AIが担うことは、次の通りです。

* 人間の依頼内容を読む
* 目的を整理する
* 要件定義の下書きを作る
* 設計案を出す
* 人間への質問を作る
* Web検索する
* MCPツールを使う
* ローカルドキュメントを検索する
* ソースコードを読む
* 既存プロジェクトの慣習を推測する
* ベストプラクティスを調べる
* コードを書く
* テストを実行する
* エラーを分析する
* diffをレビューする
* フォーマッターやリンターの結果を分析する
* セキュリティチェックの結果を読む
* コミットメッセージを提案する
* 今後参照した方がよいルールやFlow候補を提案する

AIは、各工程の中で判断し、調査し、作業します。
devflow は、その作業をどの順序で進めるか、どこで止まるか、どの成果物や承認が必要かを管理します。

## 人間が担うこと

人間は、目的を決め、重要な判断を承認します。

人間が担うことは、次の通りです。

* 作業の目的を決める
* AIからの質問に答える
* 要件定義を承認する
* 設計を承認する
* 重要な修正を承認する
* リスクを受け入れるか判断する
* コミットやデプロイの最終判断をする
* 成果物が本来の目的に合っているか判断する
* devflow の Flow 定義を採用するか判断する

devflow は、人間が毎回入力していた進行指示を減らします。
人間は、進行手順の細かい繰り返しではなく、目的、判断、承認に集中します。

## MVPで扱うユースケース

MVPでは、devflow の中心概念を確認するため、進行指示の固定化に効果が高いユースケースから扱います。

Flowは、次の3分類で整理します。

* MVPで必須実行する公開サンプルFlow
* 開発者自身の検証用Flow
* MVP後に追加できるFlow候補

MVPの必須実装対象は、公開サンプルFlowである `post-task-review` を中心にします。

`purpose-driven-development` は、公開サンプルとして前面に出すのではなく、開発者自身のdogfooding用Flowとして扱います。

MVP後に追加できるFlow候補は、MVPの必須実装対象には含めません。

### MVPで必須実行する公開サンプルFlow

#### タスク後レビューフロー

MVPの公開サンプルFlowとして、タスク後レビューフローを扱います。

タスク後レビューフローは、AIによる実装や修正が完了した後に、変更内容、テスト、レビュー、関連ルールの確認を行うためのFlowです。

このFlowでは、次のような進行を扱います。

* 対象判定
* 変更ファイル確認
* 変更内容の要約
* 依頼内容とのずれの確認
* テスト・lint・型チェックなどの確認
* コードレビュー
* 問題点の整理
* 必要な修正
* 人間承認

このFlowをMVPサンプルにする理由は、AI支援開発で多く発生する「作ったものが意図とずれる」「確認前に完了扱いになる」という問題に対して、devflow の価値を示しやすいためです。

### 開発者自身の検証用Flow

#### 目的中心の開発フロー

目的中心の開発フローは、開発者自身の検証用Flowとして扱います。

このFlowは、目的を中心に、要件定義、設計、タスク化、実装、検証、文書整理までを進めるためのFlowです。

MVPでは、公開サンプルとして前面に出すのではなく、devflow 自体を作るための dogfooding 用Flowとして扱います。

`purpose-driven-development` は、`devflow init` では作成しません。

`purpose-driven-development` は、開発者自身の検証環境で `.devflow/flows/purpose-driven-development.cue` に手動配置する dogfooding 用Flowとして扱います。

このFlowは、公開サンプルではなく、devflow 自体の開発・検証に使うFlowです。

### MVP後に追加できるFlow候補

次のFlowは、MVP後に追加できる候補として扱います。

* ドキュメント同期フロー
* バグ修正フロー
* コミット前レビューフロー
* Flow作成フロー
* 設定確認フロー

これらはdevflowの考え方と相性が良いですが、MVPの必須実装対象には含めません。

## Flow定義と状態ファイルの置き場所

MVPでは、Flow定義と状態ファイルを次の場所に置きます。

`devflow init` 後は、次の状態になります。

```text
.devflow/
  flows/
    post-task-review.cue
  .gitignore
```

`devflow start <flow>` 後は、ローカル状態として `state.json` が作成されます。

```text
.devflow/
  flows/
    post-task-review.cue
  .gitignore
  state.json
```

### `.devflow/flows/`

Flow定義を置くディレクトリです。

`.devflow/flows/` は、原則としてGit管理対象にします。

MVPでは、まず `post-task-review.cue` を公開サンプルFlowとして扱います。

将来的に、次のようなFlowを追加できます。

```text
.devflow/flows/
  post-task-review.cue
  doc-sync.cue
  bugfix.cue
  commit-review.cue
  purpose-driven-development.cue
```

### `.devflow/state.json`

現在地（state）を保存するファイルです。

現在どのFlowのどの工程（step）にいるか、完了済み工程、承認、スキップ、戻り、終了理由などを記録します。

`.devflow/state.json` は、作業者、ブランチ、AIセッション、現在の依頼に依存するローカル状態です。
そのため、原則としてGit管理対象外にします。

MVPでは、`.devflow/.gitignore` に次の内容を置きます。

```gitignore
state.json
```

## Flow定義の最小要件

MVPのFlow定義は、最低限次の情報を持ちます。

### Flow

Flowは次の情報を必須で持ちます。

* `id`
* `title`
* `steps`

Flowは、必要に応じて次の情報を持ちます。

* `description`

### 工程（step）

工程は次の情報を持ちます。

* `id`
* `title`
* `instruction`

工程は、必要に応じて次の情報を持ちます。

* `artifacts`
* `approval`

### IDの扱い

Flow IDは、Flowを識別するためのIDです。

工程IDは、Flow内で一意にします。
MVPでは、工程IDが全Flowで一意であることは求めません。

### 工程順序

工程の順序は、Flow定義内の `steps` の並び順で扱います。

MVPでは、複雑な条件分岐や自動的な遷移先指定は扱いません。

## 通過条件（gate）のMVP仕様

MVPの通過条件（gate）は、次の2つに限定します。

* 必須成果物が存在すること
* 承認が必要な工程で承認が記録されていること

MVPでは、次のような通過条件は扱いません。

* 任意条件式
* コマンド実行結果による判定
* 成果物の中身の判定
* テスト結果の自動判定
* 外部サービスの状態確認

成果物の中身の妥当性はAIが確認し、人間が必要に応じて承認します。
devflow は、MVPでは成果物の存在と承認記録のみを確認します。

## 成果物パスの扱い

MVPでは、成果物（artifact）はプロジェクトルートからの相対ファイルパスとして扱います。

次のようなパスは扱いません。

* 絶対パス
* `..` を含むパス
* URL
* 外部ストレージ
* glob
* ディレクトリ

MVPでは、成果物はファイルのみを対象にします。
空ファイルでも、ファイルが存在すれば存在扱いにします。

## 状態管理の基本方針

MVPでは、同時に実行できるFlowは1つだけとします。

activeなFlowが存在する状態で `devflow start <flow>` を実行した場合はエラーにします。
別のFlowを開始する場合は、現在のFlowを `devflow finish` で終了してから開始します。

stateには、最低限次の情報を記録します。

* 現在のFlow
* 現在の工程
* 完了済み工程
* スキップ済み工程
* 承認
* 戻り操作
* 終了状態
* 終了理由

MVPでは、Flow定義とstateの不整合を自動修復しません。

stateが参照するFlowや工程が存在しない場合は、エラーを表示します。
`state.json` が壊れている場合も、エラーを表示します。

## MVPの機能要件

MVPでは、devflowを最小限の進行管理ツールとして実装します。

### Flow一覧を表示できること

devflow は、利用可能なFlowを一覧表示できます。

`devflow list` は、Flowを選ぶときや、現在のFlowに違和感があるときに使います。

表示内容には、次の情報を含めます。

* Flow ID
* Flow title
* Flow description（存在する場合）
* 工程数
* 現在実行中のFlow

### Flow定義を読み込めること

devflow は、`.devflow/flows/` 配下の Flow 定義を読み込めるようにします。

Flow定義は、英語のIDと設定キーを使います。
説明文、表示名、AIへの指示文は日本語で書けます。

### 現在地を保存できること

devflow は、現在どのFlowのどの工程にいるかを `.devflow/state.json` に保存します。

現在地は、AIの会話文脈ではなく、devflowの状態ファイルに保存します。

### 現在の工程を表示できること

devflow は、現在の工程を表示できます。

表示内容には、次の情報を含めます。

* Flow ID
* Flow title
* 工程ID
* 工程title
* 完了済み工程
* 必要な成果物
* 承認の有無

### AIへの作業指示を表示できること

devflow は、現在の工程に書かれたAI向けの作業指示を表示できます。

AIは、作業の節目で `devflow prompt` を確認し、その出力に従って作業します。

`devflow prompt` は、最低限次の情報を表示します。

* Flow ID
* Flow title
* 現在の工程ID
* 現在の工程title
* AIへの指示
* 必要な成果物
* 承認の要否
* 作業後に使うdevflowコマンド

### 工程を完了できること

devflow は、現在の工程を完了扱いにできます。

工程を完了すると、現在地は次の工程へ進みます。

成果物や承認が不足している場合は、完了扱いにしません。

最後の工程で `devflow done` を実行した場合、Flowは完了状態になります。

### 成果物を確認できること

工程に成果物（artifact）が指定されている場合、devflow はそのファイルが存在するか確認します。

成果物が存在しない場合、その工程は完了扱いにしません。
不足している成果物がある場合は、どのファイルが不足しているかを表示します。

### 承認を記録できること

工程に承認（approval）が必要な場合、devflow は人間の承認を記録します。

承認には、任意のメモを残せます。

例:

```json
{
  "approvals": {
    "review_result": {
      "approved": true,
      "note": "指摘事項を反映済み。次へ進めてよい。"
    }
  }
}
```

承認が必要な工程では、承認が記録されるまで完了扱いにしません。
承認が不足している場合は、どの工程の承認が不足しているかを表示します。

### 前の工程へ戻れること

devflow は、必要に応じて前の工程へ戻れるようにします。

`devflow back` は、理由を記録します。

例:

```bash
devflow back --reason "レビューで設計見直しが必要になったため"
```

MVPでは、戻り操作の詳細な状態遷移は設計書で定義します。

### 工程をスキップできること

devflow は、現在の工程をスキップできます。

`devflow skip` は、理由を必須とします。

例:

```bash
devflow skip --reason "今回はREADME更新が不要なため"
```

スキップ理由は、状態ファイルに記録します。

スキップは、人間が明示した場合だけ実行する運用とします。
ただし、devflow本体では人間判定を強制しません。

### Flowを終了できること

devflow は、現在のFlowを終了できます。

`devflow finish` は、理由を必須とします。

例:

```bash
devflow finish --reason "対象外の変更だったため"
```

終了理由は、状態ファイルに記録します。

### Flow定義の形式を確認できること

devflow は、Flow定義として必要な項目が存在するか確認します。

MVPでは、主に次の項目を確認します。

* Flow ID
* Flow title
* 工程の存在
* 工程ID
* 工程title
* 工程instruction

## コマンドの成功・失敗方針

各コマンドは、成功時に終了コード `0` を返します。

次のような場合は、非0の終了コードを返します。

* active Flow を必要とするコマンドで、active Flow が存在しない
* activeなFlowが存在する状態で別のFlowを開始しようとした
* 指定されたFlowが存在しない
* Flow定義が不正
* stateが壊れている
* stateが存在しないFlowや工程を参照している
* 必須成果物が不足している
* 必須承認が不足している

active Flow を必要とする主なコマンドは、次の通りです。

* `devflow status`
* `devflow prompt`
* `devflow done`
* `devflow approve`
* `devflow back`
* `devflow skip`
* `devflow finish`

`devflow init`、`devflow list`、`devflow start <flow>` は、active Flow が存在しない状態でも実行できます。

失敗時は、人間とAIが次の行動を判断できるエラーメッセージを表示します。

MVPでは、人間とAIが読めるテキスト出力を優先します。
機械可読な `--json` 出力は、将来の拡張として検討します。

## MVPで扱わないこと

devflow本体は、進行管理として最小限の機能に集中します。

次の内容は、devflow本体では扱いません。

* 禁止コマンドの強制ブロック
* 自動コマンド実行
* 複雑な条件式エンジン
* 成果物の中身の自動判定
* AI APIの直接呼び出し
* MCPのオーケストレーション
* Web検索機能
* テスト実行機能
* CI/CD機能
* 複数Flowの自動合成
* subflow呼び出し
* fileMatchによる自動Flow起動
* policy定義の本格実装
* capability定義の本格実装
* Flow定義のバージョン管理
* Flow定義変更時のstate自動マイグレーション

禁止コマンドの制御は、Flowではなく実行安全性の領域として扱います。
必要になった場合は、devflow本体に含めるのではなく、別ツールとして作り、devflowと連携します。

MVPでは、現在地、工程、成果物、承認、戻り、スキップ、終了を中心にします。

## MVPコマンド案

MVPでは、次のコマンドを中心にします。

```bash
devflow init
devflow list
devflow start <flow>
devflow status
devflow prompt
devflow done
devflow approve [--step <step>] [--note <note>]
devflow back --reason <reason>
devflow skip --reason <reason>
devflow finish --reason <reason>
```

### `devflow init`

devflow用の初期ディレクトリとサンプルFlowを作成します。

作成するものは次の通りです。

* `.devflow/`
* `.devflow/flows/`
* `.devflow/.gitignore`
* `.devflow/flows/post-task-review.cue`

既存ファイルがある場合は、原則として上書きしません。

`state.json` は、`devflow init` では作成しません。

`state.json` は、`devflow start <flow>` によってFlowを開始した時に作成します。
これは、state が active Flow の現在地を表すローカル状態であり、Flow開始前には意味を持たないためです。

`.devflow/.gitignore` には、次の内容を入れます。

```gitignore
state.json
```

### `devflow list`

利用可能なFlowを一覧表示します。

Flow ID、表示名、説明、工程数を確認できます。
作業に合うFlowを選ぶときや、現在のFlowに違和感があるときに使います。

Flow定義が壊れている場合、そのFlowは一覧から隠さず、`invalid` として表示します。  
壊れたFlowが1つでも存在する場合、`devflow list` は非0の終了コードを返します。

これにより、利用可能なFlowを確認しながら、修正が必要なFlowにも気づけるようにします。

### `devflow start <flow>`

指定されたFlowを開始し、最初の工程を現在地として保存します。

Flow定義の形式確認は、`devflow start <flow>` の実行時に行います。

MVPでは、同時に実行できるFlowは1つだけです。
activeなFlowが存在する状態で実行した場合はエラーにします。

### `devflow status`

現在のFlow、現在の工程、完了済み工程、承認状態を表示します。

### `devflow prompt`

現在の工程でAIが行う作業指示を表示します。

### `devflow done`

現在の工程を完了扱いにし、次の工程へ進めます。

成果物や承認が不足している場合は、完了扱いにしません。

最後の工程で `devflow done` を実行した場合、Flowは完了状態になります。

### `devflow approve [--step <step>] [--note <note>]`

承認が必要な工程に対して、人間の承認を記録します。

現在の工程を承認できます。
必要に応じて、対象工程と任意メモを指定できます。

例:

```bash
devflow approve
devflow approve --step human_approval
devflow approve --note "確認済み。次へ進めてよい。"
devflow approve --step human_approval --note "レビュー結果を確認済み。"
```

承認メモは任意です。

### `devflow back --reason <reason>`

前の工程へ戻ります。

戻る理由を記録します。

MVPでは、戻り操作の詳細な状態遷移は設計書で定義します。

### `devflow skip --reason <reason>`

現在の工程をスキップし、次の工程へ進みます。

スキップには理由を記録します。

スキップは、人間が明示した場合だけ実行する運用とします。
ただし、devflow本体では人間判定を強制しません。

### `devflow finish --reason <reason>`

現在のFlowを終了します。

終了には理由を記録します。

## 完了条件

MVPは、次の状態になったら完了とします。

* Flow定義を読み込める
* 利用可能なFlowを `devflow list` で確認できる
* 現在地を `.devflow/state.json` に保存できる
* `.devflow/state.json` をGit管理対象外にできる
* `devflow status` で現在のFlow、現在の工程、完了済み工程、承認状態を確認できる
* `devflow prompt` で現在の工程の指示を表示できる
* `devflow done` で次の工程へ進める
* 最後の工程で `devflow done` した場合、Flowを完了状態にできる
* 成果物がない場合に完了を止められる
* 不足している成果物を表示できる
* 承認がない場合に完了を止められる
* 不足している承認を表示できる
* `devflow approve` で承認とメモを記録できる
* `devflow back` で理由付きで前の工程へ戻れる
* `devflow skip` で理由付きで工程をスキップできる
* `devflow finish` で理由付きでFlowを終了できる
* activeなFlowが存在する状態で `devflow start` した場合にエラーを出せる
* stateが壊れている場合にエラーを表示できる
* stateが存在しないFlowや工程を参照している場合にエラーを表示できる
* artifact path をプロジェクトルート相対パスとして解決できる
* 不正なartifact pathをエラーにできる
* `post-task-review` を公開サンプルFlowとして実行できる
* `purpose-driven-development` を公開サンプルではなく、開発者自身のdogfooding用Flowとして実行確認できる

## 用語

### Flow

開発作業の流れを定義したものです。

人間が毎回チャットや設定ファイルで伝えていた進行指示を、devflow では Flow として扱います。

### 工程（step）

Flowの中の1つの区切りです。

AIは、現在の工程に応じて作業します。
設定ファイル上では `step` として扱います。

### 通過条件（gate）

次の工程へ進むための条件です。

MVPでは、主に成果物の存在と承認の有無を扱います。
設定ファイル上では `gate` として扱います。

### 成果物（artifact）

工程の中で作られるファイルや記録です。

要件定義書、設計書、レビュー結果、質問と回答、テスト結果などが含まれます。
設定ファイル上では `artifact` として扱います。

### 承認（approval）

人間が次へ進んでよいと判断した記録です。

設定ファイルや状態管理では `approval` として扱います。

### 現在地（state）

今どの Flow のどの工程にいるかを表す情報です。

devflow は、現在地をAIの記憶ではなく、ツール側で保持します。
状態ファイル上では `state` として扱います。

## 成果物

MVPで想定する成果物は、主にファイルとして扱います。

例:

* `docs/purpose.md`
* `docs/requirements.md`
* `docs/requirements-review.md`
* `docs/questions.md`
* `docs/research-notes.md`
* `docs/design.md`
* `docs/design-review.md`
* `docs/tasks.md`
* `docs/test-results.md`
* `docs/code-review.md`
* `docs/final-review.md`
* `docs/commit-message.md`
* `docs/rule-candidates.md`
* `docs/flow-improvements.md`

成果物ファイルの名前は英語を基本にします。
ファイルの中身は日本語で書けます。

devflow は、MVPでは成果物の中身を判定せず、存在確認を中心に扱います。

## 承認条件

devflow は、人間の承認が必要な工程で承認を記録します。

承認が必要な工程の例は次の通りです。

* 要件定義の承認
* 設計の承認
* 重要な仕様変更の承認
* コミット前承認
* Flow定義の採用承認
* ステアリング由来Flowの移行承認

承認は、工程IDに対して記録します。

例:

```json
{
  "approvals": {
    "approve_requirements": {
      "approved": true,
      "note": "設計へ進めてよい"
    },
    "approve_design": {
      "approved": true,
      "note": "この設計でタスク化してよい"
    }
  }
}
```

承認が必要な工程では、承認が記録されるまで `devflow done` による完了扱いを行いません。

## まとめ

devflow は、人間が毎回AIに伝えていた進行指示を Flow として扱うためのツールです。

AIは、devflow に現在の工程を確認しながら、各工程の中で調査・設計・実装・レビューを行います。
devflow は、現在地、工程、通過条件、成果物、承認を管理します。

MVPでは、人間の繰り返し進行指示を機械的に扱うことに集中します。
