# onyx

The uncompromising Golang code formatter

onyx formats Go source deterministically: the same program always produces the
same output, whatever the input layout looks like, with very few knobs. Its
output is stable under `gofmt` and idempotent.

## Usage

```shell
onyx [flags] [path ...]
```

Paths are files or directories (walked recursively, skipping `vendor/`,
hidden directories and generated files). `-` reads standard input and writes
the result to standard output.

| Flag                           | Description                                                  |
| ------------------------------ | ------------------------------------------------------------ |
| `--check`                      | Do not write; exit with status 1 if any file would change    |
| `--diff`                       | Print a unified diff instead of writing                      |
| `--fast`                       | Skip the safety checks below                                 |
| `--config path`                | Use this configuration file instead of discovering one       |
| `--line-length n`              | Maximum line length (default 100)                            |
| `--tab-width n`                | Width of a tab when measuring lines (default 4)              |
| `--magic-trailing-comma=false` | Do not treat a trailing comma as a request to keep exploding |

Exit statuses: 0 success, 1 changes needed (with `--check`), 2 error.

### Configuration

Settings are resolved in this order, later ones winning: defaults, the
user-level `onyx/config.toml` in the user config directory, the nearest
`.onyx.toml` found by searching upward from the file (which replaces the
user-level file), then the command line flags.

```toml
line-length = 100
tab-width = 4
magic-trailing-comma = true
```

### Safety checks

Unless `--fast` is given, every formatted file is parsed again and verified:

1. its syntax tree must equal the original one, ignoring positions, redundant
   parentheses, empty statements and number literal casing, and its comments
   must be unchanged;
2. formatting the output again must change nothing.

When a check fails the file is left untouched and onyx reports a bug.
Constructs onyx cannot reproduce faithfully, such as a comment in the middle
of an expression, are rejected with an "unsupported construct" error rather
than formatted incorrectly.

## Style

- Lines are broken at the line length. Calls, signatures and composite
  literals try, in order: everything on one line, one item per line (or all
  items on one indented line when the magic trailing comma is disabled), and
  hugging a last function literal or composite literal argument.
- A trailing comma after the last item keeps the collection exploded one item
  per line.
- Binary expressions break after the operator; method chains break after the
  dot.
- Struct and interface types with members are always multi-line.
- One blank line separates top-level declarations (but not consecutive
  imports); more blank lines collapse to one.
- A blank line precedes `return` unless it is the first statement of a block,
  and follows a statement that ends with a multi-line block.
- Control-clause parentheses and redundant nested parentheses are removed.
- Number literals are normalized like `gofmt` does.

### Known limitations

- Trailing comments on consecutive lines of a broken list are not always
  aligned like `gofmt` would (about 1% of the standard library).
- Comments inside an expression are not supported.

## Testing

Besides unit and golden tests, the suite checks the standard library as a
corpus (stability under `gofmt`, idempotency, equivalence, under several
settings) and ships a fuzz target:

```shell
go test ./internal/safety -run xxx -fuzz FuzzSource -fuzztime 1m
```

## Development

To run the formatters and linters automatically on commit, you can use [pre-commit](https://pre-commit.com/)

```shell
pre-commit install
```

### Make commands

Makefiles are provided with the most common commands. For a complete list, run:

```shell
make help
```
