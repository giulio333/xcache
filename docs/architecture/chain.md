# Chain cache

Una singola cache in memoria è velocissima ma non condivisa tra istanze. Una cache distribuita (Redis, Memcached) è condivisa ma introduce latenza di rete su ogni operazione. La chain cache combina i due livelli: la cache in memoria assorbe il traffico ripetuto, quella distribuita garantisce la coerenza tra istanze.

---

## Il meccanismo di backfill

`ChainStore` scorre i tier da sinistra a destra. Non appena un tier risponde con successo, `ChainStore` ripopola automaticamente tutti i tier precedenti e ritorna il valore.

```
prima richiesta per "user:1":
  L1.Get → miss
  L2.Get → hit → backfill L1 con TTL residuo e tag originali → ritorna valore

richieste successive per "user:1":
  L1.Get → hit → ritorna valore   (L2 non viene toccato)
```

Il punto critico è il **TTL residuo**: se la chiave in L2 scade tra 3 minuti, L1 la memorizza con 3 minuti, non con il TTL originale di 10. Questo garantisce che L1 e L2 scadano in modo coerente — la chiave non sopravvive in L1 oltre la sua scadenza in L2.

I tag vengono propagati allo stesso modo: `DeleteByTag` continua a funzionare su tutti i tier, anche su quelli ripopolati tramite backfill.

---

## Scritture e cancellazioni

`Set`, `Delete`, `DeleteMany`, `DeleteByTag` e `Clear` propagano a tutti i tier in ordine, dal primo all'ultimo. La prima operazione che ritorna un errore ferma la propagazione.

!!! warning "Non atomica tra tier"
    Se L1 viene aggiornato e L2 fallisce, i due tier restano in stato inconsistente fino alla prossima riconciliazione. Per invalidazioni critiche, considerare retry lato applicativo.

---

## Coerenza del TTL nel backfill

Vale la pena soffermarsi su questo punto perché è controintuitivo.

Considera una chiave scritta con TTL di 10 minuti. Dopo 7 minuti, L1 viene riavviato e perde la chiave. Al `Get` successivo:

- senza propagazione corretta del TTL: L1 memorizza la chiave con 10 minuti → la chiave sopravvive in L1 per 10 minuti dopo il restart, mentre in L2 scade dopo 3 minuti → inconsistenza.
- con propagazione del TTL residuo: L1 memorizza la chiave con 3 minuti residui → entrambi i tier scadono nello stesso momento.

`ChainStore` usa sempre il TTL residuo dell'`Entry` restituita da L2, non il TTL originale con cui la chiave era stata scritta.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
