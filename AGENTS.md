# AGENTS.md – Инструкции для разработки VPN-сервиса (Go + React + Marzban + Nginx)

## 📌 Общая информация

Этот документ описывает соглашения, архитектурные принципы и автоматизированные процедуры для проекта VPN-сервиса. Он предназначен для разработчиков, AI-агентов и CI/CD-систем.

**Цель**: Создание надёжного, масштабируемого и защищённого VPN-сервиса, использующего VLESS+Reality для обхода блокировок.

**Ключевые компоненты**:

- Backend: Go (Gin)
- Frontend: React (TypeScript) Vite+
- Панель управления: Marzban (с Xray-core)
- Базы данных: PostgreSQL, Redis
- Веб-сервер: Nginx (Termination TLS, статика)
- Оркестрация: Docker Compose

---

## 🏗️ Архитектура системы

```
Пользователь (браузер/клиент)
         │
         ▼
   Nginx (80/443) – SSL, статика, прокси
         │
         ├── → / → React SPA
         ├── → /api/* → Go Backend (Gin)
         └── → /dashboard/* → Marzban Admin
                     │
         Go Backend  ──► Marzban API (только внутри Docker-сети)
                     │
         Marzban     ──► PostgreSQL (пользователи, подписки)
                     ──► Redis (кэш, сессии, rate limiting)
                     ──► Xray-core (VLESS+Reality, порт 443)
```

Все контейнеры запускаются через `docker-compose.yml` с изолированными сетями.

---

## 🧩 Правила разработки

### Backend (Go)

- **Фреймворк**: Gin
- **Структура проекта**:
  ```
  backend/
  ├── cmd/                  # main.go
  ├── internal/
  │   ├── config/           # загрузка .env (viper)
  │   ├── db/               # работа с PostgreSQL (GORM или sqlx)
  │   ├── handlers/         # обработчики HTTP
  │   ├── middleware/       # JWT, CORS, rate limit, CSRF
  │   ├── models/           # структуры данных
  │   ├── services/         # бизнес-логика (интеграция с Marzban)
  │   └── utils/            # хэлперы
  ├── go.mod
  ├── go.sum
  └── Dockerfile
  ```
- **Кодинг-стайл**: Следовать официальным рекомендациям Go (gofmt, golangci-lint).
- **API**: Документировать через Swagger (генерация из комментариев).
- **Тесты**: Писать модульные тесты для сервисов и интеграционные тесты для API.

---

### 🖥️ Frontend (React + TypeScript) на Vite+

- **Стек**: React 18+, TypeScript, Vite+ (установлен глобально).
- **Проверка окружения** (перед созданием проекта):
  ```bash
  vp env doctor
  ```
  Должен показать `✓ All checks passed`, как в вашем примере.

- **Инициализация проекта** (интерактивно):
  ```bash
  vp create
  ```
  В меню выберите:
  - Шаблон: `React`
  - Вариант: `TypeScript`
  - Название проекта: `frontend`

  После этого в текущей папке появится каталог `frontend/` с готовой структурой.

- **Структура** (стандартная для Vite+):
  ```
  frontend/
  ├── src/
  │   ├── api/              # клиент для Go API (axios)
  │   ├── components/       # переиспользуемые UI-компоненты
  │   ├── pages/            # страницы (Login, Dashboard, Profile)
  │   ├── hooks/            # кастомные хуки
  │   ├── store/            # состояние (Redux Toolkit или Context)
  │   ├── utils/            # helpers
  │   ├── App.tsx
  │   ├── main.tsx
  │   └── vite-env.d.ts
  ├── public/
  ├── index.html
  ├── package.json
  ├── tsconfig.json
  ├── vite.config.ts        # конфигурация Vite+ (расширяется при необходимости)
  ├── Dockerfile
  └── nginx.conf            # (опционально для отдачи статики)
  ```

- **Команды разработки** (все через `vp`):
  - `vp dev` – запуск dev-сервера с HMR.
  - `vp build` – сборка продакшен-версии в `dist/`.
  - `vp preview` – локальный предпросмотр собранного приложения.
  - `vp lint` – проверка кода (Oxlint).
  - `vp test` – запуск тестов (Vitest).
  - `vp fmt` – форматирование кода (Oxfmt).

- **Установка дополнительных зависимостей**:
  ```bash
  npm install axios react-router-dom @reduxjs/toolkit react-redux react-hook-form zod
  ```

- **Прокси для API** (в режиме разработки):  
  В `vite.config.ts`:
  ```ts
  export default defineConfig({
    server: {
      proxy: {
        '/api': 'http://localhost:8080', // ваш Go-бэкенд
      },
    },
  });
  ```

- **Деплой статики**: после `vp build` папка `dist/` монтируется в Nginx для раздачи.

- **Безопасность**: JWT – только в HttpOnly cookies, передача через `withCredentials: true` в axios.

---

### Marzban (панель)

- Конфигурация через `.env` в `marzban/.env`.
- Inbound настраивать через API или веб-интерфейс.
- **Reality**: Использовать `"dest": "www.microsoft.com"` (или другой белый сайт).
- **API**: Доступен только внутри Docker-сети для Go-бэкенда.

### Nginx

- **SSL/TLS**: Настройка через Let's Encrypt (certbot) или самоподписанные сертификаты для dev.
- **Безопасные заголовки**: HSTS, CSP, X-Frame-Options, etc.
- **Проксирование**: `/api` → Go Backend, `/dashboard` → Marzban, остальное → React статика.

---

## 🐳 Настройка окружения (Docker Compose)

1. **Клонируйте репозиторий**:

   ```bash
   git clone ...
   cd vpn-service
   ```
2. **Создайте `.env`** (в корне) со следующими переменными:

   ```
   DB_USER=vpn_user
   DB_PASSWORD=secure_password
   DB_NAME=vpn_db
   MARZBAN_ADMIN_PASSWORD=very_strong_password
   JWT_SECRET=your_jwt_secret
   ```

   Также создайте `marzban/.env` (скопируйте из примера).
3. **Запустите сборку и контейнеры**:

   ```bash
   docker-compose up -d --build
   ```
4. **Проверьте статус**:

   ```bash
   docker-compose ps
   ```
5. **Инициализируйте Marzban** (создайте первого администратора, если нужно):

   - Перейдите в веб-интерфейс: `https://your-domain.com/dashboard` (или локально).
6. **Настройте inbound** в Marzban:

   - Создайте Inbound с протоколом **VLESS TCP REALITY**.
   - Сгенерируйте privateKey и publicKey, укажите `dest` и `serverName`.
   - Запишите `inbound_id` (обычно 1) для использования в Go-бэкенде.

---

## 🚀 Процесс деплоя (CI/CD)

Используется GitHub Actions (или аналоги). Сценарий:

1. **Проверка кода**: lint, тесты (Go, React).
2. **Сборка образов**: `docker build` для backend, frontend.
3. **Пуш в реестр** (например, GitHub Container Registry).
4. **Обновление на сервере**:
   - Скопировать `docker-compose.yml` и `.env`.
   - Выполнить `docker-compose pull && docker-compose up -d`.

Для ручного деплоя:

```bash
git pull
docker-compose down
docker-compose up -d --build
```

---

## 🔒 Безопасность – обязательные меры

### Общие

- Все контейнеры запускаются от non-root пользователя (`user: "1000:1000"`).
- Секреты передаются через переменные окружения, не зашиты в образы.
- Использовать `Dockerfile` с многократной сборкой для уменьшения размера и уязвимостей.

### Backend (Go)

- **JWT**: ES256, короткий срок жизни (15 минут), Refresh-токен в HttpOnly cookie.
- **Rate Limiting**: Redis-backed (например, `go-redis/redis_rate`).
- **Валидация**: Обязательная валидация всех входных данных.
- **CORS**: Разрешён только доверенный домен.
- **Логи**: Все попытки авторизации, изменения прав, создание пользователей.

### Frontend (React)

- **CSP**: Настроить заголовки через Nginx.
- **HttpOnly cookies**: Для JWT, не использовать `localStorage`.
- **Обновление зависимостей**: Регулярно `npm audit fix`.

### Marzban

- Изменить путь админки: `DASHBOARD_PATH=/секретный-путь`.
- Использовать PostgreSQL (не SQLite) для production.
- Ограничить доступ к API только для Go-бэкенда (`ports: "127.0.0.1:8000:8000"`).

### Nginx

- **TLS 1.2/1.3**, отключить устаревшие протоколы.
- **HSTS**: max-age=31536000; includeSubDomains; preload.
- **Заголовки безопасности**: добавить все рекомендуемые (см. раздел выше).
- **Ограничение запросов**: `limit_req` и `limit_conn`.

---

## 📊 Мониторинг и логирование

- **Логи**: Все сервисы пишут структурированные логи (JSON) в stdout. Собирать через `docker logs` или централизованно (ELK/Loki).
- **Метрики** (опционально): Prometheus + Grafana для отслеживания:
  - Количество активных пользователей.
  - Использование трафика.
  - Загрузка CPU/памяти контейнеров.
- **Алерты**: Настроить уведомления (Telegram, Email) при ошибках входа, превышении лимитов, падении сервисов.

---

## 🤝 Вклад в проект

1. **Ветки**: `main` – продакшен, `develop` – разработка. Фичи создавать от `develop` с именем `feature/название`.
2. **Pull Request**: Требуется ревью, все тесты должны проходить, линтер без ошибок.
3. **Changelog**: Вести `CHANGELOG.md` с описанием изменений.

---

## 🧪 Тестирование

- **Backend**: Написать интеграционные тесты с использованием тестовой БД.
- **Frontend**: Unit-тесты (Jest) и e2e (Cypress/Playwright) для критических путей.
- **Нагрузочное тестирование**: (опционально) использовать k6 для проверки API под нагрузкой.

---

## 🗂️ Полезные команды

- **Перезапустить все**: `docker-compose restart`
- **Посмотреть логи**: `docker-compose logs -f [service]`
- **Войти в контейнер**: `docker exec -it vpn-backend sh`
- **Обновить сертификаты Let's Encrypt**: `certbot renew` (если не автоматизировано).
