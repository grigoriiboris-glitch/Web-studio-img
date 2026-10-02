package variants

import (
  "context"
  "crypto/sha256"
  "database/sql"
  "encoding/json"
  "errors"
  "fmt"

  "github.com/google/uuid"
)

var ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")

func mutationRequestHash(operation string, payload any) string {
  raw, _ := json.Marshal(struct {
    Operation string
    Payload   any
  }{operation, payload})
  sum := sha256.Sum256(raw)
  return fmt.Sprintf("%x", sum[:])
}

func claimVariantMutation(ctx context.Context, tx *sql.Tx, userID, projectID uuid.UUID, operation, key, requestHash string) (bool, []byte, error) {
  var id uuid.UUID
  err := tx.QueryRowContext(ctx, "INSERT INTO variant_mutation_idempotency(user_id,project_id,operation,idempotency_key,request_hash) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,idempotency_key) DO NOTHING RETURNING id", userID, projectID, operation, key, requestHash).Scan(&id)
  if err == nil {
    return false, nil, nil
  }
  if !errors.Is(err, sql.ErrNoRows) {
    return false, nil, err
  }

  var storedHash string
  var response []byte
  if err := tx.QueryRowContext(ctx, "SELECT request_hash,response FROM variant_mutation_idempotency WHERE user_id=$1 AND idempotency_key=$2", userID, key).Scan(&storedHash, &response); err != nil {
    return false, nil, err
  }
  if storedHash != requestHash {
    return false, nil, ErrIdempotencyConflict
  }
  if response == nil {
    return false, nil, errors.New("idempotency record has no completed response")
  }
  return true, response, nil
}

func storeVariantMutationResponse(ctx context.Context, tx *sql.Tx, userID uuid.UUID, operation, key string, entityID uuid.UUID, response []byte) error {
  _, err := tx.ExecContext(ctx, "UPDATE variant_mutation_idempotency SET entity_id=$1,response=$2::jsonb WHERE user_id=$3 AND operation=$4 AND idempotency_key=$5", entityID, response, userID, operation, key)
  return err
}
