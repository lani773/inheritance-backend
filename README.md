# INHERITANCE CHOIR — Go Gin Enterprise Backend

> **Advanced Backend Series — Task 4 (Go Gin)**
> - ✅ Task 1: FastAPI + MongoDB (Python)
> - ✅ Task 2: Node.js/Express + MongoDB
> - ✅ Task 3: Celery Automation Engine
> - ✅ **Task 4: Go/Gin Enterprise Backend** ← *this project*

---

## ⚡ Why Go for Enterprise?

| Metric | Go Gin | FastAPI | Node.js | Express |
|---|---|---|---|---|
| **Binary size** | ~10 MB static | ~200 MB env | ~150 MB | ~150 MB |
| **Memory idle** | ~15 MB | ~80 MB | ~60 MB | ~60 MB |
| **Memory @ 10k RPS** | ~50 MB | ~300 MB | ~200 MB | ~200 MB |
| **Cold start** | ~10 ms | ~2 s | ~500 ms | ~500 ms |
| **Max throughput** | ~150k RPS | ~30k RPS | ~50k RPS | ~50k RPS |
| **Worker pool** | ✅ Built-in goroutines | ❌ Needs Celery | ❌ Needs Bull | ❌ |
| **Type safety** | ✅ Compile-time | Partial | ❌ Runtime | ❌ |
| **Docker image** | ~10 MB (scratch) | ~500 MB | ~300 MB | ~300 MB |
| **Goroutines** | Millions (2KB each) | Threads limited | Event loop | Event loop |

---

## 🏗️ Architecture

```
Go 1.22 + Gin + MongoDB Go Driver + JWT + bcrypt + robfig/cron + gomail + zap
```

```
inheritance-choir-go/
├── cmd/server/main.go               ← Entry + graceful shutdown (30s)
├── go.mod                           ← All dependencies
├── Dockerfile                       ← Multi-stage → ~10MB scratch image
├── docker-compose.yml
├── .env.example
├── scripts/seed.go                  ← Seed admin + sample data
│
├── internal/
│   ├── config/config.go             ← Viper type-safe settings
│   ├── models/models.go             ← 15 MongoDB document types
│   ├── repository/mongo.go          ← Connection pool + all indexes
│   │
│   ├── api/v1/
│   │   ├── router/router.go         ← 87 Gin routes
│   │   ├── middleware/middleware.go  ← Auth, Admin, RateLimit, Logger, Recovery
│   │   └── handlers/
│   │       ├── auth_handler.go      ← 9 auth routes
│   │       ├── member_handler.go    ← 10 member routes + QR PNG
│   │       ├── contribution_handler.go ← 10 contribution routes
│   │       ├── event_attendance_handler.go ← 16 event + attendance routes
│   │       ├── other_handlers.go    ← 42 remaining routes
│   │       └── helpers.go           ← bindJSON, sendSuccess, paginate
│   │
│   ├── services/auth_service.go     ← Login, register, OTP, token rotation
│   ├── workers/pool.go              ← Goroutine worker pool (no Redis needed)
│   ├── scheduler/scheduler.go       ← 9 cron jobs (robfig/cron)
│   ├── notifications/mailer.go      ← HTML email via SMTP (gomail)
│   └── utils/format.go              ← MemberPublic DTO formatter
│
└── pkg/
    ├── jwt/jwt.go                   ← Token signing, OTP, receipt generators
    └── crypto/crypto.go             ← bcrypt hash/verify
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- MongoDB 6.0+
- (Optional) Redis for rate limiting

### Run locally
```bash
cd inheritance-choir-go
cp .env.example .env
# Edit .env — set JWT_SECRET (min 32 chars), MONGODB_URI, MAIL_* credentials

# Seed database
go run ./scripts/seed.go

# Run server
go run ./cmd/server

# Visit http://localhost:8080/health
```

### Build production binary
```bash
CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-s -w" \
  -trimpath \
  -o choir-server \
  ./cmd/server

# Single ~10MB static binary — deploy anywhere
./choir-server
```

### Docker
```bash
docker-compose up -d
docker-compose exec api go run ./scripts/seed.go
# API: http://localhost:8080
# DB UI: http://localhost:8083
```

---

## 🔑 Default Credentials

| Role  | Email                      | Password    |
|-------|----------------------------|-------------|
| Admin | inheritancechoir@gmail.com | Umurage123. |
| Member| marie@choir.rw             | Password123.|

---

## 📡 All 87 Endpoints

| Group | Routes | Highlights |
|---|---|---|
| `/auth` | 9 | JWT + refresh rotation, OTP, bcrypt |
| `/members` | 10 | CRUD, QR PNG, CSV import, Excel export |
| `/contributions` | 10 | Verify, bulk-verify, Excel/PDF export |
| `/events` | 8 | Calendar, upcoming, attendance per event |
| `/attendance` | 8 | Bulk-mark, QR check-in, excuse workflow |
| `/messages` | 5 | Inbox/sent/broadcast, emoji reactions |
| `/analytics` | 6 | Dashboard, YoY, audit log |
| `/welfare` | 5 | Cases + timeline entries |
| `/posts` | 7 | CRUD, likes, comments |
| `/songs` | 5 | CRUD, search |
| `/notifications` | 4 | List, mark-read, mark-all, delete |
| `/settings` | 2 | Get/update app settings |
| `/admin` | 6 | Registrations, broadcast, session revoke |

---

## ⚙️ Goroutine Worker Pool

The `internal/workers/pool.go` implements an enterprise-grade fixed-size goroutine pool:

```go
pool := workers.New(20, 500, log)  // 20 workers, 500-task queue

pool.Submit(workers.Task{
    Name:    "send-monthly-reminder",
    Retries: 2,
    Handler: func(ctx context.Context, _ interface{}) error {
        // runs in a pooled goroutine
        return mailer.SendMonthlyTitheReminder(email, name, month)
    },
})
```

Features:
- **Fixed memory** — no goroutine spawning per request
- **Backpressure** — returns false when queue is full
- **Retry** — exponential backoff (1s, 2s, 4s…)
- **Metrics** — atomic counters for processed/failed/retried
- **Graceful drain** — waits for in-flight tasks on shutdown

---

## 🕐 9 Scheduled Jobs

| Job | Schedule | Description |
|---|---|---|
| Monthly tithe reminder | 5th @ 09:00 | Email non-contributors |
| Weekly attendance alert | Sunday @ 08:00 | At-risk member alerts |
| Event reminders | Every hour | 24h-before mandatory events |
| Nightly stats recalc | 02:00 | Recalculate all member stats |
| Monthly finance report | 1st @ 07:00 | Email report to admins |
| Weekly choir digest | Friday @ 17:00 | Summary to admins |
| Birthday greetings | 08:00 daily | Birthday emails + notifications |
| Token cleanup | Every 6h | Remove expired OTPs + tokens |
| Welcome sequences | Every 5min | Welcome newly approved members |

---

## ⚙️ Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MONGODB_URI` | localhost:27017 | MongoDB connection string |
| `JWT_SECRET` | *(required)* | Min 32 chars |
| `JWT_ACCESS_EXPIRY` | `1h` | Access token TTL |
| `JWT_REFRESH_EXPIRY` | `720h` | Refresh token TTL (30 days) |
| `PORT` | `8080` | HTTP port |
| `WORKER_POOL_SIZE` | `20` | Goroutine workers |
| `TASK_QUEUE_SIZE` | `500` | Task queue depth |
| `ATTENDANCE_THRESHOLD` | `70.0` | At-risk % threshold |
