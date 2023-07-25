package migrate

import (
	"fmt"
	"testing"
	
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

var mi = []*Migration{
	{
		Version: "202307241038_person", //默认时间戳，也可以为 202307211350_tableName
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
	{
		Version: "202307241039_pet", //默认时间戳，也可以为 202307211350_tableName
		Migrate: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE pet DROP COLUMN p_name")
			return err
		},
		Rollback: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE pet ADD COLUMN (p_name varchar(255))")
			return err
		},
	},
	{
		Version: "202307241042_person", //默认时间戳，也可以为 202307211350_tableName
		Migrate: func(tx *xorm.Engine) error {
			type Person struct {
				A string
			}
			e := tx.Sync2(new(Person))
			return e
			
		},
		Rollback: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE person DROP COLUMN a")
			return err
		},
	},
	{
		Version: "202307241043_person", //默认时间戳，也可以为 202307211350_tableName
		Migrate: func(tx *xorm.Engine) error {
			type Person struct {
				B string
			}
			e := tx.Sync2(new(Person))
			return e
			
		},
		Rollback: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE person DROP COLUMN b")
			return err
		},
	},
	{
		Version: "202307241044_person", //默认时间戳，也可以为 202307211350_tableName
		Migrate: func(tx *xorm.Engine) error {
			type Person struct {
				C string
			}
			e := tx.Sync2(new(Person))
			return e
			
		},
		Rollback: func(tx *xorm.Engine) error {
			_, err := tx.Exec("ALTER TABLE person DROP COLUMN c")
			return err
		},
	},
}

type Person struct {
	Name string
	Age  int
}

type Pet struct {
	Name  string
	PName string
}

func TestMigrate(t *testing.T) {
	Engine, err := xorm.NewEngine("mysql", "dns")
	if err != nil {
		fmt.Println(err)
	}
	Engine.ShowSQL(true)
	initmigrator := New(Engine, DefaultOptions, []*Migration{})
	//initmigrator.model()
	migrator := New(Engine, DefaultOptions, mi)
	initmigrator.InitSchema(func(engine *xorm.Engine) error {
		return engine.Sync2(new(Pet), new(Person))
	})
	fmt.Println(initmigrator.Migrate())
	fmt.Println(migrator.Migrate())
	
	//Engine.Table(&Person{}).Insert(map[string]interface{}{
	//	"name": "lisy",
	//	"age":  20,
	//	"a":    "aaa",
	//	"b":    "bbb",
	//})
	//migrator.RollbackTo("202307241042_person")
	migrator.RollbackLast()
}
