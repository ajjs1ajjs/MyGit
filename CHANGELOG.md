# Changelog

## [3.0.8] - 2026-08-12

### 🔴 Виправлено (безпека)

- **Path traversal через username (CRITICAL)** — реєстрація приймала будь-який username, включно з `..`, `/`, `\\`. Через `filepath.Join` це дозволяло створювати та видаляти каталоги за межами `repos/` (sandbox escape). Тепер username валідується (тільки `[a-zA-Z0-9._-]`, перший символ — літера або цифра), а owner перевіряється на місцях створення/fork/видалення репозиторію.
- **Webhook secrets** — endpoint `GET /projects/{id}/hooks/` повертав signing secret будь-якому читачеві репозиторію. Тепер secret маскується (`has_secret`).
- **Rate-limit/auth hardening** — внутрішній токен порівнюється константним часом.
- **HSTS + Permissions-Policy** додані до security headers (при TLS).
- **Race при bootstrap superuser** — два одночасні перші реєстрації могли обидві стати superuser. Тепер count+insert виконуються під `BEGIN IMMEDIATE`.

### 🟠 Виправлено (bugs / API)

- **Blob по `ref:path` був зламаний** — `GET /projects/{id}/blobs/{sha}/` ігнорував `ref`/`path` і завжди робив `git cat-file blob <sha>`; frontend шле `sha=0` як sentinel, тож перегляд README і файлів повертав 404. Тепер `sha=0` резолвиться у `ref:path`.
- **Commits** — `author` тепер обʼєкт `{name, email}`, додано `committed_at` (RFC3339) і `parents` у деталях коміта; `git log --parents` для списку.
- **Commit diff** — повертає розібраний `{diffs: [{type, old_path, new_path, diff}]}` замість сирого тексту.
- **Blame** — повертає структуровані `{lines: [{sha, author, author_email, committed_at, line_number, line}]}` (парсинг `--line-porcelain`).
- **Branches/Tags** — повертають `{name, sha}` замість голих рядків.
- **Tree entries** — додано поле `name`.
- **MR comments / MR diff / Wiki CRUD** — реалізовано відсутні ендпоінти (схема БД вже існувала).
- **Authorization** — створення/видалення гілок тепер вимагає роль developer (30), не лише read (10).
- **Обмеження пагінації** коммітів (макс. 500).
- **Docker HEALTHCHECK** — тепер перевіряє `/api/v1/health` замість auth-захищеного `/api/v1/projects/` (healthcheck завжди падав з 401).

### 🟢 Покращення

- Видалено мертвий код (localStorage-авторизація у frontend router, `_ = repoPath`, `_ = context.Background`).
- Frontend: XSS-стійкий fallback підсвітки, raw-посилання вказують на API, дата комітів і clone URL використовують правильні поля/протокол.

## [3.0.7] - 2026-08-12

### Змінено

- fix: auto-select a free port when the requested one is busy
- chore: default MyGit to port 8060
- fix: add `/api/v1/health` and make install.sh verify MyGit is actually up
- fix: return JSON 404 for unmatched `/api/*` instead of the SPA HTML
- fix: support `--version` so install.sh validates the downloaded binary
- fix(merge): set a committer identity in the scratch merge clone

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
