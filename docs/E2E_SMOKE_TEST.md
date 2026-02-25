# E2E Smoke Test (local)

Este guia valida o fluxo ponta a ponta do `Processing Service`:

1. consumo de `video.process`
2. persistencia em `processing_jobs`
3. publicacao de `status.update` e evento final (`video.completed` ou `video.failed`)

## Pre-requisitos

- Go instalado
- `ffmpeg` instalado e acessivel em `WORKER_FFMPEG_BIN`
- PostgreSQL acessivel via `POSTGRES_URL`
- RabbitMQ acessivel via `RABBITMQ_URL`
- Bucket S3 acessivel pelas credenciais configuradas no `.env`

## 1) Configurar ambiente

```bash
cp .env.example .env
```

Edite `.env` com valores reais para:

- `POSTGRES_URL`
- `RABBITMQ_URL`
- `S3_REGION`
- `S3_BUCKET`

Se usar S3 compativel:

- `S3_ENDPOINT`
- `S3_ACCESS_KEY_ID`
- `S3_SECRET_ACCESS_KEY`
- `S3_USE_PATH_STYLE=true`

## 2) Aplicar migration

```bash
psql "$POSTGRES_URL" -f migrations/001_create_processing_jobs.sql
```

## 3) Subir o servico

```bash
go run ./cmd/processing-service
```

Valide operacao:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## 4) Publicar mensagem de teste na fila `video.process`

Exemplo de payload:

```json
{
  "videoId": "550e8400-e29b-41d4-a716-446655440000",
  "userId": "7ad7f47d-0bce-4c0d-a7ff-f35b7f5d67f1",
  "s3VideoKey": "videos/550e8400-e29b-41d4-a716-446655440000/original.mp4",
  "originalFilename": "sample.mp4",
  "createdAt": "2026-02-24T10:00:00Z"
}
```

Use seu client RabbitMQ preferido para publicar em `video.process`.

## 5) Validar persistencia no banco

```bash
psql "$POSTGRES_URL" -c "SELECT video_id,status,s3_zip_key,error_message,frame_count,updated_at FROM processing_jobs ORDER BY created_at DESC LIMIT 10;"
```

Resultado esperado:

- em sucesso: `status=COMPLETED`, `s3_zip_key` preenchido
- em erro: `status=FAILED`, `error_message` preenchida

## 6) Validar eventos de saida

Consuma as filas:

- `status.update`
- `video.completed`
- `video.failed`

Resultado esperado:

- sempre deve existir `status.update` para `PROCESSING`
- depois:
  - sucesso: `status.update` com `COMPLETED` + `video.completed`
  - falha: `status.update` com `FAILED` + `video.failed`

## Checklist rapido

- [ ] `/health` retorna `200`
- [ ] `/ready` retorna `200`
- [ ] mensagem publicada em `video.process`
- [ ] linha criada/atualizada em `processing_jobs`
- [ ] evento final publicado (`video.completed` ou `video.failed`)
