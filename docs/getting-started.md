# Getting Started: Coreで`post-task-review`を完走する

このチュートリアルでは、devflowのCoreだけを使い、標準Flowの`post-task-review`を1回最後まで進めます。Automation Runtime、Executor、Check Adapterは使いません。

devflowはAIを呼び出したり作業内容を決めたりしません。Flowが「何を成立させるか」を工程契約として管理し、人間またはAIがその工程の作業を行います。`status`は現在地、`prompt`は現在の工程契約を示します。`done`が確認するのはFlowで宣言されたGateであり、Objectiveの実施内容そのものではありません。

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

`list`に`post-task-review`が表示され、開始直後のStepは`check_changes`です。`prompt`に表示される`Objective`が、今のStepで成立させることです。Flowの次のStepを自分で進めず、現在のStepを完了可能な状態にしてから`done`を実行します。

`status`と`prompt`には`Current attempt:`も表示されます。Attemptは、同じStepに入り直した場合でも記録を混同しないためのIDです。Artifact Evidenceの記録とApprovalでは、表示された現在のAttempt IDを使います。

## 4. Artifactがない3つのStepを完了する

このチュートリアルでは実際の実装変更は行わず、Coreのライフサイクルを確認します。最初の3つのStepは確認結果を整理する工程です。実際の利用では、それぞれの`prompt`に従って変更ファイル、変更内容、品質確認を行い、その結果を人間またはAIが判断してから`done`を実行します。ここではサンプルなので、各工程を確認したものとして順に完了します。

```bash
/tmp/devflow done
/tmp/devflow prompt

/tmp/devflow done
/tmp/devflow prompt

/tmp/devflow done
/tmp/devflow prompt
```

最後の`prompt`のStepは`write_review`です。ここでは`docs/code-review.md`がRequired artifactとして表示されます。

## 5. レビュー結果を作成し、Artifact Evidenceを記録する

まず、Flowが求めるファイルを作成します。

```bash
printf '%s\n' '# Code review' '' '- サンプル変更を確認した。' '' '## 結論' '' '問題なし。' > docs/code-review.md
```

次に`status`または`prompt`の`Current attempt:`の値を確認し、下の`<write-review-attempt-id>`をその値で置き換えます。`artifact record`はファイルのdigestとsizeをArtifact Evidenceとして現在のAttemptに記録します。

```bash
/tmp/devflow status
/tmp/devflow artifact record \
  --step write_review \
  --attempt <write-review-attempt-id> \
  --path docs/code-review.md
/tmp/devflow status
/tmp/devflow done
```

記録後の`status`では、`docs/code-review.md`が`current`になります。ファイルを変更した場合は、`done`の前にもう一度`artifact record`を実行してEvidenceを更新します。

## 6. 人間が承認してFlowを完了する

`done`の後、現在のStepは`human_approval`になります。`prompt`でレビュー結果を確認し、人間が承認判断を行います。

```bash
/tmp/devflow prompt
```

表示された`Current attempt:`の値を`<approval-attempt-id>`に入れて承認し、最後のStepを完了します。

```bash
/tmp/devflow approve \
  --step human_approval \
  --attempt <approval-attempt-id> \
  --note "レビュー結果を確認し、承認する"
/tmp/devflow done
```

最後の`done`には次が表示されます。

```text
Flow completed: post-task-review
```

これで、Stepごとに新しいAttemptが作られ、Artifact Evidenceは`write_review`のAttemptに、Approvalは`human_approval`のAttemptに記録される流れを完走しました。

## 次に学ぶこと

外部Executorに作業を一回実行させ、ArtifactやCheckを記録する必要が出たら、READMEのAutomation Runtimeを参照してください。Automation RuntimeもApproval、`done`、次のStepへの遷移を自動では行いません。
