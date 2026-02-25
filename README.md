# Processing Service (Go)

Microsservico de processamento de video da arquitetura FIAP X.

## O que este servico faz

- Consome eventos `video.process` do RabbitMQ.
- Baixa o video do S3.
- Extrai todos os frames e gera um `.zip`.
- Faz upload do zip para o S3.
- Persiste o job em `processing_jobs` (PostgreSQL).
- Publica eventos `status.update`, `video.completed` e `video.failed`.

## Estrutura atual

- `cmd/processing-service`: bootstrap da aplicacao
- `internal/domain`: regras de dominio e transicoes de status
- `internal/port`: contratos (ports) para adapters
- `internal/adapter`: adapters de Postgres, RabbitMQ, S3 e ffmpeg
- `internal/usecase`: orquestracao do fluxo de processamento
- `internal/worker`: consumo da fila e execucao do use case
- `migrations`: scripts SQL de schema

## Configuracao basica

Variaveis ja suportadas no bootstrap:

- `HTTP_PORT` (default: `8080`)
- `HTTP_READ_TIMEOUT` (default: `10s`)
- `HTTP_WRITE_TIMEOUT` (default: `10s`)
- `HTTP_IDLE_TIMEOUT` (default: `30s`)
- `HTTP_SHUTDOWN_TIMEOUT` (default: `15s`)
- `LOG_LEVEL` (`debug`, `info`, `warn`, `error`)

## Endpoints operacionais

- `GET /health`
- `GET /ready`

## Banco de dados

Executar migration:

```bash
psql "$DATABASE_URL" -f migrations/001_create_processing_jobs.sql
```

## Qualidade e testes

Comandos recomendados:

```bash
make fmt
make test
make test-race
make coverage
```

Sem Makefile:

```bash
go fmt ./...
go test ./...
go test -race ./...
```
