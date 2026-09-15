# money-services


## Описание
Программа состоит из 4 связанных микросервисов:
  * **gw-exchanger** - для получения текущего курса обмена валюты
  * **gw-currency-wallel** - кошелёк-обменник с авторизацией
  * **gw-notification** - сохраняет крупные денежные переводы
  * **gw-analytics** - для аналитики денежных операций

Сервисы gw-currency-wallet и gw-exchanger общаются по ппотоколу gRPC.

Сервис gw-currency-wallet отправляет данные о транзакциях в брокер сообщений Kafka.

Сервисы gw-notification и gw-analytics читают сообщения из Kafka.

## Предварительные требования
Для начала работы над проектом вам понадобятся:
  * Docker Engine v24.0+
  * Docker Compose v2.20+

## 🚀 Быстрый запуск
Выполните следующие действия, чтобы запустить все приложения локально:

1. Клонируйте репозиторий:
```bash
  git clone https://github.com/Lirikman/money-services
  cd money-services
```

2. Запустите проект:
```bash
  make run
```

3. Проверьте работу:
   
После успешного запуска приложение будет доступно по адресу:
   👉 http://127.0.0.1:8080/api/v1

Документация swagger для REST-API:
  👉 http://localhost:8080/api/v1/swagger/index.html

Kafka UI будет доступен по адресу:
  👉 http://127.0.0.1:8081


4. Остановка контейнеров и приложения:
```bash
  make stop
``` 


## Микросервис "GW-EXCHANGER"
Сервис для хранения и получения атуального курса валют.
Сервис обрабатывает запросы на получение курса валют по gRPC.

Сервис хранит курсы следующей валюты:
  * Рубль (RUB)
  * Доллар (USD)
  * Евро (EUR)

### 🛠️ Технологический стек
  * Язык: Go (Golang)
  * База данных: PostgreSQL
  * Миграции БД: golang-migrate
  * RPC: gRPC + Protocol Buffers (v3)
  * Логирование: log/slog

## gRPC и Генерация кода (Protobuf)

Проект использует протокол **gRPC** для межсервисного взаимодействия. Схемы данных и эндпоинты описаны в файлах `.proto`.

### Системные требования

Для генерации кода вам понадобятся:
1. Компилятор `protoc`(Инструкция по установке - https://grpc.io)
2. Плагины Go для `protoc`, установите их командой:
```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Генерация и удаление кода
Запустите команду компиляции из корня проекта. Она сгенерирует Go-структуры и gRPC-клиенты/серверы:
```bash
  make generate
```

Для удаления сгенерированных файлов используйте команду:
```bash
  make clean
```

### Запуск приложения (без Docker)
Для запуска приложения локально для тестерования и отладки, необходимо:
1. Подготовить и запусистить базу данных PostgeSQL (локалько либо в контейнере)
2. Настроить переменные окружения для Вашей БД в файле conf.env (`money-services/cmd/gw-exchanger`)
3. Запустить приложение с помощью команды:
```bash
  make run-svc1
```

## Микросервис "GW-CURRENCY-WALLET"
Сервис для регистрации и авторизации пользоватлей, управления кошельком, а также обмена валют.

Севрис отправляет сообщения в брокер Kafka:
* топик "large-transfers" для сервиса сохранения крупных переводов
* топик "wallet-transactions" для сервиса аналитики

Поддерживаемые операции:
* регистрация пользователя
* авторизация пользователя
* получение баланса кошелька
* пополнение кошелька
* вывод средств с кошелька
* обмен валюты
* получение текущего курса обмена валют

Поддерживаемая валюта:
* Рубль (RUB)
* Доллар (USD)
* Евро (EUR)

### 🛠️ Технологический стек
* Язык: Go (Golang)
* REST API: Стандартный пакет net/http
* База данных: PostgreSQL
* Миграции БД: golang-migrate
* RPC: gRPC + Protocol Buffers (v3)
* Авторизация: JWT (JSON Web Tokens)
* Логирование: log/slog

### 🔒 Авторизация и безопасность
Сервис защищен авторизацией на базе JWT.

**Схема работы:**
Клиент отправляет запрос на /api/v1/login (REST).
Сервис проверяет данные и возвращает access_token.
Все остальные эндпоинты требуют передачи токена.

**Как передавать токен:**
В REST: Через HTTP-заголовок Authorization: Bearer <ваш_токен>. 
Обрабатывается через Middleware.

### Запуск приложения (без Docker)
Для запуска приложения локально для тестерования и отладки, необходимо:
1. Подготовить и запусистить базу данных PostgeSQL (локалько либо в контейнере)
2. Настроить переменные окружения для Вашей БД и сервера gRPC в файле conf.env (`money-services/cmd/gw-сurrency-wallet`)
3. Запустить приложение с помощью команды:
```bash
  make run-svc2
```
**ПРИМЕЧАНИЕ:** Без запуска приложения "gw-exchanger" недосутпны будут операции получения курса валют и обмена.

### 📖 API Документация (Swagger)
* Интерактивный UI: После запуска сервера перейдите по адресу http://localhost:8080/api/v1/swagger/index.html для тестирования эндпоинтов прямо из браузера.
* Генерация документации: Для генерации аннотаций используется утилита swag.
Запуск генерации: 
```bash
make swag-init
```

Все запросы отпреаляются на базовый URL:
http://127.0.0.1:8080/api/v1

Заголовки: Content-Type: application/json

### Регистрация нового пользователя
Создаёт и сохраняет нового пользователя кошелька.
Проверяется уникальность имени пользователя и адреса электронной почты.

Требования:
- имя пользователя не пустое, длина от 3 до 30 символов;
- длина пароля не менее 8 символов;
- пароль должен содержать прописные, заглавные букву, а также специальные символы;
- электронная почта должна иметь корректный формат (example@mail.ru)

**POST** /register

**Пример тела запроса:**
```json
{
  "email": "petrov_ivan2020@ya.ru",
  "password": "SuPer!Sec12reT00pas",
  "username": "petrov_20"
}
```

**Пример ответа:**
```json
{
  "message": "User registered successfully"
}
```
• Успех: 201 Created  

или

```json
{
  "error": "Username or email already exists"
}
```
• Ошибка: 400 Bad Request  

или

```json
{
  "error": "invalid email format"
}
```
• Ошибка: 400 Bad Request  

### Авторизация пользователя
В случае успешной авторизации пользователь получает JWT-токен для последующих запросов.

**POST**  /login 
   
**Пример тела запроса:**
```json 
{  
 "username": "petrov_20",  
 "password": "SuPer!Sec12reT00pas"  
} 
```
   
**Пример ответа:**
```json

{  
   "token": "JWT_TOKEN"  
} 
```   
• Успех: 200 OK  

или

```json
{  
   "error": "Invalid username or password"  
}
``` 
• Ошибка: 401 Unauthorized  

### Получение баланса пользователя
Возвращает баланс кошелька пользователя

**GET**  /balance 

Заголовки: *Authorization: Bearer JWT_TOKEN*  

**Пример ответа:**
```json
{
  "balance": {
    "EUR": "100.00",
    "RUB": "3000.00",
    "USD": "500.00"
  }
}
```
• Успех: 200 OK  

или

```json
{
  "Invalid token"
}
```
• Ошибка: 401 Unauthorized

### Пополнение счета
Пополняет кошелёк пользователя на определённую сумму и валюту.
Проверяется корректность суммы и валюты.  
Поддерживаемая валюта: USD, RUB, EUR

**POST** /wallet/deposit

Заголовки: *Authorization: Bearer JWT_TOKEN*

**Пример тела запроса:**
```json
{  
   "amount": 100.00,  
   "currency": "USD"  
}  
```

**Пример ответа:**
```json
{
  "message": "deposit successful or withdrawal successful",
  "new_balance": {
    "EUR": "100.00",
    "RUB": "3000.00",
    "USD": "500.00"
  }
}
```
• Успех: 200 OK 

или

```json
{  
 "error": "Invalid amount or currency"  
}  
```
• Ошибка: 400 Bad Request

### Вывод средств
Позволяет пользователю вывести средства со своего счета.
Проверяется наличие достаточного количества средств и корректность суммы.
Поддерживаемая валюта: USD, RUB, EUR

**POST** /wallet/withdraw

Заголовки: *Authorization: Bearer JWT_TOKEN*

**Пример тела запроса:**
```json
{  
   "amount": 2200.00,  
   "currency": "RUB"  
}  

**Пример ответа:**
```json
{
  "message": "deposit successful or withdrawal successful",
  "new_balance": {
    "EUR": "100.00",
    "RUB": "800.00",
    "USD": "500.00"
  }
}
```
• Успех: 200 OK 

или

```json
{  
  "error": "Insufficient funds or invalid amount"  
}
```
• Ошибка: 400 Bad Request

### Получение курса валют
Получение актуальных курсов валют из внешнего gRPC-сервиса gw-exchanger.

**GET**  /exchange/rates

Заголовки: *Authorization: Bearer JWT_TOKEN*

**Пример ответа:** 
```json
{  
     "rates":   
     {  
       "USD": "float",  
       "RUB": "float",  
       "EUR": "float"  
     }  
}
```  
• Успех: 200 OK  

или

```json
{  
   "error": "Failed to retrieve exchange rates"
}
```
• Ошибка: 500 Internal Server Error

### Обмен валют
Производит обмен валюты с учётом текущего курса обмена валюты.
Курс валют берётся по данным сервиса exchange.
Проверяется наличие средств для обмена, и обновляется баланс пользователя.

**POST**  /exchange
   
Заголовки: *Authorization: Bearer JWT_TOKEN*

**Пример тела запроса:**
```json  
{  
   "from_currency": "USD",  
   "to_currency": "EUR",  
   "amount": 100.00  
}  
```

**Пример ответа:**   
```json
{  
   "message": "Exchange successful",  
   "exchanged_amount": 85.00,  
   "new_balance":  
   {  
     "USD": 0.00,  
     "EUR": 85.00  
   }  
}
```  
• Успех: 200 OK  

или

```json
{  
   "error": "Insufficient funds or invalid currencies"  
}
```
• Ошибка: 400 Bad Request


## Микросервис "GW-NOTIFICATION"
Сервис, который получает запрос на перевод больших денежных сумм по Kafka для дальшейшей обработки и сохранения истории операций.
Сервис сохраняет переводы на сумму свыше 30 000 (руб, дол, евро) в MongoDB.

### 🛠️ Технологический стек
* Язык: Go (Golang)
* База данных: MongoDB
* Миграции БД: golang-migrate
* Kafka: segmentio/kafka-go
* Логирование: log/slog

### Работа с базой данных
Для получения информации о сохраннёных переводах необходимо:

1. Подключиться к базе данных MongoDB c помощью команды:
```bash
  docker exec -it mongo mongosh -u root -p secret --authenticationDatabase admin
```
2. Переключиться на базу "notification":
```bash
  use notification
```
3. Получить все записи из коллекции "transactions":
```bash
    db.transactions.find()
```

**Пример вывода:**
```json
{
    {
    _id: ObjectId('6aa82c0c07b9a3808c2dd069'),
    transaction_id: '01a0a0ec-1093-77f1-a6f2-2fd010ad1b6e',
    user_id: '1',
    operation: 'deposit',
    amount: 363000,
    currency: 'EUR',
    created_at: ISODate('0001-01-01T00:00:00.000Z')
    },
    {
    _id: ObjectId('6aa82db607b9a3808c2dd06a'),
    transaction_id: '01a0a0f2-8f7d-713d-92e2-a3146ee5f2e6',
    user_id: '1',
    operation: 'withdraw',
    amount: 30000,
    currency: 'RUB',
    created_at: ISODate('0001-01-01T00:00:00.000Z')
    }
}
```

## Микросервис "GW-ANALYSTICS"
Сервис, получает уведомление от кошелька через брокер сообщений Kafka и сохраняет данные в Clickhouse. 
Режим доставки: at-least-once с реализацией идемпотентности.

### 🛠️ Технологический стек
* Язык: Go (Golang)
* База данных: ClickHouse
* Миграции БД: golang-migrate
* Kafka: segmentio/kafka-go
* Логирование: log/slog

### Аналитические запросы
Поскольку события хранятся в ClickHouse, большая часть аналитики получается обычными SQL-запросами.

**Количество событий по типам:**
```sql
SELECT
    operation,
    count() AS total
FROM analytics.transaction_events FINAL
GROUP BY operation
ORDER BY total DESC;
```
**Результат:**
deposit     125000
withdraw     87000
exchange     43000
