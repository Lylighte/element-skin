package permission

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	core "element-skin/backend/internal/permission"
)

type PermissionCache interface {
	GetEffective(ctx context.Context, subjectID string) (core.BitSet, bool, error)
	SetEffective(ctx context.Context, subjectID string, bits core.BitSet, ttl time.Duration) error
	DeleteEffective(ctx context.Context, subjectID string) error
}

var permissionCachePrefix = catalogCachePrefix()

func encodeBitSet(bits core.BitSet) string {
	if len(bits) == 0 {
		return permissionCachePrefix + ":"
	}
	buf := make([]byte, len(bits)*8)
	for i, w := range bits {
		binary.BigEndian.PutUint64(buf[i*8:], w)
	}
	return permissionCachePrefix + ":" + base64.RawStdEncoding.EncodeToString(buf)
}

func decodeBitSet(s string) (core.BitSet, error) {
	if s == "" {
		return nil, nil
	}
	prefix, encoded, ok := strings.Cut(s, ":")
	if !ok || prefix != permissionCachePrefix {
		return nil, nil
	}
	if encoded == "" {
		return nil, nil
	}
	buf, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	bits := make(core.BitSet, len(buf)/8)
	for i := range bits {
		bits[i] = binary.BigEndian.Uint64(buf[i*8:])
	}
	return bits, nil
}

func catalogCachePrefix() string {
	hash := sha256.New()
	for _, def := range core.Definitions {
		fmt.Fprintf(hash, "%d:%s:%d\n", def.ID, def.Code, def.BitIndex)
	}
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))[:16]
}

type RedisPermCache struct {
	Store PermCacheStore
}

type PermCacheStore interface {
	GetPermissionCache(ctx context.Context, subjectID string) (string, bool, error)
	SetPermissionCache(ctx context.Context, subjectID string, encoded string, ttl time.Duration) error
	DeletePermissionCache(ctx context.Context, subjectID string) error
}

func (c *RedisPermCache) GetEffective(ctx context.Context, subjectID string) (core.BitSet, bool, error) {
	raw, ok, err := c.Store.GetPermissionCache(ctx, subjectID)
	if err != nil || !ok {
		return nil, ok, err
	}
	bits, err := decodeBitSet(raw)
	if err != nil {
		return nil, false, err
	}
	if bits == nil {
		return nil, false, nil
	}
	return bits, true, nil
}

func (c *RedisPermCache) SetEffective(ctx context.Context, subjectID string, bits core.BitSet, ttl time.Duration) error {
	return c.Store.SetPermissionCache(ctx, subjectID, encodeBitSet(bits), ttl)
}

func (c *RedisPermCache) DeleteEffective(ctx context.Context, subjectID string) error {
	return c.Store.DeletePermissionCache(ctx, subjectID)
}
