# GoWave

Бэкенд [Remnawave](https://github.com/remnawave), переписанный на Go. Тот же API и веб-интерфейс, но памяти нужно примерно в 20 раз меньше, а запросов в секунду он выдерживает в разы больше.

**Документация: [aaasazan.github.io/gowave-docs](https://aaasazan.github.io/gowave-docs/)** · [English](https://aaasazan.github.io/gowave-docs/en/)

| | GoWave | Remnawave |
| :--- | :--- | :--- |
| Память под нагрузкой | около 50 МБ | около 1 ГБ |
| API готов после старта | 0,2 с | 4,8 с |
| Процессы | один | три, плюс Valkey |
| База данных | SQLite или PostgreSQL | PostgreSQL |

Замеры и методика - на странице [бенчмарков](https://aaasazan.github.io/gowave-docs/benchmarks/).

## Установка

Нужны Linux-сервер (Debian, Ubuntu или RHEL-семейство, `amd64`/`arm64`) и домен, A-запись которого указывает на этот сервер.

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/aaaSaZaN/gowave/main/install.sh)"
```

Установщик спросит базу данных (SQLite или PostgreSQL в Docker), reverse proxy (Caddy, Nginx или свой) и домен, затем настроит HTTPS. Если на сервере есть Remnawave или PasarGuard, он предложит перенести данные. Повторный запуск той же команды открывает меню: обновление, смена домена, удаление.

Первый, кто откроет панель, создаёт администратора, поэтому откройте её сразу после установки.

## Документация

| Тема | Страница |
| :--- | :--- |
| С чего начать | [Быстрый старт](https://aaasazan.github.io/gowave-docs/overview/quick-start/), [требования](https://aaasazan.github.io/gowave-docs/install/requirements/) |
| Установка | [установщик](https://aaasazan.github.io/gowave-docs/install/installer/), [Docker и PostgreSQL](https://aaasazan.github.io/gowave-docs/install/docker/), [systemd и SQLite](https://aaasazan.github.io/gowave-docs/install/systemd/), [обновление](https://aaasazan.github.io/gowave-docs/install/updating/) |
| Reverse proxy | [Caddy](https://aaasazan.github.io/gowave-docs/install/reverse-proxy/caddy/), [Nginx](https://aaasazan.github.io/gowave-docs/install/reverse-proxy/nginx/) |
| Ноды | [Go-нода](https://aaasazan.github.io/gowave-docs/install/node/install/) ([remnanode-go](https://github.com/aaaSaZaN/remnanode-go)), [переход с Docker-ноды](https://aaasazan.github.io/gowave-docs/migration/docker-node/) |
| Страница подписки | [Установка](https://aaasazan.github.io/gowave-docs/install/subscription-page/) |
| Переезд | [с Remnawave](https://aaasazan.github.io/gowave-docs/migration/remnawave/), [с PasarGuard](https://aaasazan.github.io/gowave-docs/migration/pasarguard/), [с SQLite в Docker](https://aaasazan.github.io/gowave-docs/migration/sqlite-to-docker/), [переход на PostgreSQL 18](https://aaasazan.github.io/gowave-docs/migration/postgres-18/) |
| Подписки | [Response Rules (SRR)](https://aaasazan.github.io/gowave-docs/features/subscription-rules/), [шаблоны](https://aaasazan.github.io/gowave-docs/features/templates/) |
| Справочник | [переменные окружения](https://aaasazan.github.io/gowave-docs/reference/environment/), [CLI](https://aaasazan.github.io/gowave-docs/reference/cli/), [совместимость](https://aaasazan.github.io/gowave-docs/reference/compatibility/), [API](https://aaasazan.github.io/gowave-docs/api/) |

## Совместимость

GoWave реализует все 218 команд `@remnawave/backend-contract` 3.4.15. Тесты сверяют маршруты с контрактом и проверяют ответы по его JSON-схемам на SQLite и PostgreSQL, а выдачу подписок сравнивают с эталоном оригинального бэкенда. Работают веб-интерфейс Remnawave, ноды `remnawave/node` и [Go-нода](https://github.com/aaaSaZaN/remnanode-go), официальная страница подписки.

Главное отличие: регулярные выражения в правилах и шаблонах используют синтаксис RE2 (Go), а не JavaScript. Подробнее - в [сравнении с Remnawave](https://aaasazan.github.io/gowave-docs/overview/comparison/).

## Разработка

```bash
git clone https://github.com/aaaSaZaN/gowave.git && cd gowave
pnpm --dir frontend install --frozen-lockfile && pnpm --dir frontend build
cd backend-go && go test ./... && CGO_ENABLED=0 go build -o gowave ./cmd/server
```

Проверки контракта, тесты на PostgreSQL и сборка Docker-образа описаны на странице [Сборка из исходников](https://aaasazan.github.io/gowave-docs/development/) и в [AGENTS.md](AGENTS.md).

## Автор

- GoWave: [@aaaSaZaN](https://github.com/aaaSaZaN)
- Оригинальный проект: [Remnawave](https://github.com/remnawave)

## Лицензия

[AGPL-3.0-only](LICENSE), как и кодовая база Remnawave.
