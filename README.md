# HTTP Load Balancer

HTTP-балансировщик нагрузки, реализованный на Go. Принимает входящие HTTP-запросы и распределяет их по пулу бэкенд-серверов с использованием алгоритма round-robin. Поддерживает rate-limiting на основе алгоритма Token Bucket, health checks, graceful shutdown, управление конфигурацией через API, персистентность в Redis и интерактивную документацию через Swagger UI.

## Особенности

- **Балансировка нагрузки**:
  - Алгоритм round-robin для распределения запросов.
  - Автоматическое исключение недоступных бэкендов с возвращением после восстановления.
  - Использование `net/http` для реализации reverse proxy.
- **Rate-Limiting**:
  - Реализация алгоритма Token Bucket для ограничения частоты запросов.
  - Поддержка индивидуальных лимитов для клиентов (по IP).
  - Потокобезопасные операции с минимальными блокировками.
  - Персистентность состояния в Redis.
- **Health Checks**:
  - Периодические проверки состояния бэкендов (каждые 5 секунд по умолчанию).
  - Логирование изменений статуса бэкендов.
- **API для управления**:
  - CRUD-операции для бэкендов (`/api/backends`).
  - Управление глобальными настройками rate-limiting (`/api/ratelimit`).
  - Управление клиентами и их лимитами (`/api/clients`).
  - Структурированные JSON-ответы для ошибок.
- **Swagger UI**:
  - Интерактивная документация API, доступная по `/swagger/index.html`.
- **Конфигурация**:
  - Внешний JSON-файл (`configs/config.json`) для настройки порта, бэкендов, health checks и rate-limiting.
  - Возможность изменения конфигурации без перекомпиляции.
- **Логирование**:
  - Структурированное логирование с использованием `go.uber.org/zap`.
  - Настраиваемый уровень логов через переменную окружения `LOG_LEVEL` (DEBUG, INFO, WARN, ERROR).
- **Graceful Shutdown**:
  - Корректное завершение работы при получении SIGINT/SIGTERM с обработкой текущих запросов.
- **Контейнеризация**:
  - `Dockerfile` для сборки минималистичного образа на базе `alpine:3.20`.
  - `docker-compose.yml` для развертывания балансировщика, Redis и тестовых бэкендов.
  - Использование `golang:1.24.3` для устранения критических и высоких уязвимостей.
- **Тестирование**:
  - Интеграционные тесты с использованием `go test -race`.
  - Нагрузочное тестирование через Apache Bench (`ab`).

## Требования

- **Go**: 1.24.3
- **Docker** и **Docker Compose**
- **Python 3** (для генерации `docker-compose.yml`)
- **Redis** (для персистентности rate-limiting)
- **curl** или **Apache Bench** (для тестирования)

## Установка

1. Склонируйте репозиторий:
   ```bash
   git clone https://github.com/Levan1e/load-balancer
   cd load-balancer
2. Установите зависимости GO:
   ```bash
   go mod tidy

## Запуск
При отсутствии docker-compose.yml сгенерируйте скриптом:
```bash
  python generate_docker_compose.py`
```
Запустите сервисы:
   ```bash
   docker-compose up --build
  ```
  для windows (необходимо, если во время предыдущей сессии были добавлены новые backend): 
   ```bash
   docker-compose up --build
   ```
  для linux: 
   ```bash
   d./start.sh
   ```
Балансировщик будет доступен по адресу:
   ```bash
   http://localhost:8087
  ```
## API эндпоинты

API документировано через Swagger UI, доступно по 
```bash
http://localhost:8087/swagger/index.html
```
## Основные эндпоинты:

### GET /: Пересылает запросы на здоровый бэкенд (round-robin).
### GET/POST/DELETE /api/backends: Управление бэкендами.
- GET: Возвращает список бэкендов.
- POST: Добавляет новый бэкенд. пример:
```
{"url": "http://backend3:80"}
```
- DELETE: Удаляет бэкенд (параметр url в query).
### PATCH /api/ratelimit: Обновление глобальных настроек rate-limiting (пример:
```
{"capacity": 100, "rate": 10}
```
### GET/POST/DELETE /api/clients: Управление клиентами.
- GET: Возвращает список клиентов.
- POST: Добавляет нового клиента (пример:
```
{"client_id": "192.168.1.1", "capacity": 50, "rate": 5}
```
- DELETE: Удаляет клиента (параметр client_id в query).

## Примеры запросов

1. Получить список бэкендов:
   ```
   curl http://localhost:8087/api/backends
   ```
2. Добавить новый бэкенд:
  ```
  curl -X POST http://localhost:8087/api/backends -H "Content-Type: application/json" -d '{"url": "http://backend3:80"}'
  ```
3. Обновить глобальный rate-limit:
   ```
   curl -X PATCH http://localhost:8087/api/ratelimit -H "Content-Type: application/json" -d '{"capacity": 100, "rate": 10}'
   ```
4. Добавить клиента:
   ```
   curl -X POST http://localhost:8087/api/clients -H "Content-Type: application/json" -d '{"client_id": "192.168.1.1", "capacity": 50, "rate": 5}'
   ```

## Конфигурация

Конфигурация задается в configs/config.json. Пример:
```
{
  "port": ":8087",
  "backends": [
    {"url": "http://backend1:80", "healthy": true},
    {"url": "http://backend2:80", "healthy": true}
  ],
  "health_check_path": "/health",
  "health_check_interval": "5s",
  "rate_limit": {
    "capacity": 50,
    "rate": 5
  },
  "client_configs": [
    {
      "client_id": "192.168.1.1",
      "capacity": 50,
      "rate": 5
    }
  ]
}
```
  - port: Порт для HTTP-сервера.
  - backends: Список бэкендов.
  - health_check_path: Путь для проверки здоровья бэкендов.
  - health_check_interval: Интервал проверки здоровья.
  - rate_limit: Глобальные настройки rate-limiting.
  - client_configs: Индивидуальные настройки rate-limiting для клиентов.

## Логирование:

Логирование реализовано через go.uber.org/zap. Уровень логов задается переменной окружения LOG_LEVEL:
```
DEBUG, INFO (по умолчанию), WARN, ERROR.
```

Пример настройки: 
```bash
export LOG_LEVEL=DEBUG
```
Логи включают:
  - Входящие запросы.
  - Ошибки при обращении к бэкендам.
  - Изменения статуса бэкендов (healthy/unhealthy).
  - Операции CRUD через API.

## Нагрузочное тестирование
```
wsl ab -n 5000 -c 1000 http://localhost:8087/
This is ApacheBench, Version 2.3 <$Revision: 1903618 $>
Copyright 1996 Adam Twiss, Zeus Technology Ltd, http://www.zeustech.net/
Licensed to The Apache Software Foundation, http://www.apache.org/

Benchmarking localhost (be patient)
Completed 500 requests
Completed 1000 requests
Completed 1500 requests
Completed 2000 requests
Completed 2500 requests
Completed 3000 requests
Completed 3500 requests
Completed 4000 requests
Completed 4500 requests
Completed 5000 requests
Finished 5000 requests


Server Software:        nginx/1.27.5
Server Hostname:        localhost
Server Port:            8087

Document Path:          /
Document Length:        124 bytes

Concurrency Level:      1000
Time taken for tests:   3.533 seconds
Complete requests:      5000
Failed requests:        1307
   (Connect: 0, Receive: 0, Length: 1307, Exceptions: 0)
Non-2xx responses:      1307
Total transferred:      1464117 bytes
HTML transferred:       516747 bytes
Requests per second:    1415.24 [#/sec] (mean)
Time per request:       706.595 [ms] (mean)
Time per request:       0.707 [ms] (mean, across all concurrent requests)
Transfer rate:          404.70 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0   17  22.2      5      68
Processing:     1  632 584.2    547    2301
Waiting:        1  628 582.6    544    2272
Total:          2  649 586.0    565    2358

Percentage of the requests served within a certain time (ms)
  50%    565
  66%    862
  75%    961
  80%   1163
  90%   1521
  95%   1816
  98%   1961
  99%   2126
 100%   2358 (longest request)
```
при `config.json`
```
{
  "port": ":8087",
  "backends": [
    "http://backend1:80",
    "http://backend2:80"
  ],
  "health_check_path": "/health",
  "health_check_interval": "5s",
  "rate_limit": {
    "capacity": 1000,
    "rate": 1000
  },
  "client_configs": [
    {
      "client_id": "192.168.1.1",
      "capacity": 100,
      "rate": 100
    },
    {
      "client_id": "192.168.1.2",
      "capacity": 100,
      "rate": 100
    }
  ]
}
```

## Graceful Shutdown

Балансировщик поддерживает graceful shutdown. 
Для остановки отправьте сигнал SIGINT/SIGTERM (например, Ctrl+C). 
Текущие запросы будут обработаны перед завершением.

## Архитектура

Проект организован модульно:

 - `api/`: HTTP-сервер и обработчики эндпоинтов.
  
 - `internal/balancer/`: Логика round-robin.
  
 - `internal/config/`: Парсинг и сохранение конфигурации.
  
 - `internal/health/`: Health checks бэкендов.
  
 - `internal/logger/`: Структурированное логирование.
  
 - `internal/models/`: Структуры данных.
  
 - `internal/proxy/`: Reverse proxy.
  
 - `internal/ratelimiter/`: Rate-limiting (Token Bucket).
  
 - `cmd/balancer/`: Точка входа.

## Дополнительные фичи

- Health Checks: Периодические проверки через HTTP-запросы к `/health` на бэкендах.

- Perсистентность: Состояние rate-limiting сохраняется в Redis.

- Структурированные ошибки: Все ошибки возвращаются в формате JSON.

- Модульность: Код разделен на пакеты, что упрощает тестирование и расширение.

- Потокобезопасность: Использование `sync.RWMutex` и атомарных операций для rate-limiting.
