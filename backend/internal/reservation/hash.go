package reservation

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// CanonicalRequestHash returns SHA-256(sale_id_bytes || canonical_json(body)).
// Canonical JSON: object keys sorted lexicographically, no extra whitespace,
// numbers in their JSON-native form. The hash scope deliberately includes the
// sale id (taken from the URL path) so two requests targeting different sales
// can never collide on the same idempotency key.
func CanonicalRequestHash(saleID uuid.UUID, body []byte) ([]byte, error) {
	canon, err := canonicalize(body)
	if err != nil {
		return nil, fmt.Errorf("hash: canonicalize body: %w", err)
	}
	h := sha256.New()
	saleBytes, err := saleID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("hash: marshal sale id: %w", err)
	}
	h.Write(saleBytes)
	h.Write(canon)
	return h.Sum(nil), nil
}

// canonicalize parses arbitrary JSON and re-emits it with keys sorted.
// It handles objects, arrays, and primitives.
func canonicalize(body []byte) ([]byte, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return []byte("{}"), nil
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return marshalCanonical(v)
}

func marshalCanonical(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			vb, err := marshalCanonical(t[k])
			if err != nil {
				return nil, err
			}
			buf.Write(vb)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, el := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			eb, err := marshalCanonical(el)
			if err != nil {
				return nil, err
			}
			buf.Write(eb)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		return json.Marshal(t)
	}
}
