<div align="center">

# 🐙 MyGit

### A self-hosted Git platform for teams that own their infrastructure

**Self-hosted Git platform** — альтернатива GitLab/Gitea, переписана на Go.

<img src="docs/banner.svg" width="100%" alt="MyGit">

[![CI](https://img.shields.io/github/actions/workflow/status/ajjs1ajjs/MyGit/ci.yml?label=CI)](https://github.com/ajjs1ajjs/MyGit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-3.4.0-orange.svg)](CHANGELOG.md)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-blue.svg)](https://go.dev)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20Windows-blue.svg)
![PWA](https://img.shields.io/badge/PWA-offline-cyan)

[**🌐 Live Site**](https://ajjs1ajjs.github.io/MyGit/) · [Releases](https://github.com/ajjs1ajjs/MyGit/releases) · [Actions](https://github.com/ajjs1ajjs/MyGit/actions)

</div>

---

## 🍎 macOS / Apple Silicon

MyGit має повну підтримку macOS, включаючи **Apple Silicon (M1/M2/M3/M4)** та **Intel Mac**:

- **Нативні ARM64 бінарники** — оптимізовані для Apple Silicon
- **Universal binary** — один бінарник працює на Intel та Apple Silicon
- **macOS 12+ Monterey, 13 Ventura, 14 Sonoma, 15 Sequoia, 26 Tahoe**

**Встановлення на macOS (Homebrew):**
```bash
# Через curl
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.sh | sudo bash

# Архітектура визначається автоматично: aarch64-apple-darwin (M1+) або x86_64-apple-darwin (Intel)
```

**Запуск як сервіс (launchd):**
```bash
# Створіть ~/Library/LaunchAgents/com.mygit.server.plist
cat > ~/Library/LaunchAgents/com.mygit.server.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.mygit.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/mygit/mygit</string>
    <string>-port</string><string>8060</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
EOF

launchctl load ~/Library/LaunchAgents/com.mygit.server.plist
```

**Підтримувані архітектури macOS:**

| Архітектура | Позначення | Підтримка |
|---|---|---|
| Apple Silicon (M1/M2/M3/M4) | `aarch64-apple-darwin` | ✅ Native |
| Intel Mac | `x86_64-apple-darwin` | ✅ Native |
| Universal | `universal-apple-darwin` | ✅ Fat binary |

---

## 🖼️ Screenshots

<p align="center">
  <img src="docs/screenshots/home.png" width="49%" alt="Проєкти">
  <img src="docs/screenshots/repo.png" width="49%" alt="Репозиторій">
  <img src="docs/screenshots/commits.png" width="49%" alt="Коміти">
  <img src="docs/screenshots/issues.png" width="49%" alt="Issues">
</p>

<p align="center">
  <img src="docs/screenshots/merge_requests.png" width="49%" alt="Merge Requests">
  <img src="docs/screenshots/login.png" width="49%" alt="Вхід">
</p>

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
- **SSH git**: AuthorizedKeysCommand + internal API (Linux, через системний OpenSSH)
- **Платформа**: Linux (amd64/arm64) та Windows (amd64/arm64) — статичні бінарники

---

## 🚀 Швидкий старт

**Ubuntu / Debian** (одна команда і встановлює, і оновлює):
```bash
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.sh | sudo bash
```

**Windows 10/11 / Server 2016+** (PowerShell, від імені Administrator; та ж команда встановлює і оновлює):
```powershell
irm https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.ps1 | iex
```
Потрібен встановлений Git for Windows (`winget install Git.Git`) — MyGit викликає системний `git` для smart HTTP. Інсталятор реєструє MyGit як Windows Service (`mygit`, автозапуск, авто-перезапуск при збої), дані зберігаються в `%ProgramData%\mygit`, бінарник — у `%ProgramFiles%\mygit`.

> **Обмеження:** git-over-HTTP(S) працює повністю на обох платформах. **SSH-git** (`AuthorizedKeysCommand`) залежить від системного OpenSSH-сервера з підтримкою `AuthorizedKeysCommand`, який на Linux (Ubuntu/Debian, `openssh-server`) налаштовується типово. На Windows Win32-OpenSSH теоретично підтримує `AuthorizedKeysCommand`, але це не входить у стандартний `install.ps1` і потребує додаткового ручного налаштування `sshd_config` — тому SSH-git на Windows наразі **не підтримується "з коробки"**; використовуйте git-over-HTTP(S).

Після запуску відкрийте `http://<IP>:8060/` та **зареєструйте перший обліковий запис** — він стане власником (superuser).

### Git push/pull

```bash
git clone http://<IP>:8060/alice/myrepo.git
cd myrepo
git add . && git commit -m "first"
git push origin main   # попросить логін/пароль або PAT
```

Для PAT: Профіль → Tokens → створити; використовуйте як пароль.

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
| `MYGIT_BACKUP_KEY` | Ключ шифрування архівів беккапів (AES-256-GCM) | похідний від JWT secret |
| `MYGIT_BACKUP_UPLOAD_URL` | Базова URL для upload архівів (HTTP PUT / S3-style) | порожньо |
| `MYGIT_TRUST_PROXY` | Довіряти `X-Forwarded-For` (rate limiting) та `X-Forwarded-Proto` (Secure cookie, HSTS) за довіреним reverse-proxy (`1`/`true`) | `0` |
| `MYGIT_CUSTOM_REPOS_ROOT` | Корінь для кастомних шляхів репозиторіїв (`custom_disk_path`, superuser-only) | `{base}` |

> **HTTPS:** якщо задано `MYGIT_TLS_CERT` і `MYGIT_TLS_KEY`, сервер слухає HTTPS самостійно.
> Інакше (рекомендовано) розмістіть MyGit за reverse-proxy (Caddy/Nginx) із TLS-termination.
> У режимі cookie-авторизації `Secure`-прапорець увімкнеться автоматично лише при HTTPS.

> **Беккапи:** увімкнені розклади (`enabled`) виконуються автоматично фоновими планувальником.
> Частота: `daily` / `hourly` / `weekly` + `time_of_day` (`HH:MM[:SS]`). `encrypt` шифрує архів
> (AES-256-GCM), `upload` відправляє його на `MYGIT_BACKUP_UPLOAD_URL` (PUT, підтримує
> `{filename}` або presigned S3 URL), `keep_local` лишає лише останні N архівів.

> **Авторизація:** SPA використовує **HttpOnly cookies** (`SameSite=Strict`), токени не зберігаються
> у `localStorage`. Програмні клієнти (hooks/CI/CLI) можуть і надалі передавати JWT/PAT через
> `Authorization: Bearer` або Basic auth.

## 📱 PWA

SPA-фронтенд MyGit — це **Progressive Web App** (`vite-plugin-pwa`): встановлюється на телефон/ПК як окремий застосунок і кешує assets через Service Worker. Для цього відкрийте `http://<IP>:8060/` у браузері та оберіть "Встановити додаток".

---

## 🧩 Технології

**Бекенд:** Go 1.25 · net/http · chi · SQLite (WAL, modernc.org/sqlite) · golang-jwt · git subprocess (smart HTTP)
**Фронтенд:** Vue 3 · TypeScript · Vite · Tailwind (embedded)
