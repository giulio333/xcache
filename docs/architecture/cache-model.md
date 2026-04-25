# Il modello Cache[T] e Store

`Store` non sa nulla dei tipi concreti. Riceve e restituisce `any`, gestisce TTL, tag e scadenze. Una singola istanza di `MemoryStore` può contenere indistintamente `User`, `Product`, `Session` — dal suo punto di vista sono tutti valori opachi con una chiave e una scadenza.

`Cache[T]` aggiunge esclusivamente la firma generica sopra. Quando chiami `Set(ctx, "u1", user)`, converte `user` in `any` e lo passa allo store. Quando chiami `Get(ctx, "u1")`, recupera l'`any` dallo store e lo converte in `T`. Il codice applicativo non vede mai né `any` né type assertion — quella conversione avviene in un unico punto interno alla libreria.

```go title="main.go"
store := memory.NewStore()

userCache    := xcache.New[User](store)
productCache := xcache.New[Product](store)

// Lo stesso store fisico, due cache con tipi diversi
userCache.Set(ctx, "u:1", user)
productCache.Set(ctx, "p:1", product)
```

La conseguenza pratica è che lo store è infrastruttura condivisa. Puoi costruire quante `Cache[T]` vuoi sullo stesso backend — in memoria, Redis, o una chain — senza duplicare connessioni o goroutine.

Quando più `Cache[T]` condividono lo stesso store, le chiavi devono essere distinte per evitare collisioni. `WithPrefix` si occupa di questo in modo trasparente: antepone una stringa fissa a ogni chiave prima che raggiunga lo store, e la rimuove dalle chiavi restituite in output. Il chiamante lavora sempre con chiavi corte e leggibili.

```go title="main.go"
utenti   := xcache.New[User](store,    xcache.WithPrefix("u:"))
prodotti := xcache.New[Product](store, xcache.WithPrefix("p:"))

utenti.Set(ctx, "1", user)   // salvato come "u:1"
utenti.Get(ctx, "1")         // legge "u:1", ritorna User
```

!!! note
    Il prefisso viene usato anche come parte del token di deduplicazione interno a `GetOrLoad`: chiamate concorrenti su `"1"` con prefisso `"u:"` si deduplicano correttamente su `"u:1"`.

---

*[TTL]: Time To Live
