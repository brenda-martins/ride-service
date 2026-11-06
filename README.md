# ride-service

Serviço dono do **ciclo de vida da corrida**. Recebe os comandos de passageiro e
motorista, aplica a máquina de estados e persiste cada transição. É o orquestrador
do fluxo de corrida; os demais serviços reagem a eventos ou são consultados por
ele.

- Contrato público: [`contracts/ride-api.yaml`](contracts/ride-api.yaml)
- Visão de sistema: [`docs/diagrams/arquitetura.md`](../docs/diagrams/arquitetura.md)
- Regras de domínio: [`docs/domain.md`](../docs/domain.md)
- Desenho renderizado: [`docs/arquitetura.svg`](docs/arquitetura.svg)

## 1. Limites do serviço

**Faz:**

- cria a corrida (`REQUESTED`);
- aplica as transições `ACCEPTED`, `IN_PROGRESS`, `COMPLETED`, `CANCELLED`;
- garante as invariantes do agregado `Ride`;
- controla concorrência com `version` (optimistic locking);
- garante idempotência dos comandos;
- calcula a tarifa estimada e a final (até a Fase 7, quando isso vira serviço);
- publica eventos de domínio via Outbox (Fase 4).

**Não faz:**

- não conhece nome, telefone ou veículo do motorista — só pergunta se ele está
  disponível e guarda um snapshot no aceite;
- não faz matching nem cálculo geoespacial;
- não autentica (Fase 8);
- não processa pagamento;
- não acessa o banco de nenhum outro serviço.

## 2. Estilo arquitetural

**Hexagonal (ports & adapters) com domínio rico.** A dependência aponta sempre
para dentro:

```
adapters  →  app  →  domain
cmd/api   →  tudo (composition root)
domain    →  nada além da stdlib
```

Motivo prático: na Fase 1 a disponibilidade do motorista é um *stub* e na Fase 3
vira uma chamada HTTP. Com o domínio atrás de portas, **nenhuma regra de negócio
muda** quando o adaptador troca.

```mermaid
flowchart LR
    MAIN["cmd/api<br/>composition root"]

    subgraph ENTRADA["Adaptadores de entrada"]
        HTTP["HTTP<br/>net/http · RFC 9457"]
    end

    subgraph APLICACAO["Aplicação — casos de uso"]
        UC["CreateRide · AcceptRide · StartRide<br/>CompleteRide · CancelRide · GetRide"]
        PORTS["Portas de saída<br/>RideRepository · DriverAvailability<br/>EventPublisher · Clock · IDGenerator · TxManager"]
    end

    subgraph DOMINIO["Domínio — sem dependências externas"]
        AGG["Agregado Ride<br/>transições · invariantes"]
        VO["Value objects<br/>GeoPoint · Money · Fare"]
    end

    subgraph SAIDA["Adaptadores de saída"]
        PG["Postgres<br/>pgx · migrations"]
        DRV["DriverAvailability<br/>stub (F1) → HTTP (F3)"]
        OUTBOX["Outbox publisher<br/>(F4)"]
    end

    DB[("ride DB<br/>PostgreSQL")]
    MQ{{"broker (F4)"}}

    MAIN --> HTTP
    MAIN --> PG
    MAIN --> DRV
    MAIN --> OUTBOX

    HTTP -->|"comando / consulta"| UC
    UC --> AGG
    AGG --- VO
    UC -->|"depende da interface"| PORTS
    PG -.->|"implementa"| PORTS
    DRV -.->|"implementa"| PORTS
    OUTBOX -.->|"implementa"| PORTS
    PG --- DB
    OUTBOX -.-> MQ

    classDef dom fill:#d4efdf,stroke:#1e8449,stroke-width:2px,color:#000
    classDef app fill:#ebf5fb,stroke:#2874a6,color:#000
    classDef adapt fill:#fdebd0,stroke:#b9770e,color:#000
    classDef externo fill:#f4ecf7,stroke:#7d3c98,color:#000

    class AGG,VO dom
    class UC,PORTS app
    class HTTP,PG,DRV,OUTBOX adapt
    class DB,MQ,MAIN externo
```

## 3. Estrutura de pastas (alvo)

```
ride-service/                       # este repositório
├── cmd/api/main.go                 # composition root: lê config e injeta adaptadores
├── internal/
│   ├── domain/ride/                # agregado, status, value objects, erros
│   │   ├── ride.go
│   │   ├── status.go               # máquina de estados
│   │   ├── fare.go
│   │   ├── money.go
│   │   ├── geopoint.go
│   │   ├── errors.go
│   │   └── ride_test.go            # tabela de transições
│   ├── app/                        # casos de uso + definição das portas
│   │   ├── ports.go
│   │   ├── create_ride.go
│   │   ├── accept_ride.go
│   │   ├── start_ride.go
│   │   ├── complete_ride.go
│   │   ├── cancel_ride.go
│   │   ├── get_ride.go
│   │   └── idempotency.go          # decorator dos comandos
│   ├── adapters/
│   │   ├── http/                   # handler, router, problem+json, middleware
│   │   ├── postgres/               # repositório, idempotência, transação
│   │   └── driverclient/           # stub (F1) e cliente HTTP com resiliência (F3)
│   └── platform/                   # config, clock, geração de IDs
│       ├── config/
│       ├── clock/
│       └── id/
├── migrations/                     # migrations SQL do schema ride
├── docs/arquitetura.svg
├── Dockerfile
├── go.mod
└── README.md
```