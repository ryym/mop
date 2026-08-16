# mop

Markdown ファイルをブラウザでリアルタイムプレビューする CLI ツール。

現状はプロトタイプ。

## ビルド

Markdown の解釈はブラウザ側で行うため、フロントのバンドルを先に作る必要がある。
Go だけではビルドが完結しないので **`go install` は非対応**。

```sh
make build            # bun install → bun build → go build
make VERSION=0.1.0 build
```

Bun と Go が必要。生成物は `./mop`。

## 使い方

```sh
# プレビューを開く（必要ならサーバーが自動起動する）
mop open README.md

# 表示位置をソース行で指定する
mop scroll README.md --line 42
mop scroll README.md --line 42 --viewport-ratio 0.35

# 未保存の内容をプレビューする
cat README.md | mop update README.md --line 42

# 一覧・後始末
mop list
mop close README.md
```

ポートや URL を意識する必要はない。開いているブラウザが 0 の状態が 10 分続くとサーバーは自動終了する。

サーバーを明示的に扱う場合:

```sh
mop daemon start [--port 7654] [--foreground]
mop daemon stop
```

## 構成

単一バイナリで、CLI とプレビュー用サーバー（デーモン）を兼ねる。
**デーモンは Markdown を解釈しない。** 生テキストを配信するだけで、パース・ハイライト・DOM 反映はすべてブラウザが行う。

```
cmd/mop/            エントリポイント
internal/cli/       サブコマンドの実装
internal/client/    制御 API の HTTP クライアント
internal/api/       制御 API のリクエスト・レスポンス型
internal/daemon/    HTTP サーバー、ドキュメント管理、SSE
internal/watch/     fsnotify のラッパー
internal/state/     状態ファイル、デーモンの起動・生存確認
web/src/            ブラウザ側の TypeScript と CSS
web/dist/           バンドル結果（生成物。Git 管理しない）
```

状態ファイルとログは `$XDG_STATE_HOME/mop`（既定では `~/.local/state/mop`）に置かれる。

### ブラウザ側

| 役割           | 使うもの                                  |
| -------------- | ----------------------------------------- |
| Markdown       | markdown-it（`html: false`）              |
| ハイライト     | shiki（JS RegExp エンジン、WASM 不使用）  |
| DOM 更新       | morphdom                                  |
| 図の描画       | mermaid（図がある文書でのみ遅延ロード）   |

サニタイズは markdown-it の `html: false` のみで担保している。**この設定を外す変更は、サニタイズ方針そのものの変更**として扱うこと。

## Vim / Neovim から使う

`editor/mop.vim` を source するだけ。プラグインマネージャは不要。

```vim
source /path/to/mop/editor/mop.vim
let g:mop_command = '/path/to/mop/mop'   " PATH に無い場合
```

| コマンド    | 動作                                                     |
| ----------- | -------------------------------------------------------- |
| `:Mop`      | 現在のバッファのプレビューを開き、以降の同期を開始する   |
| `:MopClose` | 同期を止め、プレビューを閉じる                           |

`:Mop` 以降は、バッファの編集が `mop update` で（**保存前の内容がそのまま**）、ウィンドウのスクロールが `mop scroll` で送られる。
バッファを閉じるか Vim を終了すると自動で `mop close` される。

**追従するのはカーソルではなくウィンドウの表示位置**（`line('w0')`）。表示範囲が変わらない限り、その中でカーソルをどれだけ動かしてもプレビューは動かない。

| 変数                   | 既定  | 意味                                                    |
| ---------------------- | ----- | ------------------------------------------------------- |
| `g:mop_command`        | `mop` | 実行するコマンド                                        |
| `g:mop_update_delay`   | 150   | 編集を送るまでのデバウンス (ms)                         |
| `g:mop_scroll_delay`   | 60    | スクロールを送るまでのデバウンス (ms)                   |
| `g:mop_viewport_ratio` | 0.0   | 画面内のどこに合わせるか。0.0 で両者の表示上端が揃う    |

## テスト

```sh
make test       # go test ./... と bun test
```

## プロトタイプでの制限

- shiki に載せている言語は javascript / rust / shell のみ。それ以外はハイライトなしのコードブロックになる
- KaTeX は未対応
- mermaid のテーマ（light/dark）はロード時に一度決まる。OS のテーマを切り替えても描画済みの図は追従しない
- Windows は未対応（デーモンのバックグラウンド起動に `setsid` を使っている）
- ブラウザ上での見た目（スクロール補間、morphdom によるパッチ）は手動確認のみ
