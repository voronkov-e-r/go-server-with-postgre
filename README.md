# REST API Сервис Управления Книгами (Book API)

Полнофункциональный RESTful API сервис на языке Go для управления каталогом книг в библиотеке. В качестве хранилища используется PostgreSQL с пулом соединений `pgxpool`. Проект включает структурированное логирование (JSON), обработку сбоев, корректное завершение работы (graceful shutdown) и поддержку частичного обновления записей.

---

## 🚀 Основные возможности

* **Полный набор CRUD-операций**: Создание, чтение, обновление и удаление книг.
* **Работа с PostgreSQL**: Высокопроизводительные запросы с использованием драйвера `pgx/v5` и пула соединений.
* **Структурированное логирование**: Использование встроенного пакета `log/slog` в формате JSON.
* **Цепочка Middleware**:
  * `Recovery`: Перехватывает паники, логирует стек вызовов и возвращает ответом `500 Internal Server Error`.
  * `RequestID`: Добавляет уникальный идентификатор запроса (`X-Request-ID`) в контекст и заголовки ответа.
  * `Logging`: Логирует метод, путь, статус ответа, IP-адрес клиента и время выполнения запроса.
* **Graceful Shutdown**: Безопасная остановка сервера по сигналам `SIGINT` и `SIGTERM` с таймаутом завершения контекста.
* **Строгая валидация**: Запрет неизвестных полей при создании книг и гибкая обработка частичных обновлений (`PATCH`/`PUT`).

---

## 🛠 Технологический стек

* **Язык**: Go (1.21+ — используется стандартный роутер `net/http` с параметрами в путях)
* **База данных**: PostgreSQL
* **Драйвер БД**: `github.com/jackc/pgx/v5/pgxpool`

---

## 📁 Структура проекта

* `main.go` — Точка входа: настройка и запуск HTTP-сервера, маршрутизация, конфигурация логера и Graceful Shutdown.
* `store.go` — Слой работы с базой данных (выполнение SQL-запросов к PostgreSQL).
* `middleware.go` — Реализация HTTP-мидлварей (`Recovery`, `RequestID`, `Logging`, обертка `statusWriter`).
* `patch.go` — Логика парсинга и валидации данных для частичного обновления полей книги.

---

## ⚙️ Быстрый старт

### Требования

* Установленный Go версии 1.21 или выше.
* Установленный и запущенный PostgreSQL.

### 1. Подготовка базы данных

Запустите PostgreSQL и создайте базу данных `bookapi`. По умолчанию приложение использует следующую строку подключения:  
`postgres://bookapi:secret@localhost:5432/bookapi?sslmode=disable`

Пример SQL-скрипта для инициализации таблицы:

```sql
CREATE TYPE book_status AS ENUM ('available', 'borrowed', 'lost');

CREATE TABLE books (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    author VARCHAR(255) NOT NULL,
    year INT,
    status book_status DEFAULT 'available',
    tags TEXT[],
    publisher JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### 2. Запуск приложения

Склонируйте репозиторий и подгрузите зависимости:

```bash
go mod init bookapi
go get github.com/jackc/pgx/v5
go mod tidy
```

Запустите сервер:

```bash
go run .
```

Сервер будет доступен по адресу: `http://localhost:8080`.

---

## 📡 Эндпоинты API

| Метод | Путь | Описание |
| :--- | :--- | :--- |
| `GET` | `/books` | Получить список всех книг |
| `GET` | `/books/{id}` | Получить книгу по её `ID` |
| `POST` | `/books` | Добавить новую книгу |
| `PUT` | `/books/{id}` | Обновить данные книги (поддерживает частичное обновление) |
| `DELETE` | `/books/{id}` | Удалить книгу по `ID` |
| `GET` | `/panic` | Тестовый эндпоинт для проверки перехвата паники (Recovery) |
| `GET` | `/slow` | Тестовый эндпоинт длительного запроса (проверка cancellation) |

---

## 💡 Примеры запросов (cURL)

### 1. Создание книги (`POST /books`)

```bash
curl -X POST http://localhost:8080/books \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Язык программирования Go",
    "author": "Алан А. А. Донован, Брайан У. Керниган",
    "year": 2016,
    "status": "available",
    "tags": ["golang", "programming"],
    "publisher": {
      "name": "Вильямс",
      "country": "Россия"
    }
  }'
```

### 2. Получение списка всех книг (`GET /books`)

```bash
curl -i http://localhost:8080/books
```

### 3. Получение информации о книге (`GET /books/1`)

```bash
curl -i http://localhost:8080/books/1
```

### 4. Частичное обновление книги (`PUT /books/1`)

```bash
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{
    "status": "borrowed",
    "tags": ["golang", "programming", "best-seller"]
  }'
```

### 5. Удаление книги (`DELETE /books/1`)

```bash
curl -X DELETE -i http://localhost:8080/books/1
```

---

## ⚙️ Настройки и таймауты

В `main.go` установлены следующие стандартные таймауты HTTP-сервера:

* **ReadTimeout**: `5s` — максимальное время чтения запроса.
* **WriteTimeout**: `10s` — максимальное время записи ответа.
* **IdleTimeout**: `60s` — время ожидания повторного использования соединения.
* **Graceful Shutdown**: `10s` — таймаут на завершение активных запросов при остановке приложения.