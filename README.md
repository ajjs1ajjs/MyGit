# MyGit

Default local login after a fresh install:

- Username: `admin`
- Password: `291263`

Change this password immediately after the first login. The admin account is marked with
`must_change_password=True`.

Optional LDAP login can be enabled with `MYGIT_LDAP_ENABLED=True` plus the
`MYGIT_LDAP_*` settings from `.env.example`. Local username/password login remains available.

Саморозгорнута Git-платформа для підприємств (аналог GitLab/Gitea).

**Один рядок для встановлення:**
```bash
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/setup.sh | sudo bash
```

Після встановлення відкрийте `http://ВАШ_IP:8060`  
Логін: `admin@example.com` / пароль генерується автоматично (або встановіть через `ADMIN_PASSWORD`)

---

## Можливості

| Функція | Статус |
|---|---|
| Git-репозиторії (створення, клонування, push/pull) | ✅ |
| Smart HTTP (git-upload-pack / git-receive-pack) | ✅ |
| SSH-доступ (AuthorizedKeysCommand) | ✅ |
| Git-хуки (pre-receive, post-receive) | ✅ |
| Користувачі, JWT-авторизація, SSH-ключі, PAT-токени | ✅ |
| Перегляд коду (файли, коміти, diff, blame, гілки, теги) | ✅ |
| Issues (лейбли, milestones, коментарі, Markdown) | ✅ |
| Merge Requests (diff, рев'ю, merge: ff / merge-commit) | ✅ |
| Групи/організації (підгрупи, ролі, команди) | ✅ |
| Wiki (Markdown, створення/редагування сторінок) | ✅ |
| Snippets (Gist-аналоги, public/private) | ✅ |
| Глобальний пошук (репо, issues, MR, wiki) | ✅ |
| CI/CD (пайплайни, джоби, `.mygit-ci.yml`, раннер) | ✅ |
| Webhooks (з Celery-доставкою, HMAC-підписом) | ✅ |
| Сповіщення (in-app, email) | ✅ |
| OpenAPI/Swagger-документація | ✅ |
| Prometheus-метрики | ✅ |
| Sentry (за бажанням) | ✅ |
| Зміна пароля при першому вході | ✅ |
| Темна тема (автоматично) | ✅ |
| Адаптивний дизайн | ✅ |

---

## Стек

**Backend:** Python 3.12+ / Django 5 / DRF / Celery / PostgreSQL / Redis  
**Frontend:** Vue 3 / TypeScript / TailwindCSS / Pinia / Vue Router  
**DevOps:** Nginx / Gunicorn / Uvicorn / systemd / Docker Compose

---

## Швидкий старт

### Вимоги
- Linux (Ubuntu 22.04+ або аналог)
- Python 3.9+
- PostgreSQL (автоматично, або SQLite як fallback)
- Node.js 22+ (автоматично)

### Встановлення

```bash
# Автоматично (IP визначається сам, порт 8060)
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/setup.sh | sudo bash

# З вказанням домену та порту
sudo DOMAIN=git.company.com PORT=443 bash <(curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/setup.sh)
```

### Оновлення

```bash
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/update.sh | sudo bash
```

### Docker

```bash
cd docker && docker-compose up -d
```

---

## Розробка

```bash
git clone https://github.com/ajjs1ajjs/MyGit.git
cd MyGit

# Бекенд
python3 -m venv venv && source venv/bin/activate
pip install -r requirements.txt
cp .env.example .env
python manage.py migrate
python manage.py runserver

# Фронтенд
cd frontend && npm install && npm run dev
```

**Перевірки якості:**
```bash
ruff check . && ruff format --check .
mypy config apps --ignore-missing-imports
pytest
cd frontend && npx vue-tsc --noEmit
```

---

## API-ендпоінти

Основні ендпоінти:
```
POST   /api/v1/auth/register/           Реєстрація
POST   /api/v1/auth/login/              JWT-логін
POST   /api/v1/auth/refresh/            Оновлення токена
POST   /api/v1/auth/password-reset/     Скидання пароля

GET    /api/v1/projects/                Список репозиторіїв
POST   /api/v1/projects/                Створити репо
GET    /api/v1/projects/:id/tree/       Файлове дерево
GET    /api/v1/projects/:id/commits/    Історія комітів
GET    /api/v1/projects/:id/branches/   Гілки

GET    /api/v1/projects/:id/issues/     Issues
POST   /api/v1/projects/:id/issues/     Створити issue

GET    /api/v1/projects/:id/merge_requests/  MR-список
POST   /api/v1/projects/:id/merge_requests/:num/merge/  Злити MR

GET    /api/v1/groups/                  Групи
POST   /api/v1/groups/                  Створити групу

GET    /api/v1/search?q=...             Глобальний пошук
GET    /api/v1/notifications/           Сповіщення
```

---

## Структура проєкту

```
mygit/
├── apps/                  # Django-додатки
│   ├── accounts/          # Користувачі, SSH-ключі, токени
│   ├── api/               # REST API (ViewSets, роути)
│   ├── ci_cd/             # CI/CD пайплайни
│   ├── core/              # Базові моделі, міксини
│   ├── git_service/       # Git-операції (subprocess, хуки)
│   ├── issues/            # Issues, лейбли, milestones
│   ├── merge_requests/    # Merge Requests, рев'ю
│   ├── notifications/     # Сповіщення
│   ├── organizations/     # Групи, учасники, команди
│   ├── repositories/      # Репозиторії (ORM)
│   ├── search/            # Глобальний пошук
│   ├── snippets/          # Snippets
│   ├── webhooks/          # Webhooks + Celery
│   └── wiki/              # Wiki-сторінки
├── config/                # Налаштування Django
├── frontend/              # Vue.js 3 SPA
├── docker/                # Docker Compose
├── ansible/               # Ansible-плейбук
├── scripts/               # SSH-скрипти, CI-раннер, бекап
├── setup.sh               # Встановлення (одна команда)
└── update.sh              # Оновлення (одна команда)
```

---

## Ліцензія

MIT
