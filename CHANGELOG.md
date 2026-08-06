# Changelog

## [3.0.0] - 2026-08-06

### Змінено (повне переписування Python/Django -> Go)

- **Бекенд переписано на Go 1.25** — один статичний бінар, net/http + chi router, SQLite (WAL, modernc.org/sqlite).
- **Git smart HTTP**: clone/push/pull через `git upload-pack`/`git receive-pack --stateless-rpc`; Basic auth (логін/пароль або PAT).
- **API**: реєстрація/логін (JWT access+refresh), користувачі, SSH-ключі, PAT, репозиторії (CRUD, fork, by-path), дерево/blob/raw/blame/коміти/гілки/теги, issues + коментарі, merge requests, webhooks, нотатки, пошук.
- **Внутрішні ендпоінти** для git hooks (pre/post-receive) та SSH (authorized_keys, check_access).
- **Фронтенд Vue 3 + Tailwind** — збирається Vite і вбудовується через `go:embed`.
- **Права**: перший зареєстрований користувач — superuser; рольова модель owner/maintainer/developer/reporter/guest.
- **Уроки з інших портів**: `[]` замість `null`, DRF-стиль `detail`-помилок, dual-stack bind, Hijacker для upgrade, секрети не в CWD.
- **Тести**: auth, проекти (створення bare-репо), git info/refs, issues, порожні списки.
- **CLI**: `mygit -port`; змінні середовища MYGIT_*.
- **Docker multi-stage**, CI (vet + test -race + Docker smoke), release workflow (linux/arm64, windows, darwin).
