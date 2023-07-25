package migrate

import (
	"errors"
	"fmt"
	"reflect"
	"time"
	
	"github.com/go-xorm/xorm"
)

const (
	// 保留Version 只有在初始化时使用
	initSchemaMigrationVersion = "SCHEMA_INIT"
)

type MigrateFunc func(engine *xorm.Engine) error

type RollbackFunc func(engine *xorm.Engine) error

type InitSchemaFunc func(engine *xorm.Engine) error

// Options define options for all migrations.
type Options struct {
	// TableName 默认migrations
	TableName string
	// IDColumnName 默认id
	VersionColumnName string
	// IDColumnSize
	VersionColumnSize int64
	// UseTransaction
	//UseTransaction bool
	// 如果数据库中有未知的迁移id, ValidateUnknownMigrations将导致迁移失败
	ValidateUnknownMigrations bool
	// 启用硬删除, 默认软删除
	HardDelete bool
}

// Migration 数据库迁移操作
type Migration struct {
	// Usually a timestamp like "201601021504".
	// 也可以 "201601021504_tableName" 追踪单表
	Version string
	// Migrate 迁移函数
	Migrate MigrateFunc
	// Rollback 回滚函数 可为nil
	Rollback RollbackFunc
	// Description 对此次迁移进行描述
	Description string
}

// XorMigrate 进行迁移
type XorMigrate struct {
	db         *xorm.Engine
	tx         *xorm.Session
	options    *Options
	migrations []*Migration
	initSchema InitSchemaFunc
}

// ReservedIDError 错误使用保留version作为某次迁移version
type ReservedIDError struct {
	Version string
}

func (e *ReservedIDError) Error() string {
	return fmt.Sprintf(`xormigrate: Reserved migration Version: %s"`, e.Version)
}

// DuplicatedIDError 存在重复ID
type DuplicatedIDError struct {
	Version string
}

func (e *DuplicatedIDError) Error() string {
	return fmt.Sprintf(`xormigrate: Duplicated migration Version: "%s"`, e.Version)
}

var (
	// DefaultOptions 默认
	DefaultOptions = &Options{
		TableName:         "migrations",
		VersionColumnName: "version",
		VersionColumnSize: 255,
		//UseTransaction:            false,
		ValidateUnknownMigrations: false,
		HardDelete:                false,
	}
	
	// ErrRollbackImpossible 回滚没有回滚功能的迁移时
	ErrRollbackImpossible = errors.New("xormigrate: It's impossible to rollback this migration")
	
	// ErrNoMigrationDefined 未定义迁移
	ErrNoMigrationDefined = errors.New("xormigrate: No migration defined")
	
	// ErrMissingID 当迁移ID等于""时
	ErrMissingVersion = errors.New("xormigrate: Missing Version in migration")
	
	// ErrNoRunMigration 在运行RollbackLast时发现正在运行迁移时返回
	ErrNoRunMigration = errors.New("xormigrate: Could not find last run migration")
	
	// ErrMigrationIDDoesNotExist 迁移或回滚到迁移列表中不存在的迁移ID时返回
	ErrMigrationVersionDoesNotExist = errors.New("xormigrate: Tried to migrate to an Version that doesn't exist")
	
	// ErrUnknownPastMigration 迁移存在于数据库中但是不存在于代码中
	ErrUnknownPastMigration = errors.New("xormigrate: Found migration in DB that does not exist in code")
)

// New Xormigrate.
func New(engine *xorm.Engine, options *Options, migrations []*Migration) *XorMigrate {
	if options.TableName == "" {
		options.TableName = DefaultOptions.TableName
	}
	if options.VersionColumnName == "" {
		options.VersionColumnName = DefaultOptions.VersionColumnName
	}
	if options.VersionColumnSize == 0 {
		options.VersionColumnSize = DefaultOptions.VersionColumnSize
	}
	return &XorMigrate{
		db:         engine,
		options:    options,
		migrations: migrations,
	}
}

// InitSchema 如果没有发现迁移,则运行该函数
// 进行初始化迁移, 在这个函数中,您应该创建应用程序所需的所有表
func (x *XorMigrate) InitSchema(initSchema InitSchemaFunc) {
	x.initSchema = initSchema
}

// Migrate 执行所有尚未运行的迁移
func (x *XorMigrate) Migrate() error {
	if !x.hasMigrations() {
		return ErrNoMigrationDefined
	}
	var targetMigrationID string
	if len(x.migrations) > 0 {
		targetMigrationID = x.migrations[len(x.migrations)-1].Version
	}
	return x.migrate(targetMigrationID)
}

// MigrateTo 根据migrationID进行迁移
// MigrateTo 执行所有尚未运行的迁移,直到匹配' migrationID '的迁移
func (x *XorMigrate) MigrateTo(migrationID string) error {
	if err := x.checkIDExist(migrationID); err != nil {
		return err
	}
	return x.migrate(migrationID)
}

func (x *XorMigrate) migrate(migrationVersion string) error {
	if !x.hasMigrations() {
		return ErrNoMigrationDefined
	}
	
	if err := x.checkReservedID(); err != nil {
		return err
	}
	
	if err := x.checkDuplicatedID(); err != nil {
		return err
	}
	
	x.begin()
	defer x.rollback()
	
	if err := x.createMigrationTableIfNotExists(); err != nil {
		return err
	}
	
	if x.options.ValidateUnknownMigrations {
		unknownMigrations, err := x.unknownMigrationsHaveHappened()
		if err != nil {
			return err
		}
		if unknownMigrations {
			return ErrUnknownPastMigration
		}
	}
	
	if x.initSchema != nil {
		canInitializeSchema, err := x.canInitializeSchema()
		if err != nil {
			return err
		}
		if canInitializeSchema {
			if err := x.runInitSchema(); err != nil {
				return err
			}
			return x.commit()
		}
	}
	
	for _, migration := range x.migrations {
		if err := x.runMigration(migration); err != nil {
			return err
		}
		if migrationVersion != "" && migration.Version == migrationVersion {
			break
		}
	}
	return x.commit()
}

// 如果有一个已定义的initSchema函数,或者如果迁移列表不为空,则会进行迁移
func (x *XorMigrate) hasMigrations() bool {
	return x.initSchema != nil || len(x.migrations) > 0
}

// 检查是否有迁移使用保留ID,目前只有一个"SCHEMA_INIT"
func (x *XorMigrate) checkReservedID() error {
	for _, m := range x.migrations {
		if m.Version == initSchemaMigrationVersion {
			return &ReservedIDError{Version: m.Version}
		}
	}
	return nil
}

// 检查重复ID
func (x *XorMigrate) checkDuplicatedID() error {
	lookup := make(map[string]struct{}, len(x.migrations))
	for _, m := range x.migrations {
		if _, ok := lookup[m.Version]; ok {
			return &DuplicatedIDError{Version: m.Version}
		}
		lookup[m.Version] = struct{}{}
	}
	return nil
}

func (x *XorMigrate) checkIDExist(migrationID string) error {
	for _, migrate := range x.migrations {
		if migrate.Version == migrationID {
			return nil
		}
	}
	return ErrMigrationVersionDoesNotExist
}

// RollbackLast 回滚至上一次迁移
func (x *XorMigrate) RollbackLast() error {
	if len(x.migrations) == 0 {
		return ErrNoMigrationDefined
	}
	
	x.begin()
	defer x.rollback()
	
	lastRunMigration, err := x.getLastRunMigration()
	if err != nil {
		return err
	}
	
	if err := x.rollbackMigration(lastRunMigration); err != nil {
		return err
	}
	return x.commit()
}

// RollbackTo 回滚至指定ID
func (x *XorMigrate) RollbackTo(migrationVersion string) error {
	if len(x.migrations) == 0 {
		return ErrNoMigrationDefined
	}
	
	if err := x.checkIDExist(migrationVersion); err != nil {
		return err
	}
	
	x.begin()
	defer x.rollback()
	
	for i := len(x.migrations) - 1; i >= 0; i-- {
		migration := x.migrations[i]
		if migration.Version == migrationVersion {
			break
		}
		migrationRan, err := x.migrationRan(migration)
		if err != nil {
			return err
		}
		if migrationRan {
			if err := x.rollbackMigration(migration); err != nil {
				return err
			}
		}
	}
	return x.commit()
}

func (x *XorMigrate) getLastRunMigration() (*Migration, error) {
	for i := len(x.migrations) - 1; i >= 0; i-- {
		migration := x.migrations[i]
		
		migrationRan, err := x.migrationRan(migration)
		if err != nil {
			return nil, err
		}
		
		if migrationRan {
			return migration, nil
		}
	}
	return nil, ErrNoRunMigration
}

// RollbackMigration 自定义回滚.
func (x *XorMigrate) RollbackMigration(m *Migration) error {
	x.begin()
	defer x.rollback()
	
	if err := x.rollbackMigration(m); err != nil {
		return err
	}
	return x.commit()
}

func (x *XorMigrate) rollbackMigration(m *Migration) error {
	if m.Rollback == nil {
		return ErrRollbackImpossible
	}
	
	if err := m.Rollback(x.db); err != nil {
		return err
	}
	
	cond := fmt.Sprintf("%s = ?", x.options.VersionColumnName)
	var err error
	// 进行硬删除
	if x.options.HardDelete {
		_, err = x.tx.Table(x.options.TableName).Where(cond, m.Version).Delete(x.model())
		return err
	}
	_, err = x.tx.Table(x.options.TableName).Where(cond, m.Version).Update(map[string]interface{}{"is_rollback": 1})
	return err
}

func (x *XorMigrate) runInitSchema() error {
	if err := x.initSchema(x.db); err != nil {
		return err
	}
	if err := x.insertMigration(initSchemaMigrationVersion); err != nil {
		return err
	}
	
	for _, migration := range x.migrations {
		if err := x.insertMigration(migration.Version); err != nil {
			return err
		}
	}
	
	return nil
}

func (x *XorMigrate) runMigration(migration *Migration) error {
	if len(migration.Version) == 0 {
		return ErrMissingVersion
	}
	
	migrationRan, err := x.migrationRan(migration)
	if err != nil {
		return err
	}
	if !migrationRan {
		if err := migration.Migrate(x.db); err != nil {
			return err
		}
		
		if err := x.insertMigration(migration.Version); err != nil {
			return err
		}
	}
	return nil
}

// model 返回指向动态创建的xorm迁移模型结构体值的指针
//
//	struct defined as {
//	  ID string `xorm:"pk Options.IDColumnName size(Options.IDColumnSize)"`
//	}
func (x *XorMigrate) model() interface{} {
	g := reflect.StructField{
		Name: reflect.ValueOf("ID").Interface().(string),
		Type: reflect.TypeOf(""),
		Tag:  reflect.StructTag(`xorm:"pk autoincr 'id' int"`),
	}
	w := reflect.StructField{
		Name: reflect.ValueOf("Version").Interface().(string),
		Type: reflect.TypeOf(""),
		Tag: reflect.StructTag(fmt.Sprintf(
			`xorm:"notnull unique '%s' varchar(%d)"`,
			x.options.VersionColumnName,
			x.options.VersionColumnSize,
		)),
	}
	c := reflect.StructField{
		Name: reflect.ValueOf("IsRollback").Interface().(string),
		Type: reflect.TypeOf(""),
		Tag:  reflect.StructTag(`xorm:"default(0) int 'is_rollback'"`),
	}
	
	structType := reflect.StructOf([]reflect.StructField{g, w, c})
	structValue := reflect.New(structType).Elem()
	//fmt.Printf("value: %+v\n", structValue.Addr().Interface())
	return structValue.Addr().Interface()
}

func (x *XorMigrate) createMigrationTableIfNotExists() error {
	exist, err := x.tx.IsTableExist(x.options.TableName)
	if exist || err != nil {
		return err
	}
	return x.tx.Table(x.options.TableName).Sync2(x.model())
}

func (x *XorMigrate) migrationRan(m *Migration) (bool, error) {
	count, err := x.db.
		Table(x.options.TableName).
		Where(fmt.Sprintf("%s = ? AND is_rollback = 0", x.options.VersionColumnName), m.Version).Count()
	return count > 0, err
}

// 只有在尚未初始化且没有其他迁移应用的情况下才可以初始化
func (x *XorMigrate) canInitializeSchema() (bool, error) {
	migrationRan, err := x.migrationRan(&Migration{Version: initSchemaMigrationVersion})
	if err != nil {
		return false, err
	}
	if migrationRan {
		return false, nil
	}
	
	// If the ID doesn't exist, we also want the list of migrations to be empty
	var count int64
	count, err = x.tx.
		Table(x.options.TableName).
		Count()
	return count == 0, err
}

// 检测是否有未知的迁移发生,数据库中存在但是migrations中不存在
func (x *XorMigrate) unknownMigrationsHaveHappened() (bool, error) {
	rows, err := x.db.Table(x.options.TableName).Select(x.options.VersionColumnName).Rows(x.model())
	if err != nil {
		return false, err
	}
	defer rows.Close()
	
	validIDSet := make(map[string]struct{}, len(x.migrations)+1)
	validIDSet[initSchemaMigrationVersion] = struct{}{}
	for _, migration := range x.migrations {
		validIDSet[migration.Version] = struct{}{}
	}
	
	for rows.Next() {
		var pastMigration = x.model()
		if err = rows.Scan(pastMigration); err != nil {
			return false, err
		}
		pm := reflect.Indirect(reflect.ValueOf(pastMigration))
		if _, ok := validIDSet[pm.Field(0).String()]; !ok {
			return true, nil
		}
	}
	
	return false, nil
}

func (x *XorMigrate) insertMigration(id string) error {
	var err error
	record := map[string]interface{}{x.options.VersionColumnName: id}
	_, err = x.tx.Table(x.options.TableName).Insert(record)
	return err
}

func (x *XorMigrate) begin() {
	x.tx = x.db.NewSession()
}

func (x *XorMigrate) commit() error {
	return x.tx.Commit()
}

func (x *XorMigrate) rollback() {
	x.tx.Rollback()
}

// TimeStampToID 根据时间戳 生成ID
func (x *XorMigrate) GenVersion() string {
	um := time.Now().UnixMicro()
	t := time.UnixMicro(um)
	// 格式化日期字符串
	dateStr := t.Format("200601021504")
	return dateStr
}
