package errors_test

import (
	"testing"

	"github.com/yourname/go-gear/errors"
)

func TestWithStack(t *testing.T) {
	base := errors.Errorf("database connection refused")

	// 中间层不想加描述，只记录堆栈
	err := errors.WithStack(base)
	t.Log("=== WithStack（不加描述，只记堆栈）===")
	t.Logf("%%v: %v", err)
	t.Log(errors.Stack(err))

	// 上层再 Wrap
	err2 := errors.Wrap(err, "init app failed")
	t.Log("=== 继续 Wrap ===")
	t.Log(errors.Stack(err2))
}

func TestFrom(t *testing.T) {
	// 模拟外部 error（如 sql.ErrNoRows、os.ErrNotExist）
	externalErr := errors.New("sql: no rows in result set")

	// 转换为项目 error
	err := errors.From(externalErr)
	t.Log("=== From（无描述）===")
	t.Log(errors.Stack(err))

	// 转换并附加描述
	err2 := errors.FromMsg(externalErr, "query user by id")
	t.Log("=== FromMsg（有描述）===")
	t.Log(errors.Stack(err2))

	// 再 Wrap 一层
	err3 := errors.Wrap(err2, "get user failed")
	t.Log("=== 继续 Wrap ===")
	t.Log(errors.Stack(err3))

	// 已是项目 error，From 不重复包裹
	err4 := errors.From(err)
	if err4 != err {
		t.Error("From should not re-wrap project error")
	}
}

func TestWrapChain(t *testing.T) {
	// 模拟三层调用
	err := repository()
	err = service(err)
	err = handler(err)

	t.Log("=== 简短 ===")
	t.Log(err.Error())

	t.Log("=== 完整堆栈 ===")
	t.Log(errors.Stack(err))

	// errors.Is 仍然正常工作
	base := errors.New("duplicate key")
	wrapped := errors.Wrap(errors.Wrap(base, "insert failed"), "create user failed")
	if !errors.Is(wrapped, base) {
		t.Error("errors.Is should work through wrap chain")
	}
}

func repository() error {
	return errors.Errorf("insert user: duplicate key \"email\"")
}

func service(err error) error {
	return errors.Wrap(err, "create user failed")
}

func handler(err error) error {
	return errors.Wrap(err, "user register failed")
}
