package tenant

import "gorm.io/gorm"

// Scope 给 GORM 查询附加租户过滤条件。
// tenantID <= 0 表示调用方未声明租户（系统内部任务或历史调用），此时不加条件，
// 由调用方自行保证不会跨租户泄露数据。
func Scope(db *gorm.DB, tenantID int64) *gorm.DB {
	if db == nil || tenantID <= 0 {
		return db
	}
	return db.Where("tenant_id = ?", tenantID)
}

// ColumnScope 与 Scope 相同，但允许指定列名，供带表别名或 JOIN 的查询使用。
func ColumnScope(db *gorm.DB, column string, tenantID int64) *gorm.DB {
	if db == nil || tenantID <= 0 || column == "" {
		return db
	}
	return db.Where(column+" = ?", tenantID)
}
