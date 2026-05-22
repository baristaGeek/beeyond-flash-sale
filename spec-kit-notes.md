Documented assumptions
1. We don’t have to deploy this. We just need to deliver the repo 
2. To better document the chat history with Claude Code we’re going to do everything in a single run, therefore this includes running CLI commands and doing git operations. 
3. Dado que el ejercicio es mas céntrico en el backend, le vamos a meter mas tiempo a esa parte de ejercicio
4. A reservation is terminal, it only ends via manual release or 60s TTL expiry. 
5. Anonymous users, identified by an X-Session-Id header (no real auth).
6.  One inventory pool per sale — no variants (size/color).
7. A session can hold multiple concurrent reservations.
8. Consistency over availability — DB down or lock timeout returns a typed error; we never serve stale/guessed inventory
9. Pessimistic locking only: every reserve transaction does SELECT ... FOR NO KEY UPDATE on the parent sales row as the serialization anchor. No optimistic CAS, no SERIALIZABLE isolation
10. Idempotency-Key is mandatory on POST /reservations (not optional)
11. Same key + same payload ⇒ same outcome cached; same key + different payload ⇒ typed mismatch error (distinct from insufficient-stock).
12. We don’t need an ORM
