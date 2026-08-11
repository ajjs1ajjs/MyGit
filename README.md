# 🐙 MyGit

**Self-hosted Git platform** — альтернатива GitLab/Gitea, переписана на Go.

[![Go 1.25](https://img.shields.io/badge/Go-1.25-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-3.0.0-orange.svg)](CHANGELOG.md)

---

## ✨ Можливості

- **Git smart HTTP** — clone/push/pull через стандартний протокол (Basic auth: логін/пароль або PAT)
- **Репозиторії**: створення, fork, видалення; bare-репо на диску `repos/{owner}/{name}.git`
- **Перегляд коду**: дерево файлів, blob, raw, blame, коміти, гілки, теги
- **Issues**: створення, коментарі, стани
- **Merge Requests**: створення, merge
- **Webhooks**: CRUD
- **Користувачі**: реєстрація, JWT, SSH-ключі, Personal Access Tokens
- **Права**: superuser / рольові (owner 50, maintainer 40, developer 30, reporter 20, guest 10)
- **SPA-фронтенд**: Vue 3 + Tailwind (embedded через `go:embed`)
- **SSH git**: AuthorizedKeysCommand + internal API

---

## 🚀 Швидкий старт

**Linux:**
```bash
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.sh | sudo bash
```

**Windows** (збірка з сирців):
```powershell
go build -o mygit.exe ./cmd/mygit
.\mygit.exe -port 8080
```

Після запуску відкрийте `http://<IP>:8080/` та **зареєструйте перший обліковий запис** — він стане власником (superuser).

### Git push/pull

```bash
git clone http://<IP>:8080/alice/myrepo.git
cd myrepo
git add . && git commit -m "first"
git push origin main   # попросить логін/пароль або PAT
```

Для PAT: Профіль → Tokens → створити; використовуйте як пароль.

### Docker

```bash
docker compose up -d   # або: docker run -d -p 8080:8080 -v mygit-data:/data mygit
```

---

## ⚙️ Змінні середовища

| Змінна | Призначення | Дефолт |
|--------|-------------|--------|
| `MYGIT_BASE_DIR` | Базова директорія | поточна |
| `MYGIT_REPOS_ROOT` | Директорія bare-репозиторіїв | `{base}/repos` |
| `MYGIT_DB_PATH` | Шлях до SQLite | `{base}/mygit.db` |
| `MYGIT_JWT_SECRET` | Секрет JWT (авто-генерація поруч із БД) | авто |
| `MYGIT_INTERNAL_API_TOKEN` | Токен для git hooks/CI | авто (друкується лише в TTY) |
| `MYGIT_GIT_BINARY` | Шлях до git | `git` |
| `MYGIT_TLS_CERT` | Шлях до TLS-сертифіката (вмикає HTTPS) | порожньо |
| `MYGIT_TLS_KEY` | Шлях до TLS-ключа | порожньо |

> **HTTPS:** якщо задано `MYGIT_TLS_CERT` і `MYGIT_TLS_KEY`, сервер слухає HTTPS самостійно.
> Інакше (рекомендовано) розмістіть MyGit за reverse-proxy (Caddy/Nginx) із TLS-termination.
> У режимі cookie-авторизації `Secure`-прапорець увімкнеться автоматично лише при HTTPS.

> **Авторизація:** SPA використовує **HttpOnly cookies** (`SameSite=Strict`), токени не зберігаються
> у `localStorage`. Програмні клієнти (hooks/CI/CLI) можуть і надалі передавати JWT/PAT через
> `Authorization: Bearer` або Basic auth.

---

## 🧩 Технології

**Бекенд:** Go 1.25 · net/http · chi · SQLite (WAL, modernc.org/sqlite) · golang-jwt · git subprocess (smart HTTP)
**Фронтенд:** Vue 3 · TypeScript · Vite · Tailwind (embedded)
