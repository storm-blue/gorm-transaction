package transaction

import (
	"context"
	"errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"testing"
	"time"
)

type User struct {
	ID         uint
	Username   string `gorm:"size:64"`
	CreateTime time.Time
}

func (User) TableName() string {
	return "user"
}

var (
	user1 = &User{Username: "test_user_1", CreateTime: time.Now()}
	user2 = &User{Username: "test_user_2", CreateTime: time.Now()}
	user3 = &User{Username: "test_user_3", CreateTime: time.Now()}
	user4 = &User{Username: "test_user_4", CreateTime: time.Now()}
)

var (
	dsn       = os.Getenv("DSN")
	db, _     = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	tm        = NewTransactionManager(db)
	mockErr   = errors.New("mock error")
	mockPanic = func() { panic("mock panic") }
	_recover  = func() { recover() }
)

func init() {
	_ = db.AutoMigrate(User{})
}

func clearData() {
	db.Delete(User{}, "1=1")
}

func AssertExist(user *User, t *testing.T) {
	var count int64 = 0
	db.Model(&User{}).Where("username = ?", user.Username).Count(&count)
	if count == 0 {
		t.Errorf("user %v should exist", user.Username)
	}
}

func AssertNotExist(user *User, t *testing.T) {
	var count int64 = 0
	db.Model(&User{}).Where("username = ?", user.Username).Count(&count)
	if count > 0 {
		t.Errorf("user %v should not exist", user.Username)
	}
}

func AssertErrorsIsEqual(err1, err2 error, t *testing.T) {
	if err1.Error() != err2.Error() {
		t.Errorf("errors: [%v], [%v] should equal", err1, err2)
	}
}

func TransactionTest(name string, t *testing.T, initFn func(), cleanFn func(), testFn func(), checkFn func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		initFn()
		defer cleanFn()
		defer func() {
			if r := recover(); r != nil {
				t.Logf("test: [%v] catch panic: %v", name, r)
			}
		}()

		testFn()
		checkFn(t)
	})
}

func DefaultTransactionTest(name string, t *testing.T, testFn func(), checkFn func(t *testing.T)) {
	TransactionTest(name, t, func() { clearData() }, func() { clearData() }, testFn, checkFn)
}

func TestTransactionManager_Transaction_PropagationRequired(t *testing.T) {

	DefaultTransactionTest("test-all-commit",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}
				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
		},
	)

	DefaultTransactionTest("test-one-error-all-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return mockErr
				}, PropagationRequired); err != nil {
					return err
				}
				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-one-error-all-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					mockPanic()
					return nil
				}, PropagationRequired); err != nil {
					return err
				}
				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-one-error-all-rollback-2",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)

					if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user4)
						return mockErr
					}, PropagationRequired); err != nil {
						return err
					}

					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
			AssertNotExist(user4, t)
		},
	)

	DefaultTransactionTest("test-one-error-all-rollback-2-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)

					if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user4)
						mockPanic()
						return nil
					}, PropagationRequired); err != nil {
						return err
					}

					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
			AssertNotExist(user4, t)
		},
	)

	DefaultTransactionTest("test-one-error-not-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				//err are ignored, no rollback
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return mockErr
				}, PropagationRequired)

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
		},
	)

	DefaultTransactionTest("test-one-error-not-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				defer _recover()
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				//err are ignored, no rollback
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					mockPanic()
					return nil
				}, PropagationRequired)

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
		},
	)

	DefaultTransactionTest("test-one-error-not-rollback-2",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)

					//err are ignored, no rollback
					_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user4)
						return mockErr
					}, PropagationRequired)

					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
			AssertExist(user4, t)
		},
	)

	DefaultTransactionTest("test-one-error-not-rollback-2-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					defer _recover()
					tx.Create(user3)

					//err are ignored, no rollback
					_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user4)
						mockPanic()
						return nil
					}, PropagationRequired)

					return nil
				}, PropagationRequired); err != nil {
					return err
				}

				return nil
			}, PropagationRequired)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
			AssertExist(user4, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationSupports(t *testing.T) {

	DefaultTransactionTest("test-supports-propagation-in-transaction",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationSupports)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-in-transaction-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationSupports)

				return mockErr
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-in-transaction-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationSupports)

				mockPanic()
				return nil
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-not-in-transaction-1",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationSupports)

				return mockErr
			}, PropagationNever)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-not-in-transaction-1-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationSupports)

				mockPanic()
				return nil
			}, PropagationNever)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-not-in-transaction-2",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)
				return mockErr
			}, PropagationSupports)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-supports-propagation-not-in-transaction-2-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)
				mockPanic()
				return nil
			}, PropagationSupports)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationMandatory(t *testing.T) {

	DefaultTransactionTest("test-mandatory-propagation-in-transaction",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationMandatory)
				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-mandatory-propagation-in-transaction-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationMandatory)
				return mockErr
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
		},
	)

	DefaultTransactionTest("test-mandatory-propagation-in-transaction-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationMandatory)
				mockPanic()
				return nil
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
		},
	)

	var err error
	DefaultTransactionTest("test-mandatory-propagation-not-in-transaction",
		t,
		func() {
			ctx := context.Background()
			err = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)
				return nil
			}, PropagationMandatory)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertErrorsIsEqual(err, ErrMandatoryPropWithoutTransaction, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationRequiresNew(t *testing.T) {

	DefaultTransactionTest("test-user1-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequiresNew)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return nil
				}, PropagationRequiresNew)

				return mockErr
			}, PropagationRequiresNew)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
		},
	)

	DefaultTransactionTest("test-user1-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequiresNew)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return nil
				}, PropagationRequiresNew)

				mockPanic()
				return nil
			}, PropagationRequiresNew)
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertExist(user2, t)
			AssertExist(user3, t)
		},
	)

	DefaultTransactionTest("test-user3-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequiresNew)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					return mockErr
				}, PropagationRequiresNew)
				return nil
			}, PropagationRequiresNew)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-user3-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				defer _recover()
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationRequiresNew)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user3)
					mockPanic()
					return nil
				}, PropagationRequiresNew)
				return nil
			}, PropagationRequiresNew)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertNotExist(user3, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationNotSupported(t *testing.T) {

	DefaultTransactionTest("test-not-supported-propagation-in-transaction",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user1)
					return nil
				}, PropagationNotSupported)
				return mockErr
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)

	DefaultTransactionTest("test-not-supported-propagation-not-in-transaction",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)
				return nil
			}, PropagationNotSupported)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationNested(t *testing.T) {

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return mockErr
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertNotExist(user2, t)
		},
	)

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				defer _recover()
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					mockPanic()
					return nil
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertNotExist(user2, t)
		},
	)

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback-2",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)

					if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user3)
						return mockErr
					}); err != nil {
						return err
					}

					return nil
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback-2-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				defer _recover()
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)

					if err := tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user3)
						mockPanic()
						return nil
					}); err != nil {
						return err
					}

					return nil
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertNotExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback-3",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)

					_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user3)
						return mockErr
					}, PropagationNested)

					return nil
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-outside-commit-nested-inside-rollback-3-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					defer _recover()
					tx.Create(user2)

					_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
						tx.Create(user3)
						mockPanic()
						return nil
					}, PropagationNested)

					return nil
				}, PropagationNested)

				return nil
			})
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertExist(user2, t)
			AssertNotExist(user3, t)
		},
	)

	DefaultTransactionTest("test-outside-rollback-cause-inside-rollback",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationNested)

				return mockErr
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
		},
	)

	DefaultTransactionTest("test-outside-rollback-cause-inside-rollback-panic",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationNested)

				mockPanic()
				return nil
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
		},
	)
}

func TestTransactionManager_Transaction_PropagationNever(t *testing.T) {

	var err error

	DefaultTransactionTest("test-never-propagation-in-no-transaction",
		t,
		func() {
			ctx := context.Background()
			err = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)
				return mockErr
			}, PropagationNever)
		},
		func(t *testing.T) {
			AssertExist(user1, t)
			AssertErrorsIsEqual(err, mockErr, t)
		},
	)

	DefaultTransactionTest("test-never-propagation-in-transaction",
		t,
		func() {
			ctx := context.Background()
			_ = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
				tx.Create(user1)

				if err = tm.Transaction(ctx, func(ctx context.Context, tx *gorm.DB) error {
					tx.Create(user2)
					return nil
				}, PropagationNever); err != nil {
					return err
				}

				return mockErr
			})
		},
		func(t *testing.T) {
			AssertNotExist(user1, t)
			AssertNotExist(user2, t)
			AssertErrorsIsEqual(err, ErrNeverPropInTransaction, t)
		},
	)
}
