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

## 📊 Monitoramento

- **RabbitMQ Management UI**: http://localhost:15672
  - Usuário: `guest`
  - Senha: `guest`

## 🔧 Configuração

### Variáveis de Ambiente

| Variável            | Descrição                  | Padrão      |
| ------------------- | -------------------------- | ----------- |
| `RABBITMQ_HOST`     | Host do RabbitMQ           | `localhost` |
| `RABBITMQ_PORT`     | Porta do RabbitMQ          | `5672`      |
| `RABBITMQ_USER`     | Usuário do RabbitMQ        | `guest`     |
| `RABBITMQ_PASSWORD` | Senha do RabbitMQ          | `guest`     |
| `WORKER_POOL_SIZE`  | Tamanho do pool de workers | `5`         |
| `MAX_RETRIES`       | Máximo de tentativas       | `3`         |
| `RETRY_DELAY`       | Delay entre tentativas     | `1000ms`    |

## 🏗️ Arquitetura

O Gohopper utiliza uma arquitetura baseada em eventos com:

- **Publisher**: Publica eventos na fila RabbitMQ
- **Consumer**: Consome eventos com pool de workers
- **Processor**: Processa mensagens com retry e DLQ
- **Queue Manager**: Gerencia conexões e configurações do RabbitMQ

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
