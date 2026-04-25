# Cache stampede e GetOrLoad

Il pattern manuale Get → carica dal DB → Set nasconde un problema strutturale che si manifesta solo sotto carico.

---

## Il problema

Quando una chiave molto richiesta scade, tutte le goroutine che erano in attesa trovano simultaneamente il miss. Ognuna di loro va al database, esegue la stessa query, ottiene lo stesso valore, e lo scrive in cache — per poi buttare il risultato perché un'altra goroutine ha già scritto. Nel peggiore dei casi, decine o centinaia di query identiche arrivano al database nello stesso istante: è la **cache stampede**.

```
chiave scaduta
  goroutine 1 → Get → miss → query DB → Set
  goroutine 2 → Get → miss → query DB → Set   ← query duplicate
  goroutine 3 → Get → miss → query DB → Set   ← query duplicate
  ...
```

La frequenza con cui accade dipende dalla popolarità della chiave e dal TTL. Su endpoint ad alto traffico con TTL brevi, il problema è ricorrente e non richiede picchi straordinari per manifestarsi.

---

## La soluzione: GetOrLoad con singleflight

`GetOrLoad` risolve il problema con **singleflight**: per ogni chiave, una sola goroutine esegue il loader; le altre aspettano il risultato senza toccare il database.

```go
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUser(1) // chiamato una sola volta, anche con 1000 goroutine concorrenti
}, xcache.WithTTL(10*time.Minute))
```

```
chiave scaduta
  goroutine 1 → GetOrLoad → miss → esegue loader → query DB → Set
  goroutine 2 → GetOrLoad → miss → aspetta goroutine 1 ↗
  goroutine 3 → GetOrLoad → miss → aspetta goroutine 1 ↗
  ...
  goroutine 1 termina → tutte ricevono lo stesso risultato
```

Il `loader` riceve lo stesso `ctx` passato a `GetOrLoad`: partecipa alle cancellazioni e deadline del chiamante. Se il contesto viene cancellato durante il caricamento, tutte le goroutine in attesa ricevono l'errore di cancellazione.

---

## Quando usarlo

`GetOrLoad` è la scelta predefinita per pattern di lettura con fallback: è semanticamente equivalente a Get + Set ma più sicuro e più semplice. Il pattern manuale ha senso solo quando il caricamento deve avvenire in un punto separato dalla lettura (es. pipeline di pre-riscaldamento della cache).

!!! note
    Il deduplicatore di `GetOrLoad` usa la chiave già prefissata come token interno. Chiamate concorrenti su `"1"` con prefisso `"users:"` si deduplicano correttamente su `"users:1"`.

---

*[TTL]: Time To Live
