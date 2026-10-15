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

## Быстрый запуск (Готовый релиз)

Вам **не требуется** устанавливать Go, Node.js или собирать проект вручную на сервере - используйте готовые бинарники для архитектур `x86_64` (AMD/Intel) или `arm64` (ARMv8).

### 1. Скачивание файлов релиза

Создайте рабочую директорию на вашем сервере:

```bash
mkdir -p /opt/gowave && cd /opt/gowave
```

#### Для серверов x86_64 (Intel / AMD):
```bash
curl -L -o gowave-server https://github.com/aaaSaZaN/remnawave-go/releases/latest/download/gowave-linux-amd64
chmod +x gowave-server
```

#### Для серверов arm64 / ARMv8:
```bash
curl -L -o gowave-server https://github.com/aaaSaZaN/remnawave-go/releases/latest/download/gowave-linux-arm64
chmod +x gowave-server
```

#### Скачивание готового веб-интерфейса:
```bash
curl -LO https://github.com/aaaSaZaN/remnawave-go/releases/latest/download/gowave-frontend.tar.gz
mkdir -p frontend/dist && tar -xzf gowave-frontend.tar.gz -C frontend/dist
```

---

### 2. Настройка окружения (`.env`)

Сгенерируйте случайный 32-байтный ключ безопасности в терминале:
```bash
openssl rand -hex 32
```
*(Скопируйте выведенную команду строку)*

Затем создайте и откройте файл окружения:
```bash
nano /opt/gowave/.env
```

Вставьте настройки и замените `APP_SECRET` на скопированный ключ:

> [!NOTE]
> Ниже приведены **минимально необходимые параметры** для первого запуска Gowave. Все доступные параметры см. в таблице [Переменные окружения](#переменные-окружения).

```env
APP_PORT=3000
APP_SECRET=вставьте_сюда_ключ_полученный_из_openssl
DB_DRIVER=sqlite
DATABASE_URL=gowave.db
PANEL_DOMAIN=panel.yourdomain.com ## change to your domain panel!
SUB_PUBLIC_DOMAIN=panel.yourdomain.com/api/sub ## change to your domain for ur subcruption!
```
*(Чтобы сохранить файл в nano: нажмите `Ctrl+O` ➔ `Enter`, затем для выхода `Ctrl+X`)*.

---

### 3. Автозапуск через Systemd

Создайте файл службы `/etc/systemd/system/gowave.service`:

```ini
[Unit]
Description=Gowave Panel Service
After=network.target

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

Активируйте и запустите службу:

```bash
systemctl daemon-reload
systemctl enable --now gowave
systemctl status gowave
```

---

## Настройка Reverse Proxy

### Вариант 1: Caddy ( автоматическое SSL и сжатие)

Отредактируйте `/etc/caddy/Caddyfile`:

```caddy
panel.yourdomain.com {
    # энкодинг Zstandard + Gzip
    encode zstd gzip

    reverse_proxy 127.0.0.1:3000
}
```

Примените изменения:
```bash
systemctl reload caddy
```

---

### Вариант 2: Nginx

#### 1. Выпуск бесплатного SSL-сертификата (через Certbot или acme.sh)
Самый простой способ - выпуск сертификата через Certbot:
```bash
apt install -y certbot python3-certbot-nginx
certbot certonly --standalone -d panel.yourdomain.com
```

#### 2. Конфигурация Nginx
Создайте конфиг `/etc/nginx/sites-available/gowave.conf` (замените `panel.yourdomain.com` на ваш домен):

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
    listen 443 ssl reuseport;
    listen [::]:443 ssl reuseport;
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

    ssl_stapling on;
    ssl_stapling_verify on;
    resolver 1.1.1.1 1.0.0.1 8.8.8.8 8.8.4.4 valid=60s;
    resolver_timeout 2s;

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

#### 3. Активация и перезапуск Nginx
```bash
ln -s /etc/nginx/sites-available/gowave.conf /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

---

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
curl -sSL https://raw.githubusercontent.com/aaaSaZaN/remnawave-go/main/scripts/migrate-node.sh | bash
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
         - REMNAWAVE_PANEL_URL=http://127.0.0.1:3000
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
git clone https://github.com/aaaSaZaN/remnawave-go.git
cd remnawave-go

# Сборка интерфейса
pnpm --prefix frontend build

# Кросс-компиляция бинарников под Linux:
cd backend-go
# Для x86_64:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o gowave-linux-amd64 main.go
# Для arm64:
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o gowave-linux-arm64 main.go
```

---

## Миграция базы данных (PasarGuard и Remnawave)

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

### 2. Переезд с Remnawave (Docker PostgreSQL ➔ Gowave SQLite)
Если вы хотите избавиться от тяжелого Docker и перейти на легкий одиночный файл SQLite:
```bash
# Перенос данных из PostgreSQL в автономный remnawave.db
gowave migrate pg-to-sqlite
```
* Автоматически находит настройки подключения к PostgreSQL старой Remnawave (`/opt/remnawave/.env`).
* Переносит все таблицы без потерь (пользователей, трафик, ноды, хосты, reality-шаблоны, сквады).
* Сам переключает `.env` на `DB_DRIVER=sqlite` (сохраняя бэкап `.env.bak`).
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
