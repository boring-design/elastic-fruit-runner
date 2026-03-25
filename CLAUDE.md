# Project Notes

## Build

- Build: `make build` (output in `output/`)
- Test: `make test`
- Lint: `make lint`
- Quick local check: `make check` (fmt + vet + build)
- Full CI checks: `make ci`
- Show all targets: `make help`

## Conventions

- Error messages must include context (relevant values) and be descriptive enough to diagnose without source code
