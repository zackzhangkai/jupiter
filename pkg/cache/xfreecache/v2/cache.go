package xfreecache

import (
	"fmt"
	"github.com/douyu/jupiter/pkg/xlog"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"reflect"
)

type storage interface {
	// SetCacheData 设置缓存数据 key：缓存key data：缓存数据
	SetCacheData(key string, data []byte) (err error)
	// GetCacheData 存储缓存数据 key：缓存key data：缓存数据
	GetCacheData(key string) (data []byte, err error)
}

type cache[K comparable, V any] struct {
	storage
}

// GetAndSetCacheData 获取缓存后数据
func (c *cache[K, V]) GetAndSetCacheData(key string, id K, fn func() (V, error)) (value V, err error) {
	resMap, err := c.GetAndSetCacheMap(key, []K{id}, func([]K) (map[K]V, error) {
		innerVal, innerErr := fn()
		return map[K]V{id: innerVal}, innerErr
	})
	value = resMap[id]
	return
}

// GetCacheValue 获取缓存数据
func (c *cache[K, V]) GetCacheValue(key string, id K) (value V) {
	resMap, _ := c.getCacheMap(key, []K{id})
	value = resMap[id]
	return
}

// SetCacheValue 设置缓存数据
func (c *cache[K, V]) SetCacheValue(key string, id K, fn func() (V, error)) (err error) {
	err = c.setCacheMap(key, []K{id}, func([]K) (map[K]V, error) {
		innerVal, innerErr := fn()
		return map[K]V{id: innerVal}, innerErr
	}, nil)
	return
}

// GetAndSetCacheMap 获取缓存后数据 map形式
func (c *cache[K, V]) GetAndSetCacheMap(key string, ids []K, fn func([]K) (map[K]V, error)) (v map[K]V, err error) {
	// 获取缓存数据
	v, idsNone := c.getCacheMap(key, ids)

	// 设置缓存数据
	err = c.setCacheMap(key, idsNone, fn, v)
	return
}

// GetCacheMap 获取缓存数据 map形式
func (c *cache[K, V]) GetCacheMap(key string, ids []K) (v map[K]V) {
	v, _ = c.getCacheMap(key, ids)
	return
}

// SetCacheMap 设置缓存数据 map形式
func (c *cache[K, V]) SetCacheMap(key string, ids []K, fn func([]K) (map[K]V, error)) (err error) {
	err = c.setCacheMap(key, ids, fn, nil)
	return
}

func (c *cache[K, V]) getCacheMap(key string, ids []K) (v map[K]V, idsNone []K) {
	var zero V
	v = make(map[K]V)
	idsNone = make([]K, 0, len(ids))

	// id去重
	ids = lo.Uniq(ids)
	for _, id := range ids {
		cacheKey := c.getKey(key, id)
		resT, innerErr := c.GetCacheData(cacheKey)
		if innerErr == nil && resT != nil {
			var value V
			// 反序列化
			value, innerErr = unmarshal[V](resT)
			if innerErr != nil {
				xlog.Jupiter().Error("cache unmarshalWithPool", zap.String("key", key), zap.Error(innerErr))
			} else {
				if !reflect.DeepEqual(value, zero) {
					v[id] = value
				}
			}
		}
		if innerErr != nil {
			idsNone = append(idsNone, id)
		}
	}
	return
}

func (c *cache[K, V]) setCacheMap(key string, idsNone []K, fn func([]K) (map[K]V, error), v map[K]V) (err error) {
	args := []zap.Field{zap.Any("key", key), zap.Any("ids", idsNone)}

	if len(idsNone) == 0 {
		return
	}

	// 执行函数
	resMap, err := fn(idsNone)
	if err != nil {
		xlog.Jupiter().Error("GetAndSetCacheMap doMap", append(args, zap.Error(err))...)
		return
	}

	// 填入返回中
	if v != nil {
		for k, value := range resMap {
			v[k] = value
		}
	}

	// 写入缓存
	for _, id := range idsNone {
		var (
			cacheData V
			data      []byte
		)

		if val, ok := resMap[id]; ok {
			cacheData = val
		}
		// 序列化
		data, err = marshal(cacheData)

		if err != nil {
			xlog.Jupiter().Error("GetAndSetCacheMap Marshal", append(args, zap.Error(err))...)
			return
		}

		cacheKey := c.getKey(key, id)
		err = c.SetCacheData(cacheKey, data)
		if err != nil {
			xlog.Jupiter().Error("GetAndSetCacheMap setCacheData", append(args, zap.Error(err))...)
			return
		}
	}
	return
}

func (c *cache[K, V]) getKey(key string, id K) string {
	return fmt.Sprintf("%s:%v", key, id)
}
