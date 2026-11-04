# Getting Started: Coreで`post-task-review`を完走する

このチュートリアルでは、devflowのCoreだけを使い、標準Flowの`post-task-review`を1回最後まで進めます。Automation Runtime、Executor、Check Adapterは使いません。

devflowはAIを呼び出したり作業内容を決めたりしません。Flowが「何を成立させるか」を工程契約として管理し、人間またはAIがその工程の作業を行います。`status`は人間向けの現在地・不足条件・次の操作、`prompt`はAI向けの現在Step契約を示します。どちらもFlow Purpose、Step Purpose（任意）、Step Outcomeを区別して表示します。`context`、`work-package`、`completion-context`は外部運用向けのJSON契約です。`done`が確認するのはFlowで宣言されたGateであり、Step Outcomeの成立そのものではありません。

## 1. Coreをbuildする

このリポジトリのルートで実行します。

```bash
go build -o /tmp/devflow ./cmd/devflow
```

## 2. 小さな作業用プロジェクトを作る

次のコマンドで、元のリポジトリを変更しない一時プロジェクトを用意して移動します。

```bash
WORKDIR="$(mktemp -d)"
mkdir -p "$WORKDIR/docs"
cd "$WORKDIR"
printf '%s\n' 'このチュートリアルでレビューFlowの進め方を確認し、結果を docs/code-review.md に残す。' > docs/task-request.md
```

`docs/task-request.md`がFlowに渡すタスクです。`--task-file`には、これから作るプロジェクトルートからの相対パスを指定します。

## 3. 標準Flowを配置して開始する

```bash
/tmp/devflow init
/tmp/devflow list
/tmp/devflow start post-task-review --task-file docs/task-request.md
/tmp/devflow status
/tmp/devflow prompt
```

`list`に`post-task-review`とそのPurposeが表示され、開始直後のStepは`describe_changes`です。`prompt`に表示される`Step Outcome`が、今のStepで成立させることです。Flowの次のStepを自分で進めず、現在のStepを完了可能な状態にしてから`done`を実行します。

この手順は、新しいプロジェクトで`init`した標準Flow v0.4を対象にしています。既存のFlowファイルは`init`で上書き・自動移行されません。

Attemptは、同じStepに入り直した場合でも記録を混同しないためのIDです。人間が直接実行するArtifact Evidenceの記録やApprovalでは、`status`に表示される具体的なコマンドを使うため、Attempt IDを手で転記する必要はありません。

## 4. 変更内容を記録し、Artifact Evidenceを残す

このチュートリアルでは実際の実装変更は行わず、Coreのライフサイクルを確認します。`describe_changes`では、変更されたファイル・パスと変更内容を`docs/code-review.md`に記録します。実際の利用では`prompt`に従って実際の変更を調べて記述してください。ここではサンプル文書を作成します。

```bash
printf '%s\n' '# Code review' '' '## 変更内容' '' '- docs/task-request.md: チュートリアル用のタスクを作成した。' > docs/code-review.md
/tmp/devflow status
```

`status`に表示された`devflow artifact record ...`のコマンドを実行します。実行ファイルを`/tmp/devflow`に読み替えてください。`artifact record`はファイルのdigestとsizeをArtifact Evidenceとして現在のAttemptに記録します。

```bash
/tmp/devflow status
/tmp/devflow done
/tmp/devflow prompt
```

記録後の`status`では、現在ファイルと記録済みEvidenceの一致を確認できます。EvidenceはAttempt内でimmutableです。同じStepで記録後に文書を変更する必要が出たら、`status`が示す`back`などの案内に従い、新しいAttemptへ入り直して記録します。

## 5. 変更影響を評価する

現在のStepは`assess_changes`です。前工程の文書をrequired Inputとして読み、同じ文書へ目的・要求との対応、影響する場面や条件、互換性への影響と維持が必要な範囲、および影響・懸念を追記・更新します。このStepでも同じパスがrequired Artifactです。

```bash
printf '%s\n' '' '## 変更影響' '' '- 目的・要求: Coreのライフサイクルを確認するためのサンプルタスク。' '- 影響する条件: この一時プロジェクト内でのチュートリアル実行時。' '- 互換性と維持範囲: 実装変更はなく、既存APIへの影響はない。' '- 懸念: 実プロジェクトでの動作は未確認。' >> docs/code-review.md
/tmp/devflow status
```

表示された、このAttemptの`devflow artifact record ...`を実行してから進めます。前工程のEvidenceは前のAttemptに保存されており、このAttemptの文書更新を妨げません。

```bash
/tmp/devflow done
/tmp/devflow prompt
```

## 6. 品質確認を検討・実施し、レビュー判断を記録する

現在のStepは`verify_quality`です。前工程の懸念だけに限定せず、変更を受け入れるために何を確認する必要があるかを独立して検討・実施します。確認結果、判断に影響する未確認・未解決事項と、それらに基づくレビュー判断を同じ文書に記録します。標準Flowには技術固有のRequired Checkを固定していません。

```bash
test -s docs/task-request.md
test -s docs/code-review.md
printf '%s\n' '' '## 品質確認とレビュー判断' '' '- 確認と結果: タスク文書とレビュー文書が存在し、空ではないことをtest -sで確認した。' '- 未確認事項: 実プロジェクトでの動作は未確認。' '- レビュー判断: この一時プロジェクトでのチュートリアルとして受け入れ可能。実利用の品質判断は対象外。' >> docs/code-review.md
/tmp/devflow status
```

表示された、このAttemptの`devflow artifact record ...`を実行してから進めます。

```bash
/tmp/devflow done
```

## 7. レビュー文書のversionを固定し、人間が承認する

現在のStepは`human_approval`です。文書をrequired Inputとして読み、人間がレビュー結果と実際の運用条件を踏まえて受け入れを判断します。同じ文書をrequired Artifactとしても宣言しているため、まずこのAttemptのEvidenceを記録して承認対象の具体的なversionを固定します。

```bash
/tmp/devflow prompt
```

`status`に表示された`devflow artifact record ...`を実行し、もう一度`status`を確認します。

```bash
/tmp/devflow status
```

受け入れる場合は、表示された`devflow approve ...`を実行してから、最後のStepを完了します。ApprovalはこのAttemptのrequired Artifact Evidence setに結び付きます。受け入れない場合は承認せず、FlowはGate未充足のままです。その後の実装修正、要件の見直し、中止などはFlow外で判断します。

```bash
/tmp/devflow done
```

最後の`done`には次が表示されます。

```text
Flow completed: post-task-review
```

これで、4 StepそれぞれのAttemptに`docs/code-review.md`のArtifact Evidenceが記録され、最後のAttemptのEvidence Set Digestに人間のApprovalが結び付く流れを完走しました。

## 次に学ぶこと

外部Executorに作業を一回実行させ、ArtifactやCheckを記録する必要が出たら、READMEのAutomation Runtimeを参照してください。Automation RuntimeもApproval、`done`、次のStepへの遷移を自動では行いません。
