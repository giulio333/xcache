# MemoryStore

Backend in-memory con sharding e TTL. Pronto all'uso senza configurazione esterna.

## Creazione

```go title="main.go"
import "github.com/giulio333/xcache/store/memory"

store := memory.NewStore()
defer store.Close()
```

Con opzioni:

```go title="main.go"
store := memory.NewStore(
    memory.WithShards(128),
    memory.WithSweepInterval(30*time.Second),
)
```

## Opzioni

| Opzione | Default | Note |
|---|---|---|
| `WithShards(n uint64)` | `64` | Numero di shard — aumentare sotto alto parallelismo |
| `WithSweepInterval(d time.Duration)` | `1m` | Frequenza dello sweep attivo delle chiavi scadute |

### `WithShards`

Le chiavi vengono distribuite sugli shard via hash FNV-1a. Con `n` shard, fino a `n` goroutine concorrenti su chiavi diverse possono operare senza bloccarsi a vicenda.

!!! note
    Aumentare gli shard riduce la contesa sui mutex ma aumenta l'uso di memoria (ogni shard alloca la propria mappa). `64` è adeguato per la maggior parte dei casi.

### `WithSweepInterval`

Controlla ogni quanto la goroutine di sweep itera tutti gli shard per rimuovere le chiavi scadute non ancora rilette.

Intervalli più brevi liberano memoria prima ma aumentano il carico CPU in background. Abbassare solo se il workload ha alto tasso di scrittura con TTL brevi e bassa rilettura.

!!! warning "Chiamare sempre `Close()`"
    `Close()` ferma la goroutine di sweep. Senza `Close()`, la goroutine rimane attiva per tutta la vita del processo.

## Eviction

Due meccanismi cooperano:

**Passiva** — al momento della lettura: se la chiave è scaduta viene rimossa e ritornato `ErrNotFound`. Nessun overhead, ma le chiavi mai rilette restano in memoria fino allo sweep.

**Attiva** — goroutine background: itera tutti gli shard ogni `sweepInterval` e rimuove le chiavi scadute. Previene l'accumulo di memoria su workload con alto tasso di scrittura e bassa rilettura.

---

*[TTL]: Time To Live
*[FNV]: Fowler–Noll–Vo hash function
