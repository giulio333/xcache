# MemoryStore

Backend in-memory con sharding, TTL e indice tag. Pronto all'uso senza configurazione esterna.

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
    Aumentare gli shard riduce la contesa sui mutex ma aumenta l'uso di memoria (ogni shard alloca le proprie mappe — items e indice tag). `64` è adeguato per la maggior parte dei casi.

### `WithSweepInterval`

Controlla ogni quanto la goroutine di sweep itera tutti gli shard per rimuovere le chiavi scadute non ancora rilette.

Intervalli più brevi liberano memoria prima ma aumentano il carico CPU in background. Abbassare solo se il workload ha alto tasso di scrittura con TTL brevi e bassa rilettura.

!!! warning "Chiamare sempre `Close()`"
    `Close()` ferma la goroutine di sweep. Senza `Close()`, la goroutine rimane attiva per tutta la vita del processo. `Close()` può essere chiamato esattamente una volta.

## Eviction

Due meccanismi cooperano:

**Passiva** — al momento della lettura: se la chiave è scaduta viene rimossa (insieme alle sue voci nell'indice tag) e ritornato `ErrNotFound`. Nessun overhead, ma le chiavi mai rilette restano in memoria fino allo sweep.

**Attiva** — goroutine background: itera tutti gli shard ogni `sweepInterval` e rimuove le chiavi scadute (e le loro voci nell'indice tag). Previene l'accumulo di memoria su workload con alto tasso di scrittura e bassa rilettura.

## Indice tag

Ogni shard mantiene un `map[string]map[string]struct{}` (tag → set di chiavi) che alimenta `DeleteByTag`:

- **Insert**: ogni `Set` aggiunge la chiave al set di ciascuno dei suoi tag
- **Overwrite**: un `Set` su una chiave esistente rimuove prima la chiave dai tag vecchi e poi la aggiunge ai nuovi — l'indice non contiene mai riferimenti stale
- **Delete / sweep / lazy eviction**: rimuovono la chiave da tutti i tag a cui era associata; quando un tag rimane senza chiavi, il bucket viene eliminato per evitare crescita illimitata
- **Lookup `DeleteByTag(t)`**: itera gli shard, raccoglie le chiavi nel set `tags[t]` e le elimina dal `items` corrispondente — O(|chiavi del tag|) per shard

L'uso di un set (anziché di una slice) elimina i duplicati su `Set` ripetuti e garantisce O(1) per chiave durante l'invalidazione.

---

*[TTL]: Time To Live
*[FNV]: Fowler–Noll–Vo hash function
