# Contributing

Thanks for helping. This is a maintained fork of teambition/rrule-go.

## Reporting a problem

Open an issue using one of the forms. For a bug, give the exact rule and DTSTART, what you expected
and what you got. If you can, say what python-dateutil returns for the same rule, since this library
aims to match it.

Security problems (panics, hangs, runaway resource use) go through private reporting instead. See
[SECURITY.md](SECURITY.md).

## Changing the code

- Keep the module path (`github.com/teambition/rrule-go`), the public API and the Go 1.16 minimum.
  Changes that would break existing callers need an issue first.
- Add a test that fails before your change and passes after it.
- Before opening a pull request, run:

  ```bash
  go vet ./...
  go test -race ./...
  ```

- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages, for example
  `fix: stop BYSETPOS from skipping the last period`.
- Open pull requests against the `release` branch.

By contributing you agree that your work is released under the MIT license in [LICENSE](LICENSE).
