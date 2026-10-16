# Gowave

Быстрая, легковесная и производительная панель управления прокси (Xray-core), написанная на **Go** и **React**.

Gowave — это оптимизированное решение тяжелых Node.js решений. Малое потребление RAM , мгновенный старт и высокая производительность , все это про Gowave.

---

## Архитектура

- **Backend (`backend-go`)**: легковесный сервис на Go с минимальным потреблением RAM.
- **Frontend (`frontend`)**: современный веб-интерфейс на React / TypeScript / Vite. by remnawave
- **Node Agent (`node-go`)**: компактный агент для удаленных серверов на Go (управление Xray-core, сбор трафика, фильтрация nftables).
- **База данных**: встроенный **SQLite** по умолчанию (не требует установки СУБД) или **PostgreSQL** при необходимости масштабирования.

---

## Установка

Нужны сервер на Debian/Ubuntu или RHEL-семействе (`amd64`/`arm64`) и домен, A-запись которого указывает на IP сервера. Панель открывается только по домену через HTTPS, без домена установщик не продолжит.

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"
```

Установщик задаст три вопроса:
1. **База данных**: SQLite (один файл, без Docker, обновление из панели) или PostgreSQL в Docker.
2. **Домен панели**: проверит, что он указывает на этот сервер.
3. **Reverse proxy**: Caddy (сертификат выпускается и продлевается сам) или Nginx + Certbot.

Дальше он сам скачает релиз и сверит `SHA256SUMS`, сгенерирует `APP_SECRET`, создаст службу `gowave`, настроит прокси с HTTPS и откроет 80/443 в `ufw`/`firewalld`. Панель слушает только `127.0.0.1` и доступна по `https://<домен>`. Первый, кто откроет панель, создаёт администратора, поэтому откройте её сразу после установки.

**Переезд с другой панели.** Если на сервере найдена Remnawave (`/opt/remnawave`) или PasarGuard (`/opt/pasarguard`, SQLite или PostgreSQL/TimescaleDB), установщик предложит перенести данные в GoWave на SQLite или на PostgreSQL в Docker. Старая панель останавливается и больше не запускается при загрузке, но её контейнеры и данные не удаляются. Если перенос не удался, она запускается обратно.

**Повторный запуск** той же команды на сервере с GoWave открывает меню: обновить, сменить домен и прокси, удалить (с резервной копией базы по желанию). Конкретную версию можно поставить, указав `GOWAVE_VERSION=v1.2.3` перед `bash`.

Лог установки: `/var/log/gowave-install.log`.

<details>
<summary><b>Ручная установка</b></summary>

### Docker Compose (GoWave + PostgreSQL)

Образ панели берётся из `ghcr.io/aaasazan/gowave`. Docker ставится официальным скриптом: `curl -fsSL https://get.docker.com | sudo sh`.

```bash
mkdir -p /opt/gowave-docker && cd /opt/gowave-docker
curl -fsSLO https://raw.githubusercontent.com/aaaSaZaN/gowave/main/docker-compose.yml
curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/.env.docker.example -o .env
sed -i "s/^APP_SECRET=.*/APP_SECRET=$(openssl rand -hex 32)/; s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 24)/" .env
nano .env   # PANEL_DOMAIN и SUB_PUBLIC_DOMAIN
docker compose up -d
```
Панель публикуется на `127.0.0.1:3000`, дальше настройте [reverse proxy](#reverse-proxy).

Обновление выполняется на сервере (контейнер не может заменить сам себя): `cd /opt/gowave-docker && docker compose pull && docker compose up -d`. CLI-команды запускаются в контейнере: `docker compose exec gowave /app/gowave rescue --list-admins`.

### Systemd + SQLite

```bash
mkdir -p /opt/gowave/frontend/dist && cd /opt/gowave
ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
curl -fL -o gowave-server https://github.com/aaaSaZaN/gowave/releases/latest/download/gowave-linux-$ARCH
curl -fLO https://github.com/aaaSaZaN/gowave/releases/latest/download/gowave-frontend.tar.gz
chmod +x gowave-server && tar -xzf gowave-frontend.tar.gz -C frontend/dist

cat > .env <<ENV
APP_HOST=127.0.0.1
APP_PORT=3000
APP_SECRET=$(openssl rand -hex 32)
DB_DRIVER=sqlite
DATABASE_URL=gowave.db
PANEL_DOMAIN=panel.yourdomain.com
SUB_PUBLIC_DOMAIN=panel.yourdomain.com/api/sub
ENV
chmod 600 .env
```

Служба `/etc/systemd/system/gowave.service`:

```ini
[Unit]
Description=GoWave Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gowave
ExecStart=/opt/gowave/gowave-server
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload && systemctl enable --now gowave
```

### Reverse proxy

**Caddy** (`/etc/caddy/Caddyfile`, затем `systemctl reload caddy`):

```caddy
panel.yourdomain.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:3000
}
```

**Nginx.** Сертификат: `certbot certonly --standalone -d panel.yourdomain.com` (порт 80 должен быть свободен). Конфиг `/etc/nginx/conf.d/gowave.conf`:

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    "" close;
}

upstream gowave {
    server 127.0.0.1:3000;
}

# Редирект с HTTP на HTTPS
server {
    listen 80;
    listen [::]:80;
    server_name panel.yourdomain.com;
    return 301 https://$host$request_uri;
}

server {
    server_name panel.yourdomain.com;
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;

    client_max_body_size 50M;

    location / {
        proxy_http_version 1.1;
        proxy_pass http://gowave;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
    }

    # SSL Configuration (Mozilla Intermediate Guidelines)
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-CHACHA20-POLY1305;
    ssl_session_timeout 1d;
    ssl_session_cache shared:MozSSL:10m;
    ssl_session_tickets off;

    ssl_certificate /etc/letsencrypt/live/panel.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.yourdomain.com/privkey.pem;

    # Gzip Compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_buffers 16 8k;
    gzip_http_version 1.1;
    gzip_min_length 256;
    gzip_types
        application/atom+xml
        application/geo+json
        application/javascript
        application/x-javascript
        application/json
        application/ld+json
        application/manifest+json
        application/rdf+xml
        application/rss+xml
        application/xhtml+xml
        application/xml
        application/wasm
        font/eot
        font/otf
        font/ttf
        image/svg+xml
        text/css
        text/javascript
        text/plain
        text/xml;
}

# Защита от прямого сканирования по IP (сброс рукопожатия)
server {
    listen 443 ssl default_server;
    listen [::]:443 ssl default_server;
    server_name _;
    ssl_reject_handshake on;
}
```

`ssl_reject_handshake` требует Nginx 1.19.4+, `http2 on` требует 1.25.1+ (на старых версиях пишите `listen 443 ssl http2;`). Проверка и применение: `nginx -t && systemctl reload nginx`.

</details>

---

## Подключение узлов (Remnanode)

Для управления удаленными серверами и прокси-ядром Xray используется легковесный агент **Remnanode на Go**:

👉 **[Репозиторий Remnanode-Go](https://github.com/aaaSaZaN/remnanode-go)**

Инструкции по чистой установке агента на ноды и привязке к панели подробно описаны в репозитории ноды.

### Обновление Remnanode Go и Xray Core

В карточке Go-ноды нажмите на версию Xray Core: откроется выбор официального release или pre-release Xray, а также поле для прямой HTTPS-ссылки на бинарник или архив. Версию самого агента выбирайте на вкладке **Remnanode Go** — список берётся из Releases репозитория `aaaSaZaN/remnanode-go`.

Для проверки фактически установленного ядра на сервере ноды выполните `remnanode status` или `/opt/remnanode/rw-core version`.

### Миграция с Docker (Node.js/TS) на Native Go (~0s Downtime)

Если у вас уже работают ноды на оригинальном Docker-образе `remnawave/node`, вы можете перевести их на нативный Go-агент без ручной настройки и потери трафика:

```bash
# Автоматическая миграция одной командой (запускать от root на сервере ноды):
curl -sSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/scripts/migrate-node.sh | bash
```

**Что делает скрипт:**
1. Находит запущенный Docker-контейнер `remnawave-node` (или docker-compose стек).
2. Выгружает боевые mTLS сертификаты, токены, ID профиля и порт.
3. Скачивает скомпилированный бинарник `remnanode` под архитектуру вашей системы (`amd64`, `arm64`, `armv7`, `mips` и др.).
4. Создает systemd-службу `/etc/systemd/system/remnanode.service`.
5. Запускает Go-ноду и останавливает старый Docker-контейнер (простой подключения составляет менее секунды).

---

## Страница подписки (Subscription Page)

**Subscription Page** — это клиентская страница для конечных пользователей (`https://sub.domain.com/<shortUuid>`). Она показывает инструкции по подключению, QR-коды для приложений (Happ, Sing-box, V2Ray, Clash, Shadowrocket) и **скрывает основной домен и IP панели** от блокировок и цензуры.

Страницу подписки можно развернуть двумя способами:
1. **Bundled** — на одном сервере вместе с панелью Gowave.
2. **Separate Server** — на отдельном сервере/VPS (максимальная маскировка и безопасность).

### Подготовка: получение API токена
1. В панели Gowave откройте: **Settings ➔ API Tokens**.
2. Создайте новый токен с правами для страницы подписки и скопируйте его.

---

### Вариант 1: Bundled (На одном сервере с панелью)

1. В файле `/opt/gowave/.env` укажите домен вашей страницы подписок:
   ```env
   SUB_PUBLIC_DOMAIN=sub.yourdomain.com
   ```
   И перезапустите Gowave: `systemctl restart gowave`.
   Страница подписки обращается к панели по её домену: внутри контейнера `127.0.0.1` указывает на сам контейнер, а панель слушает только loopback хоста.

2. Создайте каталог для страницы подписки:
   ```bash
   mkdir -p /opt/gowave-sub && cd /opt/gowave-sub
   ```

3. Создайте файл `docker-compose.yml`:
   ```yaml
   services:
     gowave-subscription-page:
       image: remnawave/subscription-page:latest
       container_name: gowave-subpage
       restart: always
       ports:
         - "127.0.0.1:3010:3010"
       environment:
         - APP_PORT=3010
         - REMNAWAVE_PANEL_URL=https://panel.yourdomain.com
         - REMNAWAVE_API_TOKEN=ВАШ_API_ТОКЕН_ИЗ_ПАНЕЛИ
         - TRUST_PROXY=1
   ```

4. Запустите сервис:
   ```bash
   docker compose up -d
   ```

5. Настройте Reverse Proxy для домена подписок `sub.yourdomain.com`:

   **Для Caddy** (допишите в `/etc/caddy/Caddyfile`):
   ```caddy
   sub.yourdomain.com {
       encode zstd gzip
       reverse_proxy 127.0.0.1:3010
   }
   ```
   *Перезапуск: `systemctl reload caddy`*

   **Для Nginx** (добавьте сервер в `/etc/nginx/sites-available/sub.conf`):
   ```nginx
   server {
       listen 443 ssl http2;
       server_name sub.yourdomain.com;

       ssl_certificate /etc/letsencrypt/live/sub.yourdomain.com/fullchain.pem;
       ssl_certificate_key /etc/letsencrypt/live/sub.yourdomain.com/privkey.pem;

       location / {
           proxy_pass http://127.0.0.1:3010;
           proxy_http_version 1.1;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
           proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
           proxy_set_header X-Forwarded-Proto $scheme;
       }
   }
   ```

---

### Вариант 2: Separate Server (На отдельном сервере)

Для того чтобы клиенты вообще не знали IP-адрес основного сервера панели:

1. На **основном сервере** в `/opt/gowave/.env` укажите:
   ```env
   SUB_PUBLIC_DOMAIN=sub.yourdomain.com
   ```

2. На **отдельном сервере** создайте `docker-compose.yml`:
   ```yaml
   services:
     gowave-subscription-page:
       image: remnawave/subscription-page:latest
       container_name: gowave-subpage
       restart: always
       ports:
         - "127.0.0.1:3010:3010"
       environment:
         - APP_PORT=3010
         - REMNAWAVE_PANEL_URL=https://panel.yourdomain.com
         - REMNAWAVE_API_TOKEN=ВАШ_API_ТОКЕН_ИЗ_ПАНЕЛИ
         - TRUST_PROXY=1
   ```

3. Запустите: `docker compose up -d` и настройте Caddy или Nginx на отдельном сервере, направив домен `sub.yourdomain.com` на `127.0.0.1:3010`.

---

## Сборка из исходников (Для разработчиков)

Если вы хотите собрать Gowave самостоятельно:

```bash
# Клонирование
git clone https://github.com/aaaSaZaN/gowave.git
cd gowave

# Сборка интерфейса
pnpm --prefix frontend build

# Кросс-компиляция бинарников под Linux:
cd backend-go
# Для x86_64:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gowave-linux-amd64 ./cmd/server
# Для arm64:
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o gowave-linux-arm64 ./cmd/server
```

Docker-образ из исходников вместо `ghcr.io`:
```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

---

## Миграция базы данных (PasarGuard и Remnawave)

Установщик (`install.sh`) сам находит Remnawave и PasarGuard и предлагает перенос. Команды ниже нужны для ручного запуска.

Gowave поддерживает быстрый перенос данных в один клик без запуска панели:

### 1. Переезд с PasarGuard (в SQLite или PostgreSQL)
```bash
# Автоматический поиск и импорт базы PasarGuard
gowave import pasarguard

# Или с явным указанием пути к файлу/DSN:
gowave import pasarguard /opt/pasarguard/data/db.sqlite3
```
* Переносит всех пользователей, VLESS UUID, пароли Trojan / Shadowsocks / Hysteria, лимиты трафика, стратегии сброса, заметки и привязки устройств по HWID.
* Автоматически привязывает пользователей к дефолтному скваду нод.
* База находится сама по `SQLALCHEMY_DATABASE_URL` из `/opt/pasarguard/.env` (SQLite, PostgreSQL или TimescaleDB, в том числе через pgbouncer) или по файлу `/var/lib/pasarguard/db.sqlite3`. MySQL/MariaDB не поддерживаются.

### 3. Переезд с GoWave SQLite (Systemd) ➔ GoWave в Docker (PostgreSQL)
Если панель установлена бинарником с SQLite, а вы хотите перейти на Docker Compose с PostgreSQL. Нужен установленный Docker с плагином compose (`curl -fsSL https://get.docker.com | sudo sh`), запускать от root на сервере панели:
```bash
gowave migrate to-docker

# Или через спасательное меню (пункт 9):
gowave rescue
```
Команда сама:
1. Найдёт текущую установку (`/opt/gowave`) и её `.env`, проверит SQLite на битые ссылки между таблицами.
2. Создаст `/opt/gowave-docker` с `docker-compose.yml` и `.env`: перенесёт **тот же** `APP_SECRET` (без него не войти в панель) и все настройки, сгенерирует пароль PostgreSQL.
3. Скачает образы, запустит PostgreSQL, остановит `gowave.service` и перенесёт все данные.
4. Запустит панель в Docker на том же порту и дождётся, пока она ответит.
5. Отключит `gowave.service`; команда `gowave` дальше выполняется внутри контейнера.

Если любой шаг не удался, Docker-проект удаляется, а старая панель запускается обратно. Файл SQLite не изменяется, поэтому откатиться можно и позже:
```bash
cd /opt/gowave-docker && docker compose down
ln -sf /opt/gowave/gowave-server /usr/local/bin/gowave
systemctl enable --now gowave
```
Флаги: `--dir <путь>` (папка проекта), `--image <образ>` (например, собранный локально), `--from <путь>` (папка старой установки), `--yes` (без подтверждения).

---

### 2. Переезд с Remnawave (Docker PostgreSQL ➔ Gowave SQLite)
Если вы хотите избавиться от тяжелого Docker и перейти на легкий одиночный файл SQLite:
```bash
# Перенос данных из PostgreSQL в автономный файл SQLite (по умолчанию gowave-migrated.sqlite)
gowave migrate pg-to-sqlite
```
* Автоматически находит настройки подключения к PostgreSQL старой Remnawave (`/opt/remnawave/.env`).
* Переносит все таблицы без потерь (пользователей, трафик, ноды, хосты, reality-шаблоны, сквады).
* Исходную базу и `.env` не меняет: укажите полученный файл в `DATABASE_URL` с `DB_DRIVER=sqlite` и перезапустите GoWave. Установщик (`install.sh`) делает это сам.
* При использовании интерактивного меню (`gowave rescue`) вежливо предложит остановить и удалить старые Docker-контейнеры (`remnawave`, `remnawave-db`, `remnawave-redis`), освободив оперативку и диск (строго по подтверждению `y`).

---

## Утилиты и CLI команды (Rescue Mode)

При первом запуске бинарник сам регистрирует глобальную команду `gowave` в системе (`/usr/local/bin/gowave`), поэтому команды можно вызывать из любой папки.

```bash
# Интерактивное меню управления и миграций
gowave rescue
```

Команды прямого вызова:

| Команда | Описание |
| :--- | :--- |
| `gowave import pasarguard [path]` | Быстрый импорт данных из PasarGuard |
| `gowave migrate pg-to-sqlite [dsn]` | Перенос данных из Remnawave PostgreSQL в Gowave SQLite |
| `gowave migrate to-docker` | Переезд установки с SQLite в Docker + PostgreSQL |
| `gowave migrate sqlite-to-pg [src] [dsn] [--force]` | Копирование данных из Gowave SQLite в уже запущенный PostgreSQL |
| `gowave rescue --reset-admin` | Удаляет администраторов для повторного прохождения первичной настройки в браузере |
| `gowave rescue --list-admins` | Выводит список всех зарегистрированных администраторов |
| `gowave rescue --enable-password-auth` | Принудительно включает аутентификацию по логину/паролю в БД |
| `gowave rescue --print-secret-key` | Генерирует и выводит валидный `SECRET_KEY` для нод |
| `gowave rescue --help` | Справка по всем доступным флагам |

---

## Переменные окружения (Environment Variables)

Все параметры задаются в файле `.env` в каталоге с бинарником или передаются через окружение ОС:

| Переменная | По умолчанию | Описание |
| :--- | :--- | :--- |
| `APP_HOST` | `127.0.0.1` | Адрес, на котором слушают панель и метрики. Наружу панель отдаётся только через reverse proxy с HTTPS; `0.0.0.0` нужен лишь внутри контейнера |
| `APP_PORT` | `3000` | HTTP-порт веб-панели и API |
| `METRICS_PORT` | `3001` | Порт внутренней телеметрии и метрик |
| `APP_SECRET` | *(обязательно)* | Секретный ключ (минимум 32 символа) для подписи JWT и хешей |
| `DB_DRIVER` | `sqlite` | Драйвер БД: `sqlite` или `postgres` |
| `DATABASE_URL` | `gowave.db` | Путь к файлу SQLite или URL подключения PostgreSQL |
| `PANEL_DOMAIN` | `localhost:3000` | Домен панели для генерации ссылок |
| `SUB_PUBLIC_DOMAIN` | `localhost:3000/api/sub` | Публичный URL точки входа пользовательских подписок |
| `FRONT_END_DOMAIN` | `*` | Разрешенные домены для CORS |
| `JWT_AUTH_LIFETIME` | `12` | Срок действия JWT сессии администратора (в часах) |

---

## Документация и API

Gowave сохраняет полную совместимость с экосистемой Remnawave:

- **API & Спецификация**: Все эндпоинты, форматы запросов, DTO и вебхуки полностью идентичны оригинальному API.
- **Инструкции по настройке нод и клиентов**: Руководства по подключению протоколов (VLESS, Shadowsocks, Trojan, Hysteria2), работе с плагинами и устранению любых проблем доступны в официальной документации: **[docs.rw](https://docs.rw)**. В будущем , конечно , может быть будет своя собственная документация Gowave , но это как то потом.

---

## Автор проекта

- Разработка и портирование на Go (**Gowave**): [@aaaSaZaN](https://github.com/aaaSaZaN)
- Оригинальный проект: [Remnawave](https://github.com/remnawave)

---

## Лицензия

Проект распространяется под лицензией **GNU Affero General Public License v3.0 (AGPL-3.0-only)** в соответствии с лицензионными требованиями кодовой базы Remnawave.
