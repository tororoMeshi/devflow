# devflow

devflowは、AI支援開発で固定すると安定する工程契約と進行状態を管理するCLIです。
Coreが工程と状態を管理し、Automation Runtime（`devflow-runner`）がその契約を使って外部Executorを一回実行します。**固定はdevflow、流動は外部ExecutorやAI。**

CoreはFlow、State、Attempt、Gate、Artifact Evidence、CheckResult、Approval、lifecycle transitionを管理します。Automation RuntimeはCore CLIの呼出し、Executorの実行、Artifactの記録、Check Adapterの実行、Completion Contextの取得、runtime resultの出力を担当します。

## Build

```bash
go build -o /tmp/devflow ./cmd/devflow
go build -o /tmp/devflow-runner ./cmd/devflow-runner
```

以降の例では、この2つのバイナリを使用します。

## Core Quick Start

`init`で標準Flowを配置し、`list`で利用可能なFlowを確認します。`start`にはタスク内容を保存したファイルを指定します。

```bash
mkdir -p docs
printf '%s\n' '実装対象と完了条件を書く' > docs/task-request.md

/tmp/devflow init
/tmp/devflow list
/tmp/devflow start post-task-review --task-file docs/task-request.md
/tmp/devflow status
/tmp/devflow prompt
```

`prompt`の工程契約に従って作業し、必要な成果物・Check・Approvalを満たしたうえで完了します。

`--task-file`はプロジェクトルートからの相対パスです。絶対パス、および`..`を含むパスは受理されません。

```bash
/tmp/devflow approve --step <step-id> --attempt <attempt-id> --note "確認済み"
/tmp/devflow done
```

現在の工程を戻す、スキップする、Flowを終了する操作には理由が必要です。

```bash
/tmp/devflow back --reason "追加調査が必要"
/tmp/devflow back --to <step-id> --reason "この工程からやり直す"
/tmp/devflow skip --reason "今回のタスクでは不要"
/tmp/devflow finish --reason "対象外と判断"
```

`status`は進行状況、`prompt`は現在工程の指示を表示します。`context`は外部Executor向けの読み取り専用の現在文脈JSONを出力します。

## Automation Runtime

`devflow-runner execute`は、指定したStepとAttemptに対して次の順で処理します。

```text
WorkPackage取得
→ Executor実行
→ ExecutionReport記録
→ ArtifactEvidence記録
→ RequiredChecks実行・記録
→ Completion Context取得
→ 停止
```

`devflow-runner`がExecutionReportを記録できたことと、Completion Gateを通過できることは別です。外側の運用が工程の`done`を検討できるのは、少なくとも次の両方を満たす場合です。

```text
report_outcome == "completed"
かつ
completion_context.completion.status == "ready"
```

Executorが`blocked`または`failed`を返した場合は、Completion Contextが形式上readyに見える工程であっても、`done`へ進める判断をしてはいけません。

後半のArtifact、Check、Completion Contextは対応するoptionを指定した場合だけ実行します。ExecutorにはWorkPackageがstdinで渡され、ExecutorはExecutionReport JSONをstdoutへ返します。

### Reportのみ

```bash
/tmp/devflow-runner execute \
  --step <step-id> \
  --attempt <attempt-id> \
  -- <executor> [executor-args...]
```

### Artifact記録

```bash
/tmp/devflow-runner execute \
  --step <step-id> \
  --attempt <attempt-id> \
  --record-artifacts \
  -- <executor> [executor-args...]
```

### Check実行

```bash
/tmp/devflow-runner execute \
  --step <step-id> \
  --attempt <attempt-id> \
  --record-artifacts \
  --check-adapter <check-adapter> \
  -- <executor> [executor-args...]
```

Check AdapterはCoreから渡されるCheckRequestをstdinで受け取り、CheckRecord JSONをstdoutへ返します。必要なら`--check-adapter-arg <argument>`を複数指定できます。

### Completion Contextまで

```bash
/tmp/devflow-runner execute \
  --project-root . \
  --devflow /tmp/devflow \
  --step <step-id> \
  --attempt <attempt-id> \
  --record-artifacts \
  --check-adapter <check-adapter> \
  --completion-context \
  -- <executor> [executor-args...]
```

## option間の制約

- `--check-adapter`には`--record-artifacts`が必要です。
- `--completion-context`にはArtifact modeとCheck mode、すなわち`--record-artifacts`と`--check-adapter`が必要です。
- Executorの制限時間は`--timeout <duration>`、終了待機時間は`--terminate-grace <duration>`で指定します。
- Check Adapterの制限時間は`--check-timeout <duration>`、終了待機時間は`--check-terminate-grace <duration>`で指定します。
- Executor commandとその引数は必ず`--`より後ろに置きます。

`--project-root`の既定値は`.`、`--devflow`の既定値は`devflow`です。timeoutを省略すると制限時間は設定されません。終了待機時間の既定値はExecutor、Check Adapterともに5秒です。

## WorkPackageとExecutionReport

WorkPackageはcurrent/active Attemptに束縛されたExecutor用の入力です。stdinでExecutorへ渡され、task、objective、inputs、artifacts、checks、Approvalの要否を含みます。

ExecutionReportはExecutorがstdoutへ返すJSONです。WorkPackageとAttemptに束縛され、Coreがimmutableに記録します。同一内容の再記録はidempotentですが、異なる内容を同じAttemptへ記録しようとすると競合として拒否されます。

手動でExecutionReportを記録する場合は次を使います。

```bash
/tmp/devflow execution-report record --file <report.json>
```

## Artifact Evidence

Artifact Evidenceとして記録できるのは、Flowで宣言したArtifactだけです。requiredとoptionalのどちらも記録でき、Coreはファイルのdigestとsizeを保存します。未宣言Artifactは拒否されます。

一度保存された記録はrollbackしません。同じ記録の再実行は収束し、途中で停止した場合も未記録のArtifactから続行できます。

手動での記録には次を使います。

```bash
/tmp/devflow artifact record \
  --step <step-id> \
  --attempt <attempt-id> \
  --path <project-relative-path>
```

## RequiredChecks

RequiredChecksはcurrent/active Attemptに束縛されます。CoreがCheckRequestを生成し、AdapterがCheckRecordを返します。Automation RuntimeはFlowの定義順にRequiredChecksを処理します。

Check自体の非ゼロexit codeはCheckRecordに保存されるドメイン結果です。これに対し、Adapter processの起動失敗・timeout・非ゼロ終了、またはJSON protocolの不正はRuntimeの実行失敗として区別されます。記録済みのCheckはAdapterを再実行しません。

手動操作も可能です。

```bash
/tmp/devflow check request \
  --step <step-id> \
  --attempt <attempt-id> \
  --check <check-id>

/tmp/devflow check record --file <result.json>
```

## Completion Context

Completion Contextは、指定したAttemptの完了判断に必要な読み取り専用JSONです。

```bash
/tmp/devflow completion-context \
  --step <step-id> \
  --attempt <attempt-id>
```

Artifact状態、Check状態、Approval状態、Completion Gate、blockerを返します。Stateは変更しません。RuntimeはこのGateを再評価して遷移せず、Completion Contextを返して停止します。

Completion ContextはExecutorの成功を単独で判断する結果ではありません。Runtime resultには、Executorの`ExecutionReport`に基づく`report_outcome`と、要求した場合の`completion_context`が含まれます。両者を確認して完了を判断してください。

標準Flowの`check_quality`は、テスト、lint、型チェックなどを確認するようExecutorへ指示する工程です。標準Flowには`required_checks`が定義されていないため、これは作業指示であり、RequiredChecksによる機械的な強制ではありません。テスト結果をCompletion Gateで必須にする場合は、プロジェクト固有Flowへ`required_checks`を定義してください。

## 再実行と部分適用

Runtimeは同一Attemptへの再実行を前提に、保存済みの事実を壊さず処理します。

- Reportは同一内容へ収束します。
- Artifact Evidenceは同一記録へ収束します。
- 記録済みCheckはAdapterを再実行しません。
- 停止後は未記録の処理から継続します。
- 保存済み記録をrollbackしません。
- current/activeではないstale AttemptはCoreが拒否します。

## 人間判断境界

Automation Runtimeは作業を実行・記録しますが、判断や進行を自動化しません。Approval判断、`approve`、`done`、次Attemptの作成、次Stepへの遷移、retry判断は実行しません。Completion Contextを返して停止し、その後の判断は人間または外部の運用に委ねます。

## Files

- `.devflow/flows/`: Flow定義（CUE）の保存場所です。
- `.devflow/current.json`: 現在のFlow runを指すpointerです。runごとのStateとsnapshotは`.devflow/runs/`に保存されます。
- `.devflow/logs/`: CoreやRuntimeの固定保存先ではありません。CheckRecordの`log_path`はAdapterが返すプロジェクトルート相対のパスであり、ログの生成・配置はAdapter側の責務です。

ExecutionReportは`.devflow/runs/<flow-run-id>/execution-reports/<attempt-id>.json`にimmutableに保存されます。

## Current Scope

現在のCore commandsは`init`、`list`、`start`、`status`、`prompt`、`context`、`completion-context`、`work-package`、`approve`、`artifact record`、`check request`、`check record`、`execution-report record`、`done`、`back`、`skip`、`finish`です。Automation Runtimeは`devflow-runner execute`を提供します。

次は対象外です。

- AI API呼出し
- MCP
- 自動Approval
- 自動done
- retry policy
- 複数Attemptの自律進行
- daemon
- queue
- remote executor
- model selection
- conversation／hidden reasoning管理
- Windowsネイティブ対応

## State compatibility

現在のState schema versionは8です。対応しないschema versionのState、schema versionを持たない旧State、旧来のState配置は利用できません。devflowはこれらを自動migration・削除しません。

更新前に作業中のStateを確認し、必要なら退避してください。移行手順を提供していないため、旧Stateを使えない場合は新しいFlow runを開始してください。
