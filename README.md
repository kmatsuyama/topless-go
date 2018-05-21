# topless-go

コマンドのアウトプットを `top` 風に表示するコマンド
[topless](https://codezine.jp/article/detail/67)
を真似して go 言語で実装したものです。

## Usage

```bash
Usage: ./topless [-s sec] [-i] [-sh] [-f] command

  -f    ignore execute error
  -i    interactive
  -s float
        sleep second (default 1)
  -sh
        execute through the shell
```
