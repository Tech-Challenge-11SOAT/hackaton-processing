# Diagramas de Banco de Dados e Arquitetura - FIAP X

## Decisões de Arquitetura

- **Storage de arquivos**: AWS S3
- **Mensageria**: RabbitMQ 
- **Banco de dados**: Database per service (cada microsserviço com seu próprio schema PostgreSQL)
- **Cache**: Redis (sessões de autenticação + cache de status)

---

## Diagrama de Arquitetura

O sistema segue uma arquitetura de microsserviços orquestrada via Kubernetes, com comunicação assíncrona via RabbitMQ e storage de arquivos no AWS S3.

```mermaid
graph TB
    subgraph client [Cliente]
        WebApp["Web App / API Client"]
    end

    subgraph k8s [Kubernetes Cluster]
        subgraph gateway [API Gateway]
            AuthProxy["Auth Proxy Service (Java)"]
        end

        subgraph services [Microsservicos]
            StatusSvc["Status Service (Java)"]
            ProcessingSvc["Processing Service (Java)"]
            NotificationSvc["Notification Service (Java)"]
        end

        subgraph messaging [Mensageria]
            RabbitMQ["RabbitMQ"]
        end

        subgraph databases [Bancos de Dados]
            AuthDB["PostgreSQL - auth_db"]
            StatusDB["PostgreSQL - status_db"]
            ProcessingDB["PostgreSQL - processing_db"]
            NotificationDB["PostgreSQL - notification_db"]
        end

        subgraph caching [Cache]
            Redis["Redis"]
        end

        subgraph monitoring [Monitoramento]
            Prometheus["Prometheus"]
            Grafana["Grafana"]
        end
    end

    subgraph external [Servicos Externos]
        S3["AWS S3"]
        SMTP["SMTP / Email Service"]
    end

    WebApp -->|"HTTP/REST"| AuthProxy
    AuthProxy -->|"JWT validation"| Redis
    AuthProxy -->|"Proxy autenticado"| StatusSvc
    AuthProxy -->|"Proxy autenticado"| ProcessingSvc

    StatusSvc -->|"CRUD"| StatusDB
    StatusSvc -->|"Cache"| Redis
    AuthProxy -->|"CRUD usuarios"| AuthDB

    ProcessingSvc -->|"Upload/Download"| S3
    ProcessingSvc -->|"CRUD jobs"| ProcessingDB
    ProcessingSvc -->|"Consome filas"| RabbitMQ

    StatusSvc -->|"Publica evento"| RabbitMQ
    RabbitMQ -->|"video.process"| ProcessingSvc
    RabbitMQ -->|"video.completed / video.failed"| NotificationSvc
    RabbitMQ -->|"status.update"| StatusSvc

    NotificationSvc -->|"Envia email"| SMTP
    NotificationSvc -->|"CRUD"| NotificationDB

    Prometheus -->|"Scrape metrics"| AuthProxy
    Prometheus -->|"Scrape metrics"| StatusSvc
    Prometheus -->|"Scrape metrics"| ProcessingSvc
    Prometheus -->|"Scrape metrics"| NotificationSvc
    Grafana -->|"Query"| Prometheus
```

### Fluxo Principal

```mermaid
sequenceDiagram
    actor User
    participant AuthProxy as Auth Proxy
    participant StatusSvc as Status Service
    participant RabbitMQ as RabbitMQ
    participant ProcessingSvc as Processing Service
    participant S3 as AWS S3
    participant NotifSvc as Notification Service
    participant SMTP as Email

    User->>AuthProxy: POST /auth/login
    AuthProxy-->>User: JWT Token

    User->>AuthProxy: POST /videos/upload + JWT
    AuthProxy->>StatusSvc: Forward request
    StatusSvc->>S3: Upload video original
    StatusSvc->>StatusSvc: Cria registro status=PENDING
    StatusSvc->>RabbitMQ: Publish video.process
    StatusSvc-->>User: 202 Accepted + videoId

    RabbitMQ->>ProcessingSvc: Consume video.process
    ProcessingSvc->>S3: Download video
    ProcessingSvc->>ProcessingSvc: Extrai frames do video
    ProcessingSvc->>ProcessingSvc: Gera arquivo ZIP
    ProcessingSvc->>S3: Upload ZIP
    ProcessingSvc->>RabbitMQ: Publish status.update + video.completed

    RabbitMQ->>StatusSvc: Consume status.update
    StatusSvc->>StatusSvc: Atualiza status=COMPLETED

    RabbitMQ->>NotifSvc: Consume video.completed
    NotifSvc->>SMTP: Envia email ao usuario
    NotifSvc->>NotifSvc: Registra notificacao

    User->>AuthProxy: GET /videos + JWT
    AuthProxy->>StatusSvc: Forward request
    StatusSvc-->>User: Lista de videos + status

    User->>AuthProxy: GET /videos/{id}/download + JWT
    AuthProxy->>StatusSvc: Forward request
    StatusSvc->>S3: Gera presigned URL
    StatusSvc-->>User: Redirect / URL download ZIP
```

---

## Diagrama de Banco de Dados (ERD)

Cada microsserviço possui seu próprio banco de dados PostgreSQL (database-per-service pattern).

### auth_db (Auth Proxy Service)

```mermaid
erDiagram
    users {
        uuid id PK
        varchar name
        varchar email UK
        varchar password_hash
        timestamp created_at
        timestamp updated_at
    }
```

### status_db (Status Service)

```mermaid
erDiagram
    videos {
        uuid id PK
        uuid user_id FK
        varchar original_filename
        varchar s3_video_key
        varchar s3_zip_key
        varchar status
        varchar error_message
        timestamp created_at
        timestamp updated_at
    }

    videos }|--|| users_ref : "user_id (logico)"
    users_ref {
        uuid id PK
        varchar note "referencia logica - auth_db"
    }
```

O campo `status` assume os valores: `PENDING`, `PROCESSING`, `COMPLETED`, `FAILED`.

### processing_db (Processing Service)

```mermaid
erDiagram
    processing_jobs {
        uuid id PK
        uuid video_id
        varchar s3_video_key
        varchar s3_zip_key
        varchar status
        integer frame_count
        varchar error_message
        timestamp started_at
        timestamp completed_at
        timestamp created_at
    }
```

### notification_db (Notification Service)

```mermaid
erDiagram
    notifications {
        uuid id PK
        uuid user_id
        uuid video_id
        varchar type
        varchar recipient
        varchar status
        varchar error_message
        timestamp sent_at
        timestamp created_at
    }
```

O campo `type` assume: `PROCESSING_COMPLETED`, `PROCESSING_FAILED`. O campo `status` assume: `PENDING`, `SENT`, `FAILED`.

### Visão Consolidada do ERD

```mermaid
erDiagram
    users ||--o{ videos : "uploads"
    users ||--o{ notifications : "receives"
    videos ||--o{ processing_jobs : "processed by"
    videos ||--o{ notifications : "triggers"

    users {
        uuid id PK
        varchar name
        varchar email UK
        varchar password_hash
        timestamp created_at
        timestamp updated_at
    }

    videos {
        uuid id PK
        uuid user_id FK
        varchar original_filename
        varchar s3_video_key
        varchar s3_zip_key
        varchar status "PENDING, PROCESSING, COMPLETED, FAILED"
        varchar error_message
        timestamp created_at
        timestamp updated_at
    }

    processing_jobs {
        uuid id PK
        uuid video_id FK
        varchar s3_video_key
        varchar s3_zip_key
        varchar status "PENDING, PROCESSING, COMPLETED, FAILED"
        integer frame_count
        varchar error_message
        timestamp started_at
        timestamp completed_at
        timestamp created_at
    }

    notifications {
        uuid id PK
        uuid user_id FK
        uuid video_id FK
        varchar type "PROCESSING_COMPLETED, PROCESSING_FAILED"
        varchar recipient
        varchar status "PENDING, SENT, FAILED"
        varchar error_message
        timestamp sent_at
        timestamp created_at
    }
```

---

## Filas RabbitMQ

```mermaid
graph LR
    subgraph producers [Producers]
        StatusSvc["Status Service"]
        ProcessingSvc["Processing Service"]
    end

    subgraph queues [RabbitMQ Queues]
        Q1["video.process"]
        Q2["status.update"]
        Q3["video.completed"]
        Q4["video.failed"]
    end

    subgraph consumers [Consumers]
        ProcessingSvcC["Processing Service"]
        StatusSvcC["Status Service"]
        NotificationSvc["Notification Service"]
    end

    StatusSvc -->|"Solicita processamento"| Q1
    ProcessingSvc -->|"Atualiza status"| Q2
    ProcessingSvc -->|"Sucesso"| Q3
    ProcessingSvc -->|"Erro"| Q4

    Q1 --> ProcessingSvcC
    Q2 --> StatusSvcC
    Q3 --> NotificationSvc
    Q4 --> NotificationSvc
```

## Estrutura do Redis

```mermaid
graph TB
    Redis["Redis"]

    subgraph keys [Chaves e Padroes]
        Sessions["session:{userId}\nTTL baseado na expiracao do token"]
        Cache["video:status:{videoId}\nTTL curto de 30s"]
        RateLimit["ratelimit:{userId}:{endpoint}\nControle de requisicoes"]
    end

    AuthProxy["Auth Proxy"] -->|"JWT Sessions"| Redis
    StatusSvc["Status Service"] -->|"Cache de Status"| Redis
    AuthProxy -->|"Rate Limiting"| Redis

    Redis --- Sessions
    Redis --- Cache
    Redis --- RateLimit
```

---

## Infraestrutura Kubernetes

```mermaid
graph TB
    subgraph ingress [Ingress / LoadBalancer]
        LB["LoadBalancer"]
    end

    subgraph deployments [Deployments]
        AuthProxy["Auth Proxy Service\n(Java)"]
        StatusSvc["Status Service\n(Java)"]
        ProcessingSvc["Processing Service\n(Java + HPA)"]
        NotificationSvc["Notification Service\n(Java)"]
    end

    subgraph statefulsets [StatefulSets]
        PG_Auth["PostgreSQL\nauth_db"]
        PG_Status["PostgreSQL\nstatus_db"]
        PG_Processing["PostgreSQL\nprocessing_db"]
        PG_Notification["PostgreSQL\nnotification_db"]
        RabbitMQ["RabbitMQ"]
        Redis["Redis"]
    end

    subgraph config [Configuracao]
        Secrets["Secrets\nDB, S3, SMTP"]
        ConfigMaps["ConfigMaps\nConfigs por servico"]
    end

    LB --> AuthProxy
    AuthProxy -->|"ClusterIP"| StatusSvc
    AuthProxy -->|"ClusterIP"| ProcessingSvc

    AuthProxy --> PG_Auth
    StatusSvc --> PG_Status
    ProcessingSvc --> PG_Processing
    NotificationSvc --> PG_Notification

    StatusSvc --> RabbitMQ
    ProcessingSvc --> RabbitMQ
    NotificationSvc --> RabbitMQ

    AuthProxy --> Redis
    StatusSvc --> Redis

    Secrets -.-> deployments
    ConfigMaps -.-> deployments
```
