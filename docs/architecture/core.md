# Meccanismi interni

Documentazione dei componenti interni non esposti all'utente finale.

---

## `cache[T]` — implementazione di `Cache[T]`

```go title="cache_impl.go"
type cache[T any] struct {
    store Store
    group singleflight.Group
}
```

Fa da **traduttore** tra il mondo generico (`T`) e il mondo `any` dello `Store`. Contiene l'unico type assertion di tutta la libreria:

```go title="cache_impl.go"
val, ok := entry.Value.(T) // (1)
if !ok {
    return zero, fmt.Errorf("xcache: type mismatch for key %q", key)
}
```

1. Unico punto in cui `any` viene convertito al tipo concreto. In caso di mismatch viene ritornato un errore tipizzato — mai un panic — così un misuso (stesso `Store` condiviso da `Cache[T]` con namespace di chiave sovrapposti) è osservabile dal chiamante.

### Singleflight in `GetOrLoad`

```go title="cache_impl.go"
val, err, _ := c.group.Do(key, func() (any, error) { // (1)
    return loader(ctx)
})
```

1. `group.Do` garantisce che per la stessa chiave, sotto carico concorrente, `loader` venga chiamato una sola volta. Le altre goroutine aspettano su un `WaitGroup` interno e ricevono lo stesso risultato.

Come funziona `singleflight.Group` internamente:

```
goroutine 1 → group.Do("key", fn)
  nessuna chiamata in corso → esegue fn(), crea entry con WaitGroup

goroutine 2, 3, ... → group.Do("key", fn)
  chiamata in corso → wg.Wait()  ← bloccate qui

fn() termina → wg.Done()
  tutte le goroutine in attesa si sbloccano e leggono lo stesso risultato
```

Il lock interno viene rilasciato immediatamente dopo aver registrato la chiamata — le goroutine non si accodano sul mutex ma sul WaitGroup, che scala senza contesa.

---

## `MemoryStore` — sharding

```go title="store/memory/store.go"
type MemoryStore struct {
    shards    []*shard  // 64 per default
    numShards uint64
    stopSweep chan struct{}
}

type shard struct {
    mu    sync.RWMutex
    items map[string]item
    tags  map[string]map[string]struct{} // tag → set di chiavi
}
```

La chiave viene distribuita sugli shard tramite hash FNV-1a:

```go title="store/memory/store.go"
func (s *MemoryStore) getShard(key string) *shard {
    h := fnv.New64a()
    h.Write([]byte(key))
    return s.shards[h.Sum64()%s.numShards]
}
```

Con 64 shard, 64 richieste concorrenti su chiavi diverse possono procedere in parallelo senza bloccarsi a vicenda. Un solo shard con `sync.RWMutex` globale creerebbe un collo di bottiglia sotto alto parallelismo.

L'indice tag è un set (`map[string]struct{}`) per chiave: questo elimina i duplicati su `Set` ripetuti e dà lookup/cancellazione O(1) per chiave durante `DeleteByTag`.

---

## `MemoryStore` — TTL passivo e attivo

Due meccanismi cooperano per rimuovere le chiavi scadute:

**Passivo** (al momento della lettura):

```go title="store/memory/store.go"
if it.isExpired() {
    sh.mu.Lock()
    removeFromTagIndex(sh, key, it.tags)
    delete(sh.items, key)
    sh.mu.Unlock()
    return xcache.Entry{}, xcache.ErrNotFound
}
```

Rimuove la chiave (e le sue voci nell'indice tag) quando viene letta dopo la scadenza. Non richiede goroutine, ma lascia chiavi scadute in memoria finché non vengono richieste.

**Attivo** (goroutine background):

```go title="store/memory/store.go"
func (s *MemoryStore) sweep(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            for _, sh := range s.shards {
                sh.mu.Lock()
                for k, it := range sh.items {
                    if it.isExpired() {
                        removeFromTagIndex(sh, k, it.tags)
                        delete(sh.items, k)
                    }
                }
                sh.mu.Unlock()
            }
        case <-s.stopSweep:
            return
        }
    }
}
```

Itera tutti gli shard ogni `sweepInterval` (default: 1 minuto) e rimuove le chiavi scadute anche se mai rilette. Prevenzione OOM su workload con alto tasso di scrittura e bassa rilettura.

!!! warning "Chiamare sempre `Close()`"
    `Close()` chiude il canale `stopSweep` e termina la goroutine di sweep. Senza `Close()`, la goroutine rimane attiva per tutta la vita del processo.

---

## `MemoryStore` — invariante dell'indice tag

Quattro punti scrivono nello shard e devono mantenere l'indice coerente con la mappa `items`:

| Operazione | Effetto sull'indice |
|---|---|
| `Set` (nuova chiave) | Aggiunge la chiave al set di ciascun tag |
| `Set` (overwrite) | Rimuove la chiave dai tag vecchi prima di aggiungerla ai nuovi |
| `Delete` | Rimuove la chiave da tutti i tag a cui era associata |
| `Get` (chiave scaduta) | Stesso comportamento di `Delete` |
| `sweep` | Stesso comportamento di `Delete`, su tutte le chiavi scadute |

L'helper `removeFromTagIndex` centralizza la rimozione (rimuovere la chiave dal set, e se il set rimane vuoto rimuovere anche il bucket del tag) e va sempre invocato con il write lock dello shard già acquisito.

---

## `ChainStore` — backfill automatico

Quando `Get` trova la chiave nello store `i` (non nel primo), ripopola automaticamente tutti gli store precedenti propagando TTL e tag originali:

```go title="chain.go"
for i, s := range c.stores {
    entry, err := s.Get(ctx, key)
    if err != nil { continue }

    opts := entryOpts(entry) // (1)
    for _, prev := range c.stores[:i] {
        _ = prev.Set(ctx, key, entry.Value, opts...)
    }
    return entry, nil
}
```

1. `entryOpts` ricostruisce le `Option` (`WithTTL`, `WithTags`) a partire dai metadati dell'`Entry`. Il backfill conserva quindi sia la scadenza residua sia le label di gruppo, così `DeleteByTag` continua a funzionare anche sui tier ripopolati.

---

## `applyOptions` — costruzione delle opzioni

```go title="options.go"
func ApplyOptions(opts []Option) *Options {
    o := &Options{}
    for _, opt := range opts {
        opt(o)
    }
    return o
}
```

Implementazione del **Functional Options Pattern**: ogni `Option` è una funzione che modifica `*Options`. L'utente non vede mai la struct `Options` direttamente — interagisce solo con `WithTTL`, `WithTags`, ecc.

Vantaggi rispetto a una struct di configurazione esposta:
- Aggiungere nuove opzioni non rompe la firma delle funzioni esistenti
- I valori di default stanno in `&Options{}`, non in ogni call site
- L'API rimane leggibile: `xcache.WithTTL(5*time.Minute)` si legge come una frase

---

*[OOM]: Out Of Memory
*[TTL]: Time To Live
*[FNV]: Fowler–Noll–Vo hash function
