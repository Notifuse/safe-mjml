# Safety

safe-mjml renders templates you do not control. Each protection below has a test.

| Protection | Test |
|---|---|
| A panic is returned as an `*mjml.Error` (cause `mjml.ErrInternal`, stack in `Error.Stack`), never raised | Fuzzing of `Render` on every pull request; `mjml/internal_error_test.go` |
| Elements nest at most 1,000 deep; an inline `mj-style` at most 100 deep | `mjml/limits_test.go` |
| Rendering time grows linearly with the template | `TestRenderScalesLinearly`, 44 families of templates |
| Inline `mj-style` stops after 250 million steps of work, 1 to 2 s | `TestHostileInlineStylesAreRefused`, `TestInlineStyleMatchingIsBounded` |
| The HTML is limited to 16 MB, and memory to about 100 MB per render | `TestOutputIsBounded`, `TestCopiesArePaidBeforeTheyAreMade` |
| Attribute values are escaped wherever they are written | The escaping suite, 1,690 documents |
| Malformed templates are refused with their line; no content is dropped silently | The author-mistakes suite, 516 templates; the visible-text invariant on every document |
| Invalid characters and invalid UTF-8 are refused with their line | `TestInvalidCharactersAreRefused` |
| No file or network access: `mj-include` is not supported, and no package imports `os`, `net` or `os/exec` | `go list -deps ./mjml` |
| No code execution: no JavaScript engine, no cgo, no subprocess; template tags are kept as text | The dependencies: the standard library and `golang.org/x/net` |
| Renders are safe to run concurrently | Every test runs under the race detector |

## What safe-mjml does not do

- It does not sanitize the HTML inside `mj-text`, `mj-raw` and the other content tags: like MJML, it writes it as is. Sanitize it yourself if your template authors should not add arbitrary HTML.
- It does not render template tags. Your template engine runs before or after safe-mjml, and escapes the data it inserts.
- It does not limit the size of the template, and a render cannot be cancelled. Limit the size of the templates you accept, and the number of renders you run at once.
