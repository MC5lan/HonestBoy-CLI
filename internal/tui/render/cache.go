package render

// Cache 是块级 LRU 渲染缓存。T708 实现 200 块上限。
type Cache struct{ max int }

func NewCache(max int) *Cache { return &Cache{max: max} }

func (c *Cache) Get(key string) (string, bool) { return "", false }
func (c *Cache) Put(key, value string)         {}
