# Changelog

## [3.2.0] - 2026-08-26

### ✨ Додано

- **Підтримка Windows**: релізи тепер містять `mygit-windows-amd64.exe` та
  `mygit-windows-arm64.exe` (плюс збірки macOS amd64/arm64), усі — зі статичною
  компіляцією (CGO_ENABLED=0) і покриті `checksums.txt`.
- **Однорядковий Windows-інсталятор/оновлювач** (`install.ps1`, PowerShell 5.1+):
  перевіряє наявність Git for Windows, завантажує бінарник, верифікує SHA-256
  (fail-closed), встановлює в `%ProgramFiles%\mygit` з даними в
  `%ProgramData\mygit`, реєструє auto-restart Windows-сервіс, обмежує ACL до
  SYSTEM/Administrators і автоматично підбирає вільний порт від 8060.
  Оновлення зберігає конфігурацію, БД, репозиторії та користувачів.
- **CI на windows-latest**: vet, тести та збірка тепер виконуються і на Windows.
- GitHub Actions закріплені на повні commit SHA (supply-chain безпека).

### 📝 Примітки

- На Windows git-over-HTTP(S) працює повністю; SSH-git
  (`AuthorizedKeysCommand`) доступний лише на Linux.

## [3.0.13] - 2026-08-12

### 🐞 Виправлено

- **Google Fonts блокувався CSP** — `style.css` імпортував `https://fonts.googleapis.com/...`, а CSP `style-src 'self' 'unsafe-inline'` його блокував. Зовнішній шрифт видалено (вже був системний font-stack) — застосунок тепер повністю self-contained, без зовнішніх запитів.
- **Refresh із порожнім тілом повертав 400** — SPA шле `POST /auth/refresh/` без тіла (покладається на HttpOnly refresh cookie), а `jsonDecode` на порожньому тілі повертав 400 "Invalid request body", через що сесія не оновлювалась. Тепер порожнє тіло ігнорується: з cookie → 200, без cookie → 401.
- **Колізія cookie між інстансами на одному хості** — імена cookie тепер включають порт (`mygit_access_<port>`, `mygit_refresh_<port>`), тож два MyGit на :8060 і :8061 не перебивають один одного (cookie scope — домен, не порт; у кожного інстансу свій JWT secret).
- Regression тест `TestRefreshEmptyBodyUsesCookie`.

## [3.0.12] - 2026-08-12

### 🐞 Виправлено (CRITICAL для SPA)

- **SPA assets віддавались 404** — `GET /assets/*` повертав 404 із `text/plain`, тож браузер відмовлявся застосовувати CSS/JS (`Refused to apply style... MIME type text/plain`). Причина: `http.StripPrefix("/assets/")` переписує шлях у `/xxx`, а `http.FileServer` був змонтований на `web/` замість `web/assets/` — файли шукались не там. Тепер FileServer змонтований на `web/assets/`. Сторінка `/` вантажила index.html, але весь інтерфейс був неспроможний завантажитись.
- Додано regression тест `TestSpaAssetsAreServed` — кожен `/assets/*` з index.html віддається з 200 і не-text/plain MIME.

## [3.0.11] - 2026-08-12

### 🗓 Авто-розклад беккапів

- **Вбудований планувальник** — фоновий goroutine перевіряє розклади кожні 30 с і запускає ті, чий час настав.
- **Частоти**: `daily` (щодня у `time_of_day`), `hourly` (щогодини у `:MM:SS` з `time_of_day`), `weekly` (щотижня у той самий день тижня).
- **Запобігання повторним запускам** — колонка `backup_schedules.last_run_at` бронює запуск до старту goroutine.
- **Grace window 10 хв** — якщо слот тільки що минув, він ще виконується; давно прострочений never-run розклад чекає наступного майбутнього слота.
- Новий розклад з `daily` виконується у вказаний час того ж дня (якщо час ще не настав) або наступного дня.
- Розклад з `frequency` у БД тепер `daily`/`hourly`/`weekly` (раніше тільки зберігався).

### 🚀 upload беккапів — гнучкіші URL

- `MYGIT_BACKUP_UPLOAD_URL` може містити плейсхолдер `{filename}`, закінчуватись `/` (ім'я файлу додається) або бути точним призначенням (наприклад, presigned S3 URL із query string).

## [3.0.10] - 2026-08-12

### 🧱 Техборг закрито

- **Реальне шифрування беккапів** — прапорець `encrypt` тепер справді шифрує tar.gz архів AES-256-GCM (ключ `MYGIT_BACKUP_KEY` або похідний від JWT secret); архів зберігається як `.tar.gz.enc`, plaintext видаляється.
- **Upload беккапів** — прапорець `upload` PUT-завантажує архів на `MYGIT_BACKUP_UPLOAD_URL` (S3-style signed URL або HTTP-ендпоінт). Якщо URL не налаштований — записується нотатка у job, без помилки.
- **Pruning беккапів** — `keep_local` видаляє найстаріші архіви, лишаючи останні N.
- **MR fast-forward merge** — `method=fast-forward` у `POST .../merge/` виконує `git merge --ff-only` (коли можливо); при розбіжності гілок fallback на merge-commit. За замовчуванням — `--no-ff`.
- **Rate limiter за reverse-proxy** — `MYGIT_TRUST_PROXY=1` вмикає використання `X-Forwarded-For` (перший IP) для per-IP лімітів; без прапора XFF ігнорується (не можна обійти ліміт підробкою заголовка).
- `createBackupArchive` тепер робить один `Walk` замість вкладеного.

### 🆕 Змінні середовища

- `MYGIT_BACKUP_KEY` — ключ шифрування архівів
- `MYGIT_BACKUP_UPLOAD_URL` — базова URL для upload архівів
- `MYGIT_TRUST_PROXY` — `1`/`true`, довіряти `X-Forwarded-For`

## [3.0.9] - 2026-08-12

### 🆕 Нові ендпоінти (frontend parity з Python/Django-версією)

- **Групи**: `GET/POST /groups/`, `GET /groups/{id}/`, `GET /groups/{id}/projects/`. Створення проєктів у групах (`owner_type=organization`) з рольовою моделлю через `group_members`.
- **Адміністрування користувачів**: `GET /users/`, `GET/PATCH /users/{username}/` (list, profile, is_superuser/is_active/username/email/password). Superuser-only; заборонено зняття адміна з себе.
- **Integration tokens**: `GET/POST /users/{username}/integration-tokens/`, `DELETE .../{provider}/` — GitHub/GitLab PAT, шифруються AES-256-GCM (ключ від JWT secret), у відповіді лише masked.
- **Імпорт проєктів**: `GET /projects/import/{provider}/repos/` (список репозиторіїв GitHub/GitLab через token користувача), `POST /projects/import/` (github/gitlab/custom URL), `git clone --bare`.
- **Disk browser (admin)**: `GET /projects/browse-disk/`, `POST /projects/create-disk-folder/` — вибір/створення фізичного каталогу для репозиторію. Superuser-only.
- **Admin system**: `GET /admin/audit-events/` (audit log), `GET/POST/PATCH /admin/backup-schedules/`, `POST /admin/backup-schedules/{id}/run_now/` (tar.gz-беккап даних у фоновому goroutine), `GET /admin/backup-jobs/`, `GET /admin/mirror-targets/`, `POST /admin/mirror-targets/{id}/sync/` (git push --mirror у локальне сховище), `GET /repository-import-jobs/`.

### 🛡 Безпека

- **Custom storage path** (`custom_disk_path`) — тільки для superuser, лише абсолютні шляхи.
- **PAT expiry**: токени приймають `expires_in_days`, `expires_at` зберігається, протерміновані PAT відхиляються при авторизації.
- **Імпорт за URL** — лише `http(s)` схеми (file://, ssh://, ftp:// заблоковані).
- **Audit log** — події repo.create/import, group.create, user.update, backup.run, mirror.sync записуються.

### 🐞 Виправлено

- `owner_id` у `/projects/` може приходити рядком (як шле SPA) — гнучке декодування.
- Git smart HTTP тепер коректно працює з репозиторіями на кастомних шляхах (`storage_path`).

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
