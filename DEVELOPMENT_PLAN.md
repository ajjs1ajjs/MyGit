# План розробки MyGit

> Ентерпрайз-рішення для керування Git-репозиторіями (аналог GitLab) для 50+ розробників. Локальне розгортання на Ubuntu Linux.

---

## 1. Стек технологій

| Шар | Технологія | Обґрунтування |
|---|---|---|
| **Backend** | Python 3.12 + Django 5 + Django REST Framework | Вбудована адмінка, auth, ORM, міграції — скорочує розробку на місяці |
| **Git-операції** | subprocess (git binary) + GitPython | subprocess для smart-HTTP streaming; GitPython для метаданих (branches, commits) |
| **Frontend** | Vue.js 3 + Vite + TailwindCSS + Pinia | Реактивний SPA, швидка збірка, utility-first CSS |
| **База даних** | PostgreSQL 16 | Повноцінна реляційна БД, FTS-пошук, JSONB для метаданих |
| **Кеш/Брокер** | Redis 7 | Кешування сесій + брокер для Celery |
| **Фонові задачі** | Celery | git gc, клонування, архівація, вебхуки, email |
| **Веб-сервер** | Nginx + Gunicorn (sync) / Uvicorn (async) | Термінація TLS, статика, проксі smart-HTTP та API |
| **DevOps** | Docker Compose (dev), Ansible (production Ubuntu) | |


## 2. Архітектура системи

```
                           ┌─────────────┐
                           │   Browser   │  (Vue.js SPA)
                           └──────┬──────┘
                                  │ HTTPS
                           ┌──────┴──────┐
                           │   Nginx     │  (reverse proxy, TLS, static)
                           └──┬──┬───┬───┘
                     ┌────────┘  │   └─────────┐
                     ▼           ▼              ▼
               /api/*      /*.git/*       /static/
              (async)      (sync)        (Vue build)
             ┌──────┐   ┌─────────┐
             │Uvicorn│   │ Gunicorn │      ┌──────────┐
             │(ASGI) │   │ (WSGI)  │      │ Celery   │
             └──┬──┬─┘   └────┬────┘      │ Workers  │
                │  │           │           └────┬─────┘
                │  │    ┌──────┴──────┐        │
                │  └───►│    Django    │◄───────┘
                │       │  (git_service│
                │       │   + ORM)     │
                │       └──┬─────┬─────┘
                │          │     │
                │    ┌─────┘     └──────┐
                │    ▼                  ▼
                │  ┌──────┐     ┌───────────┐
                │  │ git  │     │PostgreSQL │
                │  │binary│     └───────────┘
                │  └──────┘
                │
                ▼
           ┌──────────┐
           │  Redis   │
           └──────────┘
```

### Ключові архітектурні рішення

1. **Git Smart HTTP** — синхронні view через subprocess (pkt-line протокол не виграє від async). Використовується Gunicorn (WSGI) для цих endpoint'ів.
2. **REST API** — асинхронні view через Uvicorn (ASGI) для високої конкурентності CRUD-операцій.
3. **Git-операції винесені в окремий сервісний шар** — жодних викликів ORM усередині git-потоку.
4. **SSH Git** — через OpenSSH `AuthorizedKeysCommand` (масштабується для 50+ ключів).
5. **3 типи аутентифікації**: Session (браузер), JWT (API), HTTP Basic + Personal Access Tokens (git HTTP).


## 3. Дорожня карта (Roadmap)

### Пакет покращень: нові функції та можливості

- [x] Система локальних бекапів: архів `.tar.gz`, база даних, репозиторії, media, `.env`, manifest і SHA256-перевірка.
- [x] Відновлення з бекапу: контрольне підтвердження, restore SQLite/PostgreSQL, репозиторіїв і media.
- [x] Хмарні бекапи: upload/download/list через `rclone` або S3-compatible storage.
- [x] Ретеншн локальних архівів через `MYGIT_BACKUP_KEEP_LOCAL`.
- [x] Шифрування backup-архівів перед відправкою у хмару.
- [x] Автоматичний тест відновлення в окреме staging-середовище.
- [x] Реплікація Git-репозиторіїв у хмарний backup-target.
- [x] Розклад бекапів у UI адміністратора з журналом останніх запусків.
- [x] Реплікація Git-репозиторіїв на другий сервер для швидкого disaster recovery.
- [x] UI для імпорту репозиторіїв з GitHub/GitLab/Gitea.
- [x] Захищені гілки, CODEOWNERS і обов'язкові approvals для merge requests.
- [x] Releases з артефактами, changelog і підписаними тегами.
- [x] Пакетний пошук по коду з індексацією репозиторіїв.
- [x] Аудит-лог дій адміністратора і користувачів.
- [x] 2FA/WebAuthn для адміністраторів і критичних операцій.

### Фаза 0 — Інфраструктура (тиждень 1-2)

- [x] Ініціалізація Django-проєкту (`config/`, `apps/`)
- [x] Налаштування Docker Compose (Django + PostgreSQL + Redis + Nginx)
- [x] Налаштування CI/CD (GitHub Actions: ruff, mypy, pytest)
- [x] Кастомна модель `User` (`accounts.User`)
- [x] Базова адмінка Django для керування користувачами
- [x] Налаштування `.env` через `django-environ`

### Фаза 1 — Користувачі та автентифікація (тиждень 3-4)

- [x] Реєстрація, логін, логаут, відновлення пароля
- [x] Профіль користувача (аватар, ім'я, email, біо)
- [x] SSH-ключі (додавання, видалення, валідація)
- [x] Personal Access Tokens (створення, відкликання, scopes)
- [x] JWT-ендпоінти (login/refresh/verify/logout)
- [x] Ролі (admin / user) — `is_superuser` для адміністраторів
- [x] `AuthorizedKeysCommand` скрипт для SSH-доступу

### Фаза 2 — Репозиторії: ядро (тиждень 5-7)

- [x] Модель `Repository` (owner, name, description, visibility, default_branch)
- [x] CRUD репозиторіїв через API + Web UI
- [x] Ініціалізація bare-репозиторія на диску
- [x] Smart HTTP: `git-upload-pack` (pull/clone)
- [x] Smart HTTP: `git-receive-pack` (push)
- [x] Git over SSH через скрипт-обгортку
- [x] `pre-receive` / `post-receive` хуки для валідації та логування
- [x] Private/public видимість із перевіркою прав доступу
- [x] Fork репозиторія
- [x] Видалення та архівація репозиторія

### Фаза 3 — Перегляд коду (тиждень 8-10)

- [x] Файловий браузер (дерево директорій, `git ls-tree`)
- [x] Перегляд файлів із підсвічуванням синтаксису (highlight.js / Prism)
- [x] Історія комітів (пагінація, граф)
- [x] Перегляд коміту (diff, статистика)
- [x] `git blame` для файлів
- [x] Гілки (список, створення, видалення)
- [x] Теги (список, створення, видалення, annotated tags)
- [x] Завантаження архіву репозиторія (tar.gz, zip)
- [x] Raw-перегляд файлу
- [x] README.md рендеринг на головній сторінці репо

### Фаза 4 — Issues (тиждень 11-13)

- [x] Модель `Issue` (title, description, state, labels, milestone, assignee)
- [x] CRUD issues через API + UI
- [x] Лейбли (назва, колір)
- [x] Milestones (назва, дедлайн, прогрес)
- [x] Призначення відповідальних (assignees)
- [x] Закриття/відкриття issue
- [x] Коментарі до issues (Markdown)
- [x] Пошук та фільтрація issues
- [x] Board view (Kanban-дошка)

### Фаза 5 — Merge Requests (тиждень 14-17)

- [x] Модель `MergeRequest` (source_branch, target_branch, title, state)
- [x] Створення MR: вибір гілок, опис, assignee, reviewer
- [x] Diff-перегляд (зміни між source і target)
- [x] Inline-коментарі до рядків дифу
- [x] Загальні коментарі до MR
- [x] Статуси MR: open, draft, merged, closed
- [x] Кнопка Merge (fast-forward / merge commit / squash)
- [x] Перевірка конфліктів перед merge
- [x] Автоматичне закриття issue через MR (closes #123)
- [x] Activity feed

### Фаза 6 — Організації та групи (тиждень 18-20)

- [x] Модель `Organization` / `Group` (name, path, description, avatar)
- [x] Учасники групи з ролями (Owner, Maintainer, Developer, Reporter, Guest)
- [x] Репозиторії групи
- [x] Підгрупи (subgroups, ієрархія)
- [x] Успадкування прав (група → підгрупа → репозиторій)
- [x] Команди (teams) усередині груп

### Фаза 7 — Wiki, Snippets, Пошук (тиждень 21-23)

- [x] Wiki на репозиторій (сторінки в Markdown, git-backed)
- [x] Snippets (Gist-аналоги: код + опис, public/private)
- [x] Повнотекстовий пошук (PostgreSQL FTS або Elasticsearch)
  - Репозиторії, issues, MR, код, wiki
- [x] Глобальний activity feed

### Фаза 8 — CI/CD (тиждень 24-28)

- [x] Модель `Pipeline` (status, ref, commit, stages)
- [x] Модель `Job` (name, stage, status, log, artifacts)
- [x] Файл конфігурації `.mygit-ci.yml`
- [x] CI-раннер (окремий процес, опитує сервер)
- [x] Виконання джоб у Docker-контейнерах
- [x] Перегляд логів джоби в реальному часі (WebSocket)
- [x] Артефакти збірки
- [x] Інтеграція з MR (статус перевірок)

### Фаза 9 — Webhooks, Сповіщення, API (тиждень 29-31)

- [x] Webhooks (repo-level + system-level)
- [x] Доставка вебхуків (Celery tasks з retry)
- [x] Email-сповіщення (реєстрація, MR, issue зміни)
- [x] In-app сповіщення (WebSocket / polling)
- [x] REST API документація (OpenAPI/Swagger)
- [x] Rate limiting

### Фаза 10 — Фіналізація (тиждень 32-34)

- [x] Dark mode / теми
- [x] Локалізація (UA / EN)
- [x] Monitoring (Prometheus metrics, Sentry)
- [x] Бекап (pg_dump + rsync репозиторіїв)
- [x] Ansible playbook для production-деплою на Ubuntu
- [x] Документація для адміністратора
- [x] Тестування навантаження (locust — 50+ одночасних push/pull)
- [x] Реліз 1.0


## 4. Структура Django-проєкту

```
mygit/
├── config/                     # Django project config
│   ├── settings/
│   │   ├── base.py             # Спільні налаштування
│   │   ├── local.py            # Dev (debug=True, sqlite/redis локально)
│   │   └── production.py       # Prod (debug=False, PostgreSQL, Sentry)
│   ├── urls.py                 # Кореневі URL (адмінка, API, git-HTTP)
│   ├── asgi.py                 # Uvicorn entry point (async)
│   └── wsgi.py                 # Gunicorn entry point (sync, git HTTP)
├── apps/
│   ├── core/                   # Спільні моделі, міксини, утиліти
│   ├── accounts/               # User, SSHKey, PersonalAccessToken, Profile
│   ├── organizations/          # Group, GroupMember, Team
│   ├── repositories/           # Repository (ORM), налаштування
│   ├── git_service/            # Git-операції (subprocess, GitPython), без ORM
│   ├── commits/                # Коміти, диффи, blame (read-only з git)
│   ├── branches/               # Гілки, protected branches
│   ├── tags/                   # Теги, releases
│   ├── issues/                 # Issue, Label, Milestone, IssueComment
│   ├── merge_requests/         # MR, MRComment, MRReview
│   ├── wiki/                   # Wiki-сторінки
│   ├── snippets/               # Snippets (Gist)
│   ├── ci_cd/                  # Pipeline, Job, Runner
│   ├── webhooks/               # Webhook, WebhookDelivery
│   ├── notifications/          # Сповіщення
│   ├── search/                 # Пошуковий індекс
│   └── api/                    # DRF routers, ViewSets, serializers
├── frontend/                   # Vue.js 3 SPA (окремий проєкт)
│   ├── src/
│   │   ├── components/         # Спільні компоненти
│   │   ├── views/              # Сторінки
│   │   ├── stores/             # Pinia stores
│   │   ├── api/                # Axios/fetch клієнт
│   │   ├── router/             # Vue Router
│   │   └── assets/             # CSS, зображення
│   ├── vite.config.ts
│   └── package.json
├── templates/                  # Django templates (email, адмінка)
├── static/                     # Зібрані статичні файли
├── media/                      # Завантажені файли (аватари)
├── repos/                      # Bare-репозиторії на диску
├── docker/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   └── nginx.conf
├── ansible/                    # Ansible playbooks для Ubuntu
├── scripts/
│   └── mygit-shell             # SSH AuthorizedKeysCommand скрипт
├── manage.py
├── requirements.txt
├── pyproject.toml
└── README.md
```

## 5. Схема бази даних (ключові таблиці)

### accounts_user
| Поле | Тип | Примітка |
|---|---|---|
| id | UUID PK | |
| username | VARCHAR(255) UNIQUE | |
| email | VARCHAR(255) UNIQUE | |
| password | VARCHAR(255) | hashed (Django) |
| full_name | VARCHAR(255) | |
| bio | TEXT | Markdown |
| avatar | VARCHAR(512) | Шлях до файлу |
| is_active | BOOLEAN | |
| is_superuser | BOOLEAN | Адміністратор системи |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### accounts_sshkey
| Поле | Тип |
|---|---|
| id | UUID PK |
| user_id | FK → accounts_user |
| title | VARCHAR(255) |
| public_key | TEXT |
| fingerprint | VARCHAR(64) UNIQUE |
| created_at | TIMESTAMP |

### accounts_token
| Поле | Тип |
|---|---|
| id | UUID PK |
| user_id | FK → accounts_user |
| name | VARCHAR(255) |
| token_hash | VARCHAR(128) UNIQUE |
| scopes | JSONB | `["read_repo", "write_repo", "api"]` |
| last_used_at | TIMESTAMP NULL |
| expires_at | TIMESTAMP NULL |
| created_at | TIMESTAMP |

### repositories_repository
| Поле | Тип | Примітка |
|---|---|---|
| id | UUID PK | |
| owner_type | VARCHAR(50) | "user" / "organization" |
| owner_id | UUID | FK (polymorphic) |
| name | VARCHAR(255) | |
| path | VARCHAR(512) UNIQUE | `owner/repo` |
| description | TEXT | Markdown |
| visibility | VARCHAR(20) | "public" / "private" / "internal" |
| default_branch | VARCHAR(255) | |
| is_archived | BOOLEAN | |
| is_fork | BOOLEAN | |
| forked_from_id | FK (self) NULL | |
| size_kb | BIGINT | |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

### repositories_access
| Поле | Тип |
|---|---|
| id | UUID PK |
| user_id | FK → accounts_user |
| repository_id | FK → repositories_repository |
| role | INT | 0=none, 10=guest, 20=reporter, 30=developer, 40=maintainer, 50=owner |
| created_at | TIMESTAMP |

### issues_issue
| Поле | Тип |
|---|---|
| id | UUID PK |
| repository_id | FK → repositories_repository |
| author_id | FK → accounts_user |
| assignee_id | FK → accounts_user NULL |
| milestone_id | FK → issues_milestone NULL |
| title | VARCHAR(512) |
| description | TEXT (Markdown) |
| state | VARCHAR(20) | "open" / "closed" |
| number | INT | Унікальний у межах репо |
| created_at | TIMESTAMP |
| updated_at | TIMESTAMP |
| closed_at | TIMESTAMP NULL |

### merge_requests_mergerequest
| Поле | Тип |
|---|---|
| id | UUID PK |
| repository_id | FK → repositories_repository |
| author_id | FK → accounts_user |
| source_branch | VARCHAR(255) |
| target_branch | VARCHAR(255) |
| title | VARCHAR(512) |
| description | TEXT (Markdown) |
| state | VARCHAR(20) | "open" / "merged" / "closed" / "draft" |
| merge_commit_sha | VARCHAR(40) NULL |
| number | INT | Унікальний у межах репо |
| created_at | TIMESTAMP |
| updated_at | TIMESTAMP |
| merged_at | TIMESTAMP NULL |
| merged_by_id | FK → accounts_user NULL |

### organizations_group
| Поле | Тип |
|---|---|
| id | UUID PK |
| name | VARCHAR(255) |
| path | VARCHAR(255) UNIQUE |
| description | TEXT |
| parent_id | FK (self) NULL |
| created_at | TIMESTAMP |

### organizations_groupmember
| Поле | Тип |
|---|---|
| id | UUID PK |
| group_id | FK → organizations_group |
| user_id | FK → accounts_user |
| role | INT | 10=guest, 20=reporter, 30=developer, 40=maintainer, 50=owner |
| created_at | TIMESTAMP |


## 6. API-дизайн (REST)

```
GET    /api/v1/user                              # Поточний користувач
POST   /api/v1/auth/register                     # Реєстрація
POST   /api/v1/auth/login                        # JWT логін
POST   /api/v1/auth/refresh                      # JWT refresh
POST   /api/v1/auth/logout                       # JWT logout

GET    /api/v1/users/:username                   # Профіль
GET    /api/v1/users/:username/keys              # SSH ключі
POST   /api/v1/users/:username/keys              # Додати ключ
DELETE /api/v1/users/:username/keys/:id          # Видалити ключ
GET    /api/v1/users/:username/tokens            # PAT список
POST   /api/v1/users/:username/tokens            # Створити PAT
DELETE /api/v1/users/:username/tokens/:id        # Відкликати PAT

GET    /api/v1/projects                          # Список репо
POST   /api/v1/projects                          # Створити репо
GET    /api/v1/projects/:id                      # Деталі репо
PUT    /api/v1/projects/:id                      # Оновити репо
DELETE /api/v1/projects/:id                      # Видалити репо
POST   /api/v1/projects/:id/fork                 # Fork

GET    /api/v1/projects/:id/tree                 # Файлове дерево
GET    /api/v1/projects/:id/blobs/:sha           # Вміст файлу
GET    /api/v1/projects/:id/blame/:path          # Git blame
GET    /api/v1/projects/:id/archive              # Завантажити архів
GET    /api/v1/projects/:id/commits              # Історія комітів
GET    /api/v1/projects/:id/commits/:sha         # Окремий коміт
GET    /api/v1/projects/:id/commits/:sha/diff    # Diff коміту
GET    /api/v1/projects/:id/branches             # Гілки
POST   /api/v1/projects/:id/branches             # Створити гілку
DELETE /api/v1/projects/:id/branches/:name       # Видалити гілку
GET    /api/v1/projects/:id/tags                 # Теги
POST   /api/v1/projects/:id/tags                 # Створити тег
DELETE /api/v1/projects/:id/tags/:name           # Видалити тег

GET    /api/v1/projects/:id/issues               # Список issues
POST   /api/v1/projects/:id/issues               # Створити issue
GET    /api/v1/projects/:id/issues/:number       # Окремий issue
PUT    /api/v1/projects/:id/issues/:number       # Оновити issue
GET    /api/v1/projects/:id/issues/:number/comments
POST   /api/v1/projects/:id/issues/:number/comments

GET    /api/v1/projects/:id/merge_requests       # Список MR
POST   /api/v1/projects/:id/merge_requests       # Створити MR
GET    /api/v1/projects/:id/merge_requests/:number
PUT    /api/v1/projects/:id/merge_requests/:number
POST   /api/v1/projects/:id/merge_requests/:number/merge
GET    /api/v1/projects/:id/merge_requests/:number/diff
GET    /api/v1/projects/:id/merge_requests/:number/comments

GET    /api/v1/groups                            # Список груп
POST   /api/v1/groups                            # Створити групу
GET    /api/v1/groups/:id                        # Деталі групи
GET    /api/v1/groups/:id/members                # Учасники
POST   /api/v1/groups/:id/members                # Додати учасника
GET    /api/v1/groups/:id/projects               # Проєкти групи

GET    /api/v1/projects/:id/wiki/:slug           # Wiki-сторінка
PUT    /api/v1/projects/:id/wiki/:slug           # Оновити сторінку

POST   /api/v1/projects/:id/hooks                # Додати вебхук
GET    /api/v1/projects/:id/hooks                # Список вебхуків
DELETE /api/v1/projects/:id/hooks/:id            # Видалити вебхук

GET    /api/v1/search?q=...&type=...             # Повнотекстовий пошук
```


## 7. Git Smart HTTP (деталі реалізації)

```python
# apps/git_service/http_backend.py

import subprocess
from pathlib import Path

SERVICE_MAP = {
    "git-upload-pack": "upload_pack",    # git clone / git pull
    "git-receive-pack": "receive_pack",  # git push
}

def handle_smart_http(repo_path: Path, service: str, input_stream: bytes) -> bytes:
    """
    Обробляє smart-HTTP git протокол.
    
    Використовуємо subprocess для потокової передачі pkt-line даних.
    GitPython не підтримує цей протокол.
    """
    git_service = SERVICE_MAP[service]
    
    proc = subprocess.Popen(
        ["git", git_service, "--stateless-rpc", str(repo_path)],
        stdin=subprocess.PIPE if input_stream else None,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    stdout, stderr = proc.communicate(input=input_stream)
    
    if proc.returncode != 0:
        raise GitServiceError(stderr.decode())
    
    return stdout
```

**URL-маршрутизація (Django):**
```
GET  /{owner}/{repo}.git/info/refs?service=git-upload-pack   → info_refs (advertise)
POST /{owner}/{repo}.git/git-upload-pack                      → rpc
GET  /{owner}/{repo}.git/info/refs?service=git-receive-pack   → info_refs (advertise)
POST /{owner}/{repo}.git/git-receive-pack                     → rpc
```

**Аутентифікація Git HTTP:**
- Читаємо `Authorization: Basic <base64>` хедер
- Валідуємо через `accounts.models.User` з токеном або логін/паролем
- Якщо публічний репо + `git-upload-pack` → дозволено без auth
- `git-receive-pack` завжди вимагає auth

## 8. SSH-доступ

Використовуємо підхід `AuthorizedKeysCommand` (як GitLab):

```bash
# /etc/ssh/sshd_config
AuthorizedKeysCommand /opt/mygit/scripts/mygit-authorized-keys
AuthorizedKeysCommandUser mygit
```

```python
# scripts/mygit-authorized-keys
# 1. Отримує username з аргументів
# 2. Запитує Django API: GET /internal/authorized_keys?username=...
# 3. Повертає рядки authorized_keys з command="mygit-shell <verb> <key_id>"
```

```python
# scripts/mygit-shell
# 1. Читає SSH_ORIGINAL_COMMAND (напр. "git-upload-pack 'owner/repo.git'")
# 2. Парсить команду (upload-pack або receive-pack) та шлях репо
# 3. Валідує права доступу через Django API
# 4. Викликає: git-upload-pack /data/repos/owner/repo.git
```

## 9. Фронтенд (Vue.js 3 SPA)

### Сторінки

```
/                               — Головна (activity feed, список твоїх репо)
/auth/login                     — Логін
/auth/register                  — Реєстрація
/:username                      — Профіль користувача
/:username/:repo                — Репозиторій (файли, README)
/:username/:repo/-/tree/:ref/:path  — Файловий браузер
/:username/:repo/-/blob/:ref/:path  — Перегляд файлу
/:username/:repo/-/commits/:ref — Історія комітів
/:username/:repo/-/commit/:sha  — Окремий коміт
/:username/:repo/-/branches     — Гілки
/:username/:repo/-/tags         — Теги
/:username/:repo/-/issues       — Issues список
/:username/:repo/-/issues/:num  — Окремий issue
/:username/:repo/-/issues/new   — Новий issue
/:username/:repo/-/merge_requests — MR список
/:username/:repo/-/merge_requests/:num — Окремий MR
/:username/:repo/-/merge_requests/new — Новий MR
/:username/:repo/-/settings     — Налаштування репо
/:username/:repo/-/wiki         — Wiki
/groups                         — Список груп
/groups/:name                   — Група
/groups/:name/-/projects        — Проєкти групи
/admin                          — Панель адміністратора
/search?q=...                   — Глобальний пошук
```

### Компонентна структура

```vue
<!-- Основні компоненти -->
<AppLayout>                     # Глобальний layout (navbar, sidebar, footer)
<FileTree>                      # Дерево файлів
<FileViewer>                    # Перегляд файлу з підсвічуванням синтаксису
<CommitList>                    # Список комітів
<DiffView>                      # Уніфікований/розділений diff
<MarkdownRenderer>              # Рендерінг Markdown
<CodeHighlight>                 # Підсвічування коду (Prism.js / highlight.js)
<IssueBoard>                    # Kanban-дошка issues
<BranchSelector>                # Селектор гілок/тегів
<SearchBar>                     # Пошуковий рядок
<Pagination>                    # Пагінація
<Avatar>                        # Аватар користувача
<Dropdown>                      # Випадаюче меню
<Modal>                         # Модальне вікно
<Toast>                         # Сповіщення
```

## 10. Production-деплой на Ubuntu

### Компоненти на сервері

```
/opt/mygit/
├── backend/                    # Django-проєкт
├── frontend/dist/              # Зібраний Vue.js
├── repos/                      # Bare-репозиторії (LVM/NFS)
├── scripts/                    # SSH скрипти
├── venv/                       # Python virtualenv
├── logs/                       # Логи (Gunicorn, Celery, Nginx)
├── .env                        # Змінні оточення
└── docker-compose.prod.yml     # Або systemd unit files
```

### Systemd unit files

```ini
# /etc/systemd/system/mygit-api.service
[Service]
User=mygit
WorkingDirectory=/opt/mygit/backend
ExecStart=/opt/mygit/venv/bin/uvicorn config.asgi:application --host 127.0.0.1 --port 8000
Restart=always

# /etc/systemd/system/mygit-git-http.service
[Service]
User=mygit
WorkingDirectory=/opt/mygit/backend
ExecStart=/opt/mygit/venv/bin/gunicorn config.wsgi:application --bind 127.0.0.1:8001 --workers 4
Restart=always

# /etc/systemd/system/mygit-celery.service
[Service]
User=mygit
WorkingDirectory=/opt/mygit/backend
ExecStart=/opt/mygit/venv/bin/celery -A config worker -l info
Restart=always
```

### Nginx конфігурація

```nginx
server {
    listen 443 ssl http2;
    server_name git.company.local;

    location / {
        root /opt/mygit/frontend/dist;
        try_files $uri /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /admin/ {
        proxy_pass http://127.0.0.1:8000;
    }

    location ~ ^/(.+/.+)\.git/ {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header Host $host;
        client_max_body_size 0;
        proxy_request_buffering off;
    }

    location /static/ {
        alias /opt/mygit/backend/static/;
    }
    location /media/ {
        alias /opt/mygit/backend/media/;
    }
}
```

## 11. Оцінка ресурсів

| Ресурс | Кількість |
|---|---|
| Backend-розробників (Python/Django) | 2-3 |
| Frontend-розробників (Vue.js) | 1-2 |
| DevOps | 1 |
| QA | 1 |
| Тривалість до MVP (Фаза 0-3) | ~10 тижнів |
| Тривалість до v1.0 (всі фази) | ~34 тижні |
| Потужність production-сервера | 4-8 CPU, 16-32 GB RAM, SSD 500GB+ |

## 12. Наступні кроки

1. **Ініціалізувати Git-репозиторій проєкту**
2. **Розгорнути Django-скелет** із Docker Compose
3. **Реалізувати модель User** та базову auth (Фаза 1)
4. **Реалізувати Repository модель** та git HTTP (Фаза 2)
5. Після MVP — поетапно додавати Фази 3-10

---

*План створено: червень 2026 | Версія: 1.0*
