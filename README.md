# Gohopper 🐰

Sistema assíncrono de publicação e consumo de eventos usando Go e RabbitMQ. Implementa processamento concorrente de mensagens com pool de workers, controle de falhas com backoff exponencial e redirecionamento automático para Dead Letter Queue (DLQ).

## 🚀 Características

- **Processamento Concorrente**: Pool de workers para processamento paralelo de mensagens
- **Controle de Falhas**: Backoff exponencial e retry automático
- **Dead Letter Queue**: Redirecionamento automático de mensagens com falha
- **Race Conditions**: Tratamento adequado com boas práticas de concorrência
- **WaitGroups**: Sincronização de goroutines
- **Canais**: Comunicação entre goroutines de forma thread-safe

## 📁 Estrutura do Projeto

```
gohopper/
├── cmd/
│   ├── publisher/     # Aplicação publisher
│   └── consumer/      # Aplicação consumer
├── internal/
│   ├── queue/         # Lógica de filas e RabbitMQ
│   └── processor/     # Processamento de mensagens
├── configs/           # Configurações
├── docs/              # Documentação e diagramas
├── docker-compose.yml # RabbitMQ container
├── go.mod            # Dependências Go
└── .env              # Variáveis de ambiente
```

## 🛠️ Pré-requisitos

- Go 1.21+
- Docker e Docker Compose
- RabbitMQ (via Docker)

## 🚀 Instalação e Execução

### 1. Clone o repositório

```bash
git clone <repository-url>
cd gohopper
```

### 2. Inicie o RabbitMQ

```bash
docker-compose up -d
```

### 3. Configure as variáveis de ambiente

```bash
cp .env.example .env
# Edite o arquivo .env conforme necessário
```

### 4. Instale as dependências

```bash
go mod download
```

### 5. Execute o consumer

```bash
go run cmd/consumer/main.go
```

### 6. Execute o publisher (em outro terminal)

```bash
go run cmd/publisher/main.go
```

### 7. Simule envio de eventos via CLI

```bash
make publish
```

## 📊 Monitoramento

- **RabbitMQ Management UI**: http://localhost:15672

  - Usuário: `guest`
  - Senha: `guest`

- **Prometheus Metrics**: http://localhost:8080/metrics

  - Endpoint para coleta de métricas do Prometheus
  - Métricas disponíveis:
    - `messages_processed_total`: Total de mensagens processadas com sucesso
    - `messages_failed_total`: Total de mensagens que falharam
    - `retry_attempts_total`: Total de tentativas de retry
    - `processing_duration_seconds`: Duração do processamento (histograma)
    - `active_workers`: Número de workers ativos
    - `queue_size`: Tamanho atual da fila de jobs

- **Health Check**: http://localhost:8080/health

  - Endpoint para verificação de saúde do sistema

- **Dashboard**: http://localhost:8080/
  - Interface web com links para métricas e health check

## 📝 Logging

O Gohopper utiliza logging estruturado com suporte a rastreamento:

### Características

- **Formato JSON**: Logs estruturados para fácil parsing
- **Trace ID**: Rastreamento de requisições através do sistema
- **Contexto**: Informações de contexto em cada log
- **Níveis**: Debug, Info, Warn, Error, Fatal
- **Metadados**: Campos adicionais para análise

### Exemplo de Log

```json
{
  "timestamp": "2024-01-01T12:00:00Z",
  "level": "info",
  "message": "Message published successfully",
  "trace_id": "550e8400-e29b-41d4-a716-446655440001",
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "message_type": "user.created",
  "source": "gohopper-publisher",
  "exchange_name": "events_exchange",
  "routing_key": "event.user.created",
  "body_size": 512
}
```

### Configuração

```bash
LOG_LEVEL=info  # debug, info, warn, error, fatal
```

## 🔧 Configuração

### Variáveis de Ambiente

| Variável                    | Descrição                                | Padrão               |
| --------------------------- | ---------------------------------------- | -------------------- |
| `RABBITMQ_HOST`             | Host do RabbitMQ                         | `localhost`          |
| `RABBITMQ_PORT`             | Porta do RabbitMQ                        | `5672`               |
| `RABBITMQ_USER`             | Usuário do RabbitMQ                      | `guest`              |
| `RABBITMQ_PASSWORD`         | Senha do RabbitMQ                        | `guest`              |
| `WORKER_POOL_SIZE`          | Tamanho do pool de workers               | `5`                  |
| `MAX_RETRIES`               | Máximo de tentativas                     | `3`                  |
| `RETRY_DELAY`               | Delay entre tentativas                   | `1000ms`             |
| `RETRY_TIMEOUT`             | Timeout para cada tentativa              | `30s`                |
| `MAX_CONCURRENT`            | Máximo de workers ativos simultaneamente | `5`                  |
| `DLQ_NAME`                  | Nome da fila DLQ                         | `events_dlq`         |
| `PUBLISH_INTERVAL`          | Intervalo de publicação                  | `2s`                 |
| `PUBLISH_BATCH_SIZE`        | Tamanho do lote de publicação            | `10`                 |
| `EVENT_SOURCE`              | Fonte dos eventos                        | `gohopper-publisher` |
| `WORKER_SHUTDOWN_TIMEOUT`   | Timeout para parada do worker pool       | `30s`                |
| `CONSUMER_SHUTDOWN_TIMEOUT` | Timeout para parada do consumer          | `10s`                |
| `STATS_REPORT_INTERVAL`     | Intervalo de relatórios de estatísticas  | `30s`                |
| `HEALTH_CHECK_INTERVAL`     | Intervalo de health check                | `60s`                |
| `METRICS_PORT`              | Porta do servidor de métricas Prometheus | `8080`               |

## 🏗️ Arquitetura

O Gohopper utiliza uma arquitetura baseada em eventos com:

- **Publisher**: Publica eventos na fila RabbitMQ
- **Consumer**: Consome eventos com pool de workers
- **Processor**: Processa mensagens com retry e DLQ
- **Queue Manager**: Gerencia conexões e configurações do RabbitMQ

## 📤 Publisher

O publisher do Gohopper oferece duas modalidades de operação:

### Modo Contínuo

- Publica eventos automaticamente a cada intervalo configurado
- Ideal para simulação de carga e testes contínuos
- Suporte a graceful shutdown

### Modo CLI

- Publica um conjunto de eventos de teste e encerra
- Útil para testes pontuais e validação
- Executado via `make publish`

### Tipos de Eventos Suportados

- `user.created` - Criação de usuário
- `user.updated` - Atualização de usuário
- `order.created` - Criação de pedido
- `payment.processed` - Processamento de pagamento
- `notification.sent` - Envio de notificação

## 📥 Consumer

O consumer do Gohopper implementa processamento concorrente com worker pool:

### Worker Pool

- **Processamento Concorrente**: Múltiplas goroutines processando mensagens simultaneamente
- **Configurável**: Número de workers ajustável via parâmetro `-workers`
- **Graceful Shutdown**: Parada segura com finalização de jobs em andamento
- **Estatísticas**: Monitoramento de performance e status do pool

### Recursos Avançados

- **Retry com Backoff Exponencial**: Tentativas automáticas com delay crescente e jitter
- **Context Timeout**: Timeout configurável para cada tentativa de processamento
- **Dead Letter Queue (DLQ)**: Sistema robusto de DLQ com separação de tipos de falha
- **DLQ Consumer**: Consumer específico para processar mensagens falhadas
- **Semáforo Customizado**: Controle de concorrência via semáforo com chan struct{}
- **Acknowledgment**: Confirmação manual de processamento bem-sucedido
- **Trace ID**: Rastreamento completo de mensagens através do sistema
- **WaitGroup**: Sincronização de goroutines com controle de finalização
- **Graceful Shutdown**: Encerramento seguro com timeout configurável
- **Signal Handling**: Controle de processo com SIGINT/SIGTERM
- **Health Check**: Monitoramento contínuo do estado do worker pool

### Configuração

```bash
# Consumer padrão (5 workers)
make run-consumer

# Consumer com 10 workers
make run-consumer-workers

# Consumer com tag customizada
make run-consumer-tag
```

### Parâmetros de Linha de Comando

| Parâmetro  | Descrição         | Padrão              |
| ---------- | ----------------- | ------------------- |
| `-workers` | Número de workers | `5`                 |
| `-tag`     | Tag do consumer   | `gohopper-consumer` |

### Controle de Processo

O consumer implementa controle robusto de processo com:

#### **Graceful Shutdown**

- **SIGINT/SIGTERM**: Captura sinais de interrupção
- **WaitGroup**: Sincroniza finalização de todas as goroutines
- **Timeout Configurável**: Evita travamento em caso de falha
- **Context Cancellation**: Propaga cancelamento para todas as goroutines

#### **Goroutines Coordenadas**

- **Worker Pool**: Workers sincronizados com WaitGroup
- **Stats Reporting**: Relatórios periódicos com controle de vida
- **Health Check**: Monitoramento contínuo com graceful shutdown
- **Message Consumption**: Consumo de mensagens com context cancellation

#### **Logs de Shutdown**

### Retry com Exponential Backoff

O sistema implementa retry robusto com exponential backoff:

#### **Características do Retry**

- **Tentativas Limitadas**: Configurável via `MAX_RETRIES` (padrão: 3)
- **Exponential Backoff**: Delay crescente entre tentativas (base \* 2^attempt)
- **Jitter**: Variação aleatória para evitar thundering herd
- **Context Timeout**: Timeout configurável para cada tentativa
- **Graceful Cancellation**: Respeita context cancellation

#### **Fórmula do Backoff**

```
delay = baseDelay * 2^attempt + jitter
jitter = delay * 0.1 * (0.5 + 0.5 * random)
maxDelay = 30s
```

#### **Exemplo de Retry**

```json
{
  "level": "warn",
  "message": "Message retry scheduled",
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "attempt": 1,
  "max_retries": 3,
  "retry_delay_ms": 2000,
  "error": "simulated error processing user.created event"
}
{
  "level": "info",
  "message": "Message processed successfully after retry",
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "attempt": 2,
  "retry_count": 0
}
```

#### **Context Timeout**

- **Timeout por Tentativa**: Configurável via `RETRY_TIMEOUT`
- **Cancellation**: Respeita context cancellation durante retry
- **Graceful Handling**: Logs detalhados de timeout e cancellation

### Dead Letter Queue (DLQ)

O sistema implementa DLQ robusto com separação clara de tipos de falha:

#### **Tipos de Falha**

- **Retryable**: Erros temporários que podem ser retryados
- **Non-Retryable**: Erros permanentes enviados direto para DLQ
- **Max Retries Exceeded**: Após esgotar tentativas
- **Timeout**: Timeout de processamento
- **Context Cancelled**: Cancelação de contexto

#### **Estratégia de DLQ**

```go
// Determinação automática do tipo de falha
switch failureType {
case FailureTypeRetryable:
    // Reject para retry
case FailureTypeNonRetryable:
    // Enviar direto para DLQ
case FailureTypeMaxRetries:
    // Enviar para DLQ após max tentativas
case FailureTypeTimeout:
    // Enviar para DLQ por timeout
case FailureTypeContext:
    // Reject para retry (context pode ser temporário)
}
```

#### **DLQ Consumer**

```bash
# Executar consumer específico para DLQ
make run-dlq-consumer

# Com tag customizada
go run ./cmd/dlq-consumer -tag=my-dlq-consumer
```

#### **Processamento de DLQ**

- **Max Retries**: Alerting, review manual, recovery
- **Timeout**: Análise de performance, scaling
- **Non-Retryable**: Validação de dados, migração de schema
- **Unknown**: Processamento genérico

#### **Metadados de DLQ**

```json
{
  "dlq_reason": "max_retries_exceeded",
  "dlq_timestamp": "2025-07-23T11:47:30-03:00",
  "final_error": "simulated error processing user.created event"
}
```

### Semáforo Customizado

O sistema implementa controle de concorrência via semáforo customizado:

#### **Características do Semáforo**

- **Controle de Concorrência**: Limita workers ativos simultaneamente
- **Implementação Customizada**: Usando `chan struct{}`
- **Context Awareness**: Respeita context cancellation
- **Estatísticas em Tempo Real**: Monitoramento de utilização
- **Thread-Safe**: Mutex para operações concorrentes

#### **Implementação**

```go
type Semaphore struct {
	permits chan struct{}
	mu      sync.RWMutex
	active  int
	max     int
}

func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.permits <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Semaphore) Release() {
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	<-s.permits
}
```

#### **Configuração**

```bash
# Máximo de workers ativos simultaneamente
MAX_CONCURRENT=5
```

#### **Estatísticas do Semáforo**

```json
{
  "active": 3,
  "max": 5,
  "available": 2,
  "utilization": 60.0
}
```

#### **Logs de Semáforo**

```json
{
  "level": "debug",
  "message": "Semaphore permit acquired",
  "worker_id": 1,
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "semaphore_stats": {
    "active": 3,
    "max": 5,
    "available": 2,
    "utilization": 60.0
  }
}
```

```json
{
  "level": "info",
  "message": "Shutdown signal received",
  "signal": "SIGINT/SIGTERM"
}
{
  "level": "info",
  "message": "All workers stopped gracefully"
}
{
  "level": "info",
  "message": "All goroutines stopped gracefully"
}
```

## 📈 Métricas Prometheus

O Gohopper inclui métricas Prometheus para monitoramento em tempo real:

### **Métricas Disponíveis**

#### **Counters (Contadores)**

- `messages_processed_total`: Total de mensagens processadas com sucesso
- `messages_failed_total`: Total de mensagens que falharam no processamento
- `retry_attempts_total`: Total de tentativas de retry realizadas

#### **Histograms (Histogramas)**

- `processing_duration_seconds`: Duração do processamento de mensagens
  - Buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]

#### **Gauges (Medidores)**

- `active_workers`: Número atual de workers ativos
- `queue_size`: Tamanho atual da fila de jobs

### **Exemplo de Métricas**

```bash
# HELP messages_processed_total Total number of messages processed successfully
# TYPE messages_processed_total counter
messages_processed_total 42

# HELP messages_failed_total Total number of messages that failed processing
# TYPE messages_failed_total counter
messages_failed_total 3

# HELP retry_attempts_total Total number of retry attempts
# TYPE retry_attempts_total counter
retry_attempts_total 8

# HELP processing_duration_seconds Time spent processing messages
# TYPE processing_duration_seconds histogram
processing_duration_seconds_bucket{le="0.005"} 0
processing_duration_seconds_bucket{le="0.01"} 5
processing_duration_seconds_bucket{le="0.025"} 15
processing_duration_seconds_bucket{le="0.05"} 25
processing_duration_seconds_bucket{le="0.1"} 35
processing_duration_seconds_bucket{le="0.25"} 40
processing_duration_seconds_bucket{le="0.5"} 42
processing_duration_seconds_bucket{le="1"} 42
processing_duration_seconds_bucket{le="2.5"} 42
processing_duration_seconds_bucket{le="5"} 42
processing_duration_seconds_bucket{le="10"} 42
processing_duration_seconds_bucket{le="+Inf"} 42
processing_duration_seconds_sum 8.5
processing_duration_seconds_count 42

# HELP active_workers Number of currently active workers
# TYPE active_workers gauge
active_workers 5

# HELP queue_size Current size of the job queue
# TYPE queue_size gauge
queue_size 0
```

### **Configuração**

```bash
# Porta do servidor de métricas
METRICS_PORT=8080
```

### **Endpoints**

- **Métricas**: `GET /metrics` - Endpoint para coleta do Prometheus
- **Health Check**: `GET /health` - Verificação de saúde do sistema
- **Dashboard**: `GET /` - Interface web com links

### **Integração com Prometheus**

Adicione ao `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "gohopper-consumer"
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: /metrics
    scrape_interval: 15s
```

### Estrutura do Evento

O Gohopper utiliza um schema de mensagem estruturado com metadados ricos:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "user.created",
  "data": {
    "user_id": "user-1",
    "email": "user1@example.com",
    "name": "User 1",
    "timestamp": 1704067200,
    "event_data": "User data for event 1"
  },
  "metadata": {
    "priority": 1,
    "retry_count": 0,
    "headers": {
      "test_mode": true,
      "continuous_mode": false
    },
    "tags": ["test", "cli", "user"]
  },
  "timestamp": "2024-01-01T12:00:00Z",
  "source": "gohopper-publisher",
  "version": "1.0.0",
  "trace_id": "550e8400-e29b-41d4-a716-446655440001",
  "correlation_id": ""
}
```

#### Campos da Mensagem

| Campo            | Tipo   | Descrição                   |
| ---------------- | ------ | --------------------------- |
| `id`             | string | UUID único da mensagem      |
| `type`           | string | Tipo do evento              |
| `data`           | object | Dados do evento             |
| `metadata`       | object | Metadados da mensagem       |
| `timestamp`      | string | Timestamp ISO 8601          |
| `source`         | string | Origem da mensagem          |
| `version`        | string | Versão do schema            |
| `trace_id`       | string | ID de rastreamento          |
| `correlation_id` | string | ID de correlação (opcional) |

#### Metadados

| Campo         | Tipo     | Descrição                     |
| ------------- | -------- | ----------------------------- |
| `priority`    | int      | Prioridade da mensagem (1-10) |
| `retry_count` | int      | Número de tentativas          |
| `headers`     | object   | Headers customizados          |
| `tags`        | array    | Tags para categorização       |
| `ttl`         | duration | Time-to-live (opcional)       |

## 🔄 Fluxo de Processamento

1. **Publisher** envia mensagem para exchange
2. **Exchange** roteia para fila baseado no routing key
3. **Consumer** recebe mensagem com worker do pool
4. **Processor** processa com retry em caso de falha
5. **DLQ** recebe mensagens que falharam após max retries

## 🧪 Testes

```bash
# Executar todos os testes
go test ./...

# Executar testes com coverage
go test -cover ./...

# Executar testes de benchmark
go test -bench=. ./...
```

## 📈 Performance

- **Throughput**: Processamento de milhares de mensagens por segundo
- **Latência**: Baixa latência com processamento assíncrono
- **Escalabilidade**: Pool de workers configurável
- **Resiliência**: Retry automático e DLQ para falhas

## 🤝 Contribuição

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo [LICENSE](LICENSE) para mais detalhes.

## 👥 Autores

- **Seu Nome** - _Desenvolvimento inicial_ - [SeuGitHub](https://github.com/seugithub)

## 🙏 Agradecimentos

- RabbitMQ para o sistema de mensageria
- Go team pela linguagem e runtime
- Comunidade Go pelos padrões e boas práticas
