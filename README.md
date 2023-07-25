# Xormigrate
xormigrate是在gormigrate的基础上实现的，适用于xorm，提供了便利的api实现数据库迁移
```
go get github.com/lsy88/xormigrate
```
### 使用

```
// 在这里添加你的迁移函数
var mi = []*Migration{
	{
		Version: "202307241038", //默认时间戳，也可以为 202307211350_tableName
		Migrate: func(tx *xorm.Engine) error {
			type Person struct {
				Address string
			}
			e := tx.Sync2(new(Person))
			return e
			
		},
		Rollback: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE person DROP COLUMN address")
			return err
		},
	},
}
func main() {
	Engine, err := xorm.NewEngine("mysql", "dns")
	if err != nil {
		fmt.Println(err)
	}
	//Engine.ShowSQL(true)
	// 建议全量迁移和增量迁移分别使用
	// 全量
	initmigrator := New(Engine, DefaultOptions, []*Migration{})
	// 增量
	migrator := New(Engine, DefaultOptions, mi)
	initmigrator.InitSchema(func(engine *xorm.Engine) error {
		return engine.Sync2(new(Pet), new(Person))
	})
	initmigrator.Migrate()
	migrator.Migrate()
	// 回滚至指定版本
	migrator.RollbackTo("202307241042_person")
	// 回滚至上一版本
	migrator.RollbackLast()
}
```