# Project Notes

## Build

- `make build` — Build dashboard + Go binary to `output/`
- `make test` — Run all tests
- `make check` — Quick local check (fmt + vet + build)

## Pre-commit

- Run `prek run --all-files` before commits to execute all pre-commit hooks (formatting, linting, etc.)

## Conventions

- Error messages must include context (relevant values) and be descriptive enough to diagnose without source code