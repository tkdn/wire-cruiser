# 実装計画: wire-cruiser v0.0.1

要件・スコープは [2026-07-16-init.md](./2026-07-16-init.md) で確定済み。本計画はその実装詳細(ディレクトリ構成・コアモデル・実装順序)を定める。

init 計画の出力形式のうち **mermaid レンダラは本計画のスコープ外**とし、次のステップの計画で扱う。本計画は tree 出力までを対象とする。

# 前提(検証済み事実)

- Go は最新の **1.26.x** で開始する(`go` directive は `go 1.26`)。2026-08 に Go 1.27 のリリースが予定されているため、リリース後に toolchain と x/tools の追従を確認する。
- google/wire は 2025-08-25 にアーカイブ済み(最終リリース v0.7.0)。後継フォーク [goforj/wire](https://github.com/goforj/wire) も解析ロジックは internal に閉じており再利用不可。**解析は自前実装で確定**。DSL 仕様は凍結されており追従コストはない。
- ジェネリクスは wire 本体が未対応のまま終了しているため、本ツールも非対応。型パラメータ付きの構成に遭遇した場合は警告扱いとする。

# ディレクトリ構成

module path: `github.com/tkdn/wire-cruiser`

```
.
├── go.mod
├── cmd/wire-cruiser/
│   ├── main.go        # flag 解析と exit code 制御のみの薄い層
│   ├── wire.go        # ドッグフーディング: 内部組み立て(Phase 5 で追加)
│   └── wire_gen.go
├── internal/
│   ├── loader/        # パス解釈 + packages.Load(wireinject タグ付き)+ injector 検出
│   ├── graph/         # コアモデルと依存解決アルゴリズム
│   └── render/        # tree レンダラ(mermaid は次ステップ)
└── testdata/          # ケースごとの独立 Go モジュール(後述)
```

- `internal/graph` の語彙は wire 本体に合わせる(Injector / Provider / ProviderSet / Binding)。独自の用語拡張はしない。
- レンダラは `Graph → string` の純関数として実装し、出力形式の追加(mermaid / JSON / DOT)を graph 本体に触れず行えるようにする。

# コアモデル(internal/graph)

```go
type Injector struct {
    Name   string       // 例: InitializeApp
    Pkg    string       // 定義パッケージのパス
    Args   []types.Type // injector 関数の引数(グラフの葉になる)
    Result types.Type   // 第 1 戻り値(グラフの根)
    Roots  []*Node
    Diags  []Diagnostic
}

type Node struct {
    Kind     NodeKind   // FuncProvider | StructProvider | Value | InterfaceValue | Arg | Missing
    Provider string     // 例: service.NewUserService(FuncProvider の場合)
    Type     types.Type // 提供型
    Bind     types.Type // wire.Bind 経由の場合の実装型(なければ nil)
    Deps     []*Node
}

type Diagnostic struct {
    Kind DiagKind // MissingProvider | DuplicateProvider | Unsupported(ジェネリクス等)
    Type types.Type
    Pos  token.Position
}
```

- 同一型のノードはグラフ内で共有する(実体は DAG)。ツリー表示での重複展開の抑制はレンダラ側の責務とする。
- Diagnostic が 1 件でもあれば exit code 1(グラフ出力自体は行う)。

# 依存解決アルゴリズム

1. **パス解釈**: 引数が `.go` で終わる場合は所属ディレクトリをパッケージとしてロードし、対象ファイル内の injector に限定する。それ以外はパッケージパスとして扱う。
2. **ロード**: `packages.Load` に `BuildFlags: []string{"-tags=wireinject"}` を渡す。Mode は `NeedName | NeedFiles | NeedSyntax | NeedTypes | NeedTypesInfo | NeedDeps`(NewSet の再帰展開に依存パッケージの構文木・型情報が必要なため)。
3. **injector 検出**: wireinject タグ付きファイル内で `wire.Build` を呼ぶ関数宣言を対象とする。wire の判定は import パス `github.com/google/wire` で行う(名前だけの一致では誤検出するため)。`panic(wire.Build(...))` 形式・cleanup / error 戻り値付きのシグネチャも第 1 戻り値を根として扱う。1 ファイル内に injector が複数ある場合(本体用とテスト用の initializer を並べる構成など)もすべて検出対象とする。
4. **wire.Build 引数の展開**(再帰):
   - 関数参照 → FuncProvider(第 1 戻り値が提供型、引数が依存)
   - ProviderSet 変数参照 → `types.Info.Uses` で定義元を特定し、初期化式 `wire.NewSet(...)` を定義元パッケージの AST から取得して再帰展開(別パッケージ可)
   - `wire.Bind` / `wire.Struct` / `wire.FieldsOf` / `wire.Value` / `wire.InterfaceValue` → 対応するノード・バインディングに変換
5. **グラフ構築**: 展開結果から「提供型 → プロバイダ」の索引を作り、injector の戻り値型から DFS で Deps を解決する。injector 引数の型は Arg ノード、索引にない型は Missing ノード、同一型の重複提供は**両方保持**して DuplicateProvider を記録する。

# CLI 仕様

```
wire-cruiser <path>
```

- 標準 `flag` パッケージを使う(サブコマンドの予定なし)。
- `--format` フラグは mermaid レンダラ導入(次ステップ)と同時に追加する。本計画の出力は tree のみ。
- グラフは stdout、警告・エラーは stderr に出力する。
- injector が複数見つかった場合はすべて順に出力する。
- exit code: **0** = 正常 / **1** = グラフ不完全(Missing・Duplicate・Unsupported あり) / **2** = 引数不正・ロード失敗。

# レンダラ仕様(tree)

init 計画のプレビュー形式を正とする。

```
app.InitializeApp → *app.App
├─ app.NewApp → *app.App
│  ├─ service.NewUserService → *service.UserService
│  │  └─ repo.NewUserRepo → repo.UserRepo (bind: *repo.PgUserRepo)
│  │     └─ db.NewDB → *sql.DB
│  └─ config.Load → *config.Config
```

- injector 引数: `(arg) *config.Config`
- 解決不能: `[MISSING] repo.UserRepo`
- 同一ノードの 2 回目以降の出現は子の展開を省略し、末尾に ` (…)` を付す。
  - 想定場面: `*sql.DB` や `*config.Config` のような**共有依存**を複数のプロバイダが要求するケース。素朴に木へ展開すると同じサブツリーが出現回数分繰り返され、大きなグラフでは出力が膨張するため、2 回目以降は 1 行に畳む。

# テスト戦略(ゴールデンテスト先行)

- `testdata/<case>/` は各自 `go.mod` を持つ独立モジュールとする。go ツールチェーンは testdata ディレクトリを無視するため親モジュールと干渉せず、テストからは `packages.Config.Dir` で対象モジュールを指してロードする。
- testdata モジュールは実物の `github.com/google/wire v0.7.0` に依存する(`go.sum` をコミット)。モジュールキャッシュ解決なので通常はオフラインでも動くが、完全ヘルメチックな CI が必要になった時点で local stub + replace 方式に切り替える。
- ケース一覧(Phase 2 以降の受け入れ基準を兼ねる):

  | # | case | 検証対象 |
  |---|------|---------|
  | 1 | basic | 関数プロバイダのみの最小構成 |
  | 2 | newset_local / newset_crosspkg | NewSet の同一・別パッケージ再帰展開 |
  | 3 | bind | wire.Bind |
  | 4 | struct_fieldsof | wire.Struct / wire.FieldsOf |
  | 5 | value_interfacevalue | wire.Value / wire.InterfaceValue |
  | 6 | args | injector 引数の葉ノード化 |
  | 7 | missing | 解決不能型の MISSING 表示 + exit 1 |
  | 8 | duplicate | 同一型の重複提供(両方表示)+ exit 1 |
  | 9 | multi_injector | 同一ファイル・同一パッケージ内の複数 injector の全件出力 |

- 各ケースに `want_tree.txt` のゴールデンファイルを置き、テストの `-update` フラグで再生成する。
- CLI レベルは `os/exec` で main を叩く最小限の E2E(exit code と stdout/stderr の分離を検証)。

# 実装順序

各フェーズを「テストを書き失敗を確認 → 実装して成功させる → コミット」の順で進める。`go test` / `golangci-lint` をフェーズごとに実行する。

1. **Phase 1 — 足場と loader**: go.mod 初期化、golangci-lint 最小設定、`internal/loader`(パス解釈・Load・injector 検出)。テスト: basic ケースで injector が 1 件検出される。
2. **Phase 2 — graph**: NewSet 再帰・DSL 変換・missing / duplicate。テスト: ケース 1〜9 の Node 構造をレンダラを介さず直接アサートする。
3. **Phase 3 — tree レンダラ**: ゴールデンテスト(want_tree.txt)。
4. **Phase 4 — CLI 組み立て**: flag・exit code・複数 injector 出力・E2E。
5. **Phase 5 — ドッグフーディング**: cmd/wire-cruiser の組み立てを wire 化し、`wire-cruiser ./cmd/wire-cruiser` の出力を README に掲載する。

# レビューで確定した判断(round 1)

1. mermaid レンダラは次ステップへスコープアウトする。
2. exit code 体系(0 / 1 / 2)。
3. testdata は実物の wire モジュールに依存する(完全なヘルメチック性より単純さを優先)。
4. golangci-lint は Phase 1 から導入する。

# レビューで確認したい判断

1. tree での重複サブツリーの省略方針(` (…)` 表記)。想定場面の説明はレンダラ仕様に追記済み。
