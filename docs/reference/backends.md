# Backend

---

## MemoryStore

Backend in-memory con sharding, TTL passivo, sweep attivo e indice tag.

### Costruzione

```go
import "github.com/giulio333/xcache/store/memory"

store := memory.NewStore()
defer store.Close()
```

Con opzioni:

```go
store := memory.NewStore(
    memory.WithShards(128),
    memory.WithSweepInterval(30*time.Second),
)
```

### Opzioni

#### WithShards

```go
func WithShards(n uint64) Option
```

Numero di shard in cui viene suddiviso lo store. Le chiavi vengono distribuite tramite hash FNV-1a: con `n` shard, fino a `n` goroutine concorrenti su chiavi diverse possono operare senza bloccarsi a vicenda.

Default: `64`. Aumentare sotto workload ad alto parallelismo. Ogni shard alloca le proprie mappe, quindi shard più numerosi implicano un uso di memoria leggermente maggiore.

#### WithSweepInterval

```go
func WithSweepInterval(d time.Duration) Option
```

Frequenza della goroutine che itera tutti gli shard e rimuove le chiavi scadute non ancora rilette.

Default: `1m`. Abbassare solo se il workload ha alto tasso di scrittura con TTL brevi e bassa rilettura, e si vuole recuperare memoria prima.

### Eviction

Le chiavi scadute vengono rimosse in due momenti distinti.

**Passiva** — al `Get`: se la chiave risulta scaduta al momento della lettura, viene rimossa insieme alle sue voci nell'indice tag e viene ritornato `ErrNotFound`. Non richiede goroutine, ma le chiavi mai rilette restano in memoria fino allo sweep.

**Attiva** — goroutine di sweep: itera tutti gli shard ogni `sweepInterval` e rimuove le chiavi scadute, indice tag incluso. Previene l'accumulo di memoria su workload con alto tasso di scrittura e bassa rilettura.

!!! warning "Chiamare sempre `Close()`"
    `Close()` ferma la goroutine di sweep. Senza `Close()`, la goroutine rimane attiva per tutta la vita del processo.

### Indice tag

Ogni shard mantiene un indice `map[string]map[string]struct{}` (tag → set di chiavi) che alimenta `DeleteByTag`.

- **Scrittura**: ogni `Set` aggiunge la chiave al set di ciascun tag indicato.
- **Sovrascrittura**: il `Set` rimuove prima la chiave dai tag vecchi, poi la aggiunge ai nuovi. Nessun riferimento stale.
- **Cancellazione**: `Delete`, eviction passiva e sweep rimuovono la chiave da tutti i tag associati. Se il set di un tag rimane vuoto, il bucket viene eliminato.
- **Invalidazione**: `DeleteByTag(t)` itera gli shard, raccoglie le chiavi nel set `tags[t]` e le elimina — O(|chiavi del tag|) per shard.

---

## ChainStore

Store a cascata che implementa il pattern L1 → L2.

### Costruzione

```go
func xcache.NewChain(stores ...Store) Store
```

Accetta un numero arbitrario di store. La ricerca scorre da sinistra a destra.

```go
l1 := memory.NewStore()
l2 := redis.NewStore(client)

cache := xcache.New[User](xcache.NewChain(l1, l2))
```

### Comportamento in lettura

`Get` scorre i tier da sinistra a destra. Non appena un tier risponde con successo, `ChainStore` ripopola automaticamente tutti i tier precedenti con il TTL residuo e i tag originali dell'entry, poi ritorna il valore.

Il TTL residuo viene propagato, non quello originale: se la chiave in L2 scade tra 3 minuti, L1 la memorizza con 3 minuti, non con il TTL iniziale. Questo garantisce che L1 scada prima o insieme a L2.

### Comportamento in scrittura

`Set`, `Delete`, `DeleteMany`, `DeleteByTag` e `Clear` propagano a tutti i tier in ordine. La prima operazione che ritorna un errore ferma la propagazione — i tier successivi non vengono toccati.

### Tre o più livelli

```go
xcache.NewChain(l1, l2, l3)
```

Il backfill popola tutti i tier a sinistra di quello che ha risposto.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
*[FNV]: Fowler–Noll–Vo hash function
