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

1. Copie o arquivo de exemplo:

```bash
cp .env.example .env
```

2. Ajuste as variaveis obrigatorias para seu ambiente:

- `POSTGRES_URL`
- `RABBITMQ_URL`
- `S3_REGION`
- `S3_BUCKET`
- `WORKER_FFMPEG_BIN` (se `ffmpeg` nao estiver no PATH)

Para ambiente S3 compativel (ex.: MinIO/LocalStack), configure tambem:

- `S3_ENDPOINT`
- `S3_ACCESS_KEY_ID`
- `S3_SECRET_ACCESS_KEY`
- `S3_USE_PATH_STYLE=true`

## Endpoints operacionais

- `GET /health`
- `GET /ready`

`/health` valida somente liveness do processo.  
`/ready` valida prontidao real de dependencias criticas (PostgreSQL e RabbitMQ).

## Banco de dados

Executar migration:

```bash
psql "$POSTGRES_URL" -f migrations/001_create_processing_jobs.sql
```

## Execucao local

Com `.env` configurado:

```bash
go run ./cmd/processing-service
```

Em outro terminal:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Smoke test E2E (manual)

Fluxo resumido:

1. Subir PostgreSQL e RabbitMQ.
2. Garantir bucket e credenciais S3.
3. Aplicar migration.
4. Subir o servico.
5. Publicar mensagem `video.process`.
6. Validar:
   - registro em `processing_jobs`
   - eventos de saida (`status.update` + `video.completed` ou `video.failed`)

Guia detalhado: [docs/E2E_SMOKE_TEST.md](docs/E2E_SMOKE_TEST.md)

## Docker Compose (E2E com video real)

Suba tudo (PostgreSQL, RabbitMQ, MinIO, migration e app):

```bash
docker compose up --build
```

Servicos e portas:

- App: `http://localhost:8080`
- RabbitMQ UI: `http://localhost:15672` (`guest` / `guest`)
- MinIO API: `http://localhost:9000`
- MinIO Console: `http://localhost:9001` (`minioadmin` / `minioadmin`)
- PostgreSQL: `localhost:5432`

### Como testar com um video real

1. Envie um video para o bucket no MinIO:

```bash
docker run --rm --network host -v "$PWD:/work" minio/mc:latest \
  sh -c "mc alias set local http://127.0.0.1:9000 minioadmin minioadmin && \
  mc cp /work/seu-video.mp4 local/${S3_BUCKET:-processing-videos}/videos/550e8400-e29b-41d4-a716-446655440000/original.mp4"
```

2. Publique uma mensagem na fila `video.process`:

```bash
curl -u guest:guest -H "content-type:application/json" \
  -X POST "http://localhost:15672/api/exchanges/%2f/amq.default/publish" \
  -d '{
    "properties": { "delivery_mode": 2 },
    "routing_key": "video.process",
    "payload": "{\"videoId\":\"550e8400-e29b-41d4-a716-446655440000\",\"userId\":\"7ad7f47d-0bce-4c0d-a7ff-f35b7f5d67f1\",\"s3VideoKey\":\"videos/550e8400-e29b-41d4-a716-446655440000/original.mp4\",\"originalFilename\":\"seu-video.mp4\",\"createdAt\":\"2026-02-24T10:00:00Z\"}",
    "payload_encoding": "string"
  }'
```

3. Valide o resultado:

```bash
curl http://localhost:8080/ready
docker logs processing-service --tail 200
docker exec -it processing-postgres psql -U postgres -d processing_db -c "SELECT video_id,status,s3_zip_key,error_message,frame_count,updated_at FROM processing_jobs ORDER BY created_at DESC LIMIT 10;"
```

4. Confira o arquivo ZIP no MinIO Console (`http://localhost:9001`).

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
